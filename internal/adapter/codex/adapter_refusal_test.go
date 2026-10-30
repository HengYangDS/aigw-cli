package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/secret"
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
		reader, err := secret.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := secret.EnsureEntrypoint(source, reader); err != nil {
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
			binding := cfg.Clients[ID]
			selected, err := cfg.ResolveRuntime(ID, "")
			if err != nil {
				t.Fatal(err)
			}
			store := secret.NewMemoryStore()
			if err := store.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			runner := &routeVerificationRunner{}
			deps := adapter.Dependencies{Runner: runner, Secrets: store}
			want := "unknown route"
			if failure == "selected route" {
				binding.Route = "missing"
				cfg.Clients[ID] = binding
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
