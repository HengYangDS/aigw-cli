package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
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

func TestClaudeDesktopInspectionKeepsIntactRetainedVersionedReader(t *testing.T) {
	retained, current := versionedReaders(t)
	fixture := newClaudeDesktopFixture(t)
	fixture.deps.AIGWExecutable = current
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); status.Ready || !strings.Contains(status.Issue, "projection differs") {
		t.Fatalf("unsynchronized Claude Desktop projection = %#v", status)
	}
	fixture.deps.AIGWExecutable = retained
	fixture.apply(t)
	before, err := os.ReadFile(fixture.paths.Profile)
	if err != nil {
		t.Fatal(err)
	}
	fixture.deps.AIGWExecutable = current
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); !status.Ready {
		t.Fatalf("intact retained Claude Desktop reader was rejected: %s", status.Issue)
	}
	after, err := os.ReadFile(fixture.paths.Profile)
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("inspection changed Claude Desktop profile: %v", err)
	}
	if err := os.Remove(fixture.paths.Profile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.paths.Profile, 0o700); err != nil {
		t.Fatal(err)
	}
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); status.Ready || strings.Contains(status.Issue, fixture.paths.Profile) {
		t.Fatalf("Claude Desktop inspection exposed an unreadable profile: %#v", status)
	}
	if err := os.Remove(fixture.paths.Profile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.paths.Profile, before, 0o600); err != nil {
		t.Fatal(err)
	}
	changed := fixture.cfg.Clone()
	account := changed.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://changed.test"
	changed.Accounts["gateway"] = account
	changedRuntime, err := changed.ResolveRuntime(configuration.ClientClaudeDesktop, "")
	if err != nil {
		t.Fatal(err)
	}
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, changed, changedRuntime); status.Ready {
		t.Fatal("an endpoint change was mistaken for a retained-reader transition")
	}
	if err := os.RemoveAll(filepath.Dir(retained)); err != nil {
		t.Fatal(err)
	}
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, fixture.cfg, fixture.runtime); status.Ready {
		t.Fatal("missing retained Claude Desktop reader was accepted")
	}
}

type routeVerificationRunner struct {
	plans     []process.Plan
	config    []byte
	challenge string
}

func (runner *routeVerificationRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	return output, nil, err
}

func (runner *routeVerificationRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return []byte("codex-cli 9.9.9\n"), nil
	}
	outputPath := argumentValue(plan.Args, "--output-last-message")
	if outputPath == "" {
		if argumentValue(plan.Args, "--output-format") == "stream-json" {
			return []byte(`{"type":"result","is_error":false,"result":"AIGW_OK"}` + "\n"), nil
		}
		return []byte("AIGW_OK\n"), nil
	}
	home := environmentValue(plan.Env, "CODEX_HOME")
	var err error
	runner.config, err = os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		return nil, err
	}
	tool := ""
	if !slices.Contains(plan.Args, "resume") {
		challenge, err := os.ReadFile(filepath.Join(filepath.Dir(outputPath), "challenge.txt"))
		if err != nil {
			return nil, err
		}
		runner.challenge = strings.TrimSpace(string(challenge))
		tool = fmt.Sprintf(`{"type":"item.completed","item":{"type":"command_execution","status":"completed","command":"cat challenge.txt","exit_code":0,"aggregated_output":%q}}`+"\n", runner.challenge)
	}
	if err := os.WriteFile(outputPath, []byte(runner.challenge+"\n"), 0o600); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"type":"thread.started","thread_id":"00000000-0000-4000-8000-000000000001"}`+"\n"+`%s{"type":"item.completed","item":{"type":"agent_message","text":%q}}`+"\n"+`{"type":"turn.completed"}`+"\n", tool, runner.challenge)), nil
}

func argumentValue(arguments []string, name string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}

func TestFailedClaudeVerificationProjectionRemovesItsWorkspace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	_, workspace, err := isolateClaudeProjection(
		configuration.NewConfig(), configuration.Runtime{}, Dependencies{ClaudeSettingsPath: root},
	)
	if err == nil {
		t.Fatal("directory source unexpectedly copied as Claude settings")
	}
	if workspace != "" {
		t.Fatalf("failed projection retained workspace %q", workspace)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed projection left temporary resources: %v, %v", entries, err)
	}
}

func TestClaudeVerificationUsesAnIsolatedProjectionForAnUnselectedRoute(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["selected"] = qualifiedRoute("", "gateway", "claude-selected", configuration.ProtocolAnthropic)
	cfg.Routes["alternate"] = qualifiedRoute("", "gateway", "claude-alternate", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientClaude, "selected", "")
	cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
	selected, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "settings.json")
	aigwExecutable := filepath.Join(root, "aigw")
	if _, err := claude.ReconcileSettings(settings, false, selected, aigwExecutable, selected.Model); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "alternate")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &routeVerificationRunner{}
	if _, err := (claudeAdapter{}).Verify(context.Background(), Dependencies{
		Runner: runner, Secrets: store, ClaudeSettingsPath: settings, AIGWExecutable: aigwExecutable,
	}, cfg, runtime, "alternate"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("explicit Route verification changed the selected Claude projection")
	}
	request := runner.plans[len(runner.plans)-1]
	settingsIndex := slices.Index(request.Args, "--settings")
	if settingsIndex < 0 || settingsIndex+1 >= len(request.Args) {
		t.Fatalf("verification plan has no settings path: %#v", request)
	}
	verificationSettings := request.Args[settingsIndex+1]
	if verificationSettings == settings {
		t.Fatalf("verification settings = %q, want an isolated projection", verificationSettings)
	}
	if _, err := os.Stat(filepath.Dir(verificationSettings)); !os.IsNotExist(err) {
		t.Fatalf("isolated Claude projection survived verification: %s: %v", verificationSettings, err)
	}
}

func TestClaudeVerificationRefusesUnresolvedSelectionBeforeInvocation(t *testing.T) {
	for _, failure := range []string{"selected route", "retained reader"} {
		t.Run(failure, func(t *testing.T) {
			settings := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(settings, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(t.TempDir(), "claude")
			if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
			cfg.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
			binding := configuration.ClientBinding{Route: "claude", Enabled: true, Executable: executable}
			cfg.Clients[configuration.ClientClaude] = binding
			selected, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
			if err != nil {
				t.Fatal(err)
			}
			store := secrets.NewMemoryStore()
			if err := store.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			runner := &routeVerificationRunner{}
			deps := Dependencies{Runner: runner, Secrets: store, ClaudeSettingsPath: settings}
			want := "unknown route"
			if failure == "selected route" {
				binding.Route = "missing"
				cfg.Clients[configuration.ClientClaude] = binding
			} else {
				_, deps.AIGWExecutable = versionedReaders(t)
				if err := os.Remove(settings); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(settings, 0o700); err != nil {
					t.Fatal(err)
				}
				want = "retained"
			}
			if _, err := (claudeAdapter{}).Verify(t.Context(), deps, cfg, selected, ""); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("verification refusal = %v, want %q", err, want)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("verification invoked a client before resolving its selection: %#v", runner.plans)
			}
		})
	}
}
