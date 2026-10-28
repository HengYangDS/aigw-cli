package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/secrets"
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

func TestCodexVerificationRefusesUnresolvedSelectionBeforeInvocation(t *testing.T) {
	for _, failure := range []string{"selected route", "retained reader"} {
		t.Run(failure, func(t *testing.T) {
			cfg, _ := codexVerificationFixture(t)
			binding := cfg.Clients[configuration.ClientCodex]
			selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
			if err != nil {
				t.Fatal(err)
			}
			store := secrets.NewMemoryStore()
			if err := store.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			runner := &routeVerificationRunner{}
			deps := client.Dependencies{Runner: runner, Secrets: store}
			want := "unknown route"
			if failure == "selected route" {
				binding.Route = "missing"
				cfg.Clients[configuration.ClientCodex] = binding
			} else {
				_, deps.AIGWExecutable = versionedReaders(t)
				if err := os.Remove(binding.Targets[0]); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(binding.Targets[0], 0o700); err != nil {
					t.Fatal(err)
				}
				want = "retained"
			}
			if _, err := (codexAdapter{}).Verify(t.Context(), deps, cfg, selected, ""); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("verification refusal = %v, want %q", err, want)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("verification invoked a client before resolving its selection: %#v", runner.plans)
			}
		})
	}
}
