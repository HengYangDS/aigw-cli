package codex

import (
	"aigw-cli/internal/domain"
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
		reader, err := versionedReaderPath(filepath.Join(root, "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ensureReader(source, reader); err != nil {
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
			selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
			if err != nil {
				t.Fatal(err)
			}
			store := secret.NewMemoryStore()
			if err := store.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			runner := &modelVerificationRunner{}
			deps := adapter.Dependencies{Runner: runner, Secrets: store}
			want := "unknown model"
			if failure == "selected route" {
				binding.Model = domain.NewModelRef("gateway", "missing")
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

// ensureReader installs the program at source as the credential reader at
// target, or verifies the one there, as a synchronization does, and returns
// the compensation that removes only what it created.
func ensureReader(source, target string) (func() error, error) {
	program, err := secret.ReadProgram(nil, source)
	if err != nil {
		return nil, err
	}
	_, undo, err := program.EnsureEntrypoint(target)
	return undo, err
}

// versionedReaderPath is the versioned credential reader below dataDir for
// the program at source, as the program names its own.
func versionedReaderPath(dataDir, source, installName string) (string, error) {
	program, err := secret.ReadProgram(nil, source)
	if err != nil {
		return "", err
	}
	return program.EntrypointPath(dataDir, installName)
}
