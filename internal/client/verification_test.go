package client

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"

	"go.yaml.in/yaml/v3"
)

func versionedReaders(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, 2)
	for _, version := range []string{"predecessor", "successor"} {
		source := filepath.Join(root, version)
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := credential.EnsureEntrypoint(source, reader); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, reader)
	}
	return paths[0], paths[1]
}

func testHermesRetainedReader(t *testing.T, retained, current string) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "hermes")
	target := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("Hermes", "team", "model-test", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientHermes, "hermes", "")
	cfg.SetClientActivation(configuration.ClientHermes, true, executable, []string{target})
	store := secrets.NewMemoryStore()
	if err := store.Set("team", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	adapter := hermesAdapter{}
	deps := Dependencies{Secrets: store, AIGWExecutable: retained}
	selected, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	unprojected := deps
	unprojected.AIGWExecutable = current
	if status := adapter.Inspect(t.Context(), unprojected, cfg, selected); status.Ready || !strings.Contains(status.Issue, "projection differs") {
		t.Fatalf("unsynchronized Hermes projection = %#v", status)
	}
	if _, err := adapter.Apply(t.Context(), deps, configuration.NewConfig(), cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	deps.AIGWExecutable = current
	if status := adapter.Inspect(t.Context(), deps, cfg, selected); !status.Ready {
		t.Fatalf("intact retained Hermes reader was rejected: %s", status.Issue)
	}
	after, err := os.ReadFile(target)
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("inspection changed Hermes configuration: %v", err)
	}
	requireHermesVerificationReader(t, deps, cfg, selected, retained, before)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if status := adapter.Inspect(t.Context(), deps, cfg, selected); status.Ready || strings.Contains(status.Issue, target) {
		t.Fatalf("Hermes inspection exposed an unreadable target: %#v", status)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, before, 0o600); err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	account := changed.Accounts["team"]
	account.Endpoints.Anthropic = "https://changed.test"
	changed.Accounts["team"] = account
	changedRuntime, err := changed.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	if status := adapter.Inspect(t.Context(), deps, changed, changedRuntime); status.Ready {
		t.Fatal("an endpoint change was mistaken for a retained-reader transition")
	}
	if err := os.Remove(retained); err != nil {
		t.Fatal(err)
	}
	if status := adapter.Inspect(t.Context(), deps, cfg, selected); status.Ready {
		t.Fatal("missing retained Hermes reader was accepted")
	}
}

func requireHermesVerificationRefusal(t *testing.T, err error, calls int) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "cannot isolate") || calls != 0 {
		t.Fatalf("Windows verification did not refuse before launching: %v, calls=%d", err, calls)
	}
}

func requireHermesVerificationReader(t *testing.T, deps Dependencies, cfg configuration.Config, selected configuration.Runtime, retained string, before []byte) {
	t.Helper()
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes fixture"), []byte("AIGW_OK")}}
	runner.observe = func(plan process.Plan) {
		data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var observed struct {
			Providers map[string]struct {
				KeyCommand string `yaml:"key_cmd"`
			} `yaml:"providers"`
		}
		if err := yaml.Unmarshal(data, &observed); err != nil {
			t.Fatal(err)
		}
		if len(observed.Providers) != 1 {
			t.Fatal("verification omitted the managed Hermes provider")
		}
		for _, provider := range observed.Providers {
			if !strings.Contains(provider.KeyCommand, retained) || strings.Contains(provider.KeyCommand, deps.AIGWExecutable) {
				t.Fatal("verification replaced the retained credential reader before projection")
			}
		}
	}
	deps.Runner = runner
	_, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, "")
	if runtime.GOOS == "windows" {
		requireHermesVerificationRefusal(t, err, runner.calls)
	} else if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(cfg.Clients[configuration.ClientHermes].Targets[0])
	if err != nil || !slices.Equal(before, after) {
		t.Fatal("verification changed the retained Hermes configuration")
	}
}
