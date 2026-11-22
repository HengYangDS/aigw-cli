package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// projectForInspection writes cfg's Codex files as a sync does, reading the
// client's table through executable, and returns the runtime that an
// inspection of them takes.
func projectForInspection(t *testing.T, cfg domain.Config, executable string) domain.Runtime {
	t.Helper()
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	refs := make([]TargetRef, 0, len(cfg.Clients[ID].Targets))
	for _, target := range cfg.Clients[ID].Targets {
		ref := HomeTarget(target)
		ref.Executable = executable
		refs = append(refs, ref)
	}
	if _, err := ReconcileConfigs(refs, refs, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	return runtime
}

// inspectedChecks inspects cfg's Codex files and returns each check as
// "ID ready target".
func inspectedChecks(cfg domain.Config, runtime domain.Runtime) ([]string, adapter.Status) {
	status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{}, cfg, runtime)
	checks := make([]string, 0, len(status.Checks))
	for _, check := range status.Checks {
		checks = append(checks, fmt.Sprintf("%s %t %s", check.ID, check.Ready, check.Target))
	}
	return checks, status
}

// TestCodexInspectionReportsTheWindowThatItsTableGives reads the passing
// check of a target whose table describes the selected model, by an alias and
// by the model's own entry: it names the window that the entry carries, the
// default and then a lower declared one, and the target as the target's own
// check does, and no path.
func TestCodexInspectionReportsTheWindowThatItsTableGives(t *testing.T) {
	for name, model := range windowModels {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg := windowConfiguration(model, "gpt-6.2-sol", path)
			for _, declared := range []int{0, 400_000} {
				want := domain.DefaultContextWindow
				if declared != 0 {
					cfg, want = declareWindow(cfg, declared), declared
				}
				checks, status := inspectedChecks(cfg, projectForInspection(t, cfg, "codex"))
				if !status.Ready || strings.Join(checks, "; ") != "codex:target-1 true "+path+"; codex:window-1 true "+path {
					t.Fatalf("checks = %q, status = %+v", checks, status)
				}
				window := status.Checks[1]
				if detail := fmt.Sprintf("Codex's model table gives gateway/%s a context window of %d tokens", model, want); window.Detail != detail || window.RepairAction != "" {
					t.Fatalf("the window check = %+v, want the detail %q", window, detail)
				}
			}
		})
	}
}

// TestCodexInspectionSaysWhenCodexsFallbackWindowApplies reads the passing
// check of a model that the table has no entry for: its family is not in the
// table, or its alias had no name left. The text names no figure for the
// fallback, which is the client's.
func TestCodexInspectionSaysWhenCodexsFallbackWindowApplies(t *testing.T) {
	const fallback = ", so Codex uses its own fallback window whatever the model declares"
	for name, test := range map[string]struct{ table, model, family, want string }{
		"the family is not in the table": {
			table: windowedTable, model: "muse-spark-1.3", family: "muse-spark-1.3",
			want: "Codex's model table has no entry for the family muse-spark-1.3 of gateway/muse-spark-1.3" + fallback,
		},
		"the alias has no name left": {
			table: `{"models":[{"slug":"gpt-6.2-sol","display_name":"Sol by the gateway"},{"slug":"other","display_name":"Sol by the gateway (gpt-6.2-sol-cdx)"}]}`,
			model: "gpt-6.2-sol-cdx", family: "gpt-6.2-sol",
			want: "Codex's model table has no entry for gateway/gpt-6.2-sol-cdx" + fallback,
		},
	} {
		t.Run(name, func(t *testing.T) {
			original := codexBundledCatalog
			t.Cleanup(func() { codexBundledCatalog = original })
			codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
				return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, []byte(test.table), nil
			}
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg := windowConfiguration(test.model, test.family, path)
			checks, status := inspectedChecks(cfg, projectForInspection(t, cfg, "codex"))
			if !status.Ready || len(checks) != 2 || checks[1] != "codex:window-1 true "+path || status.Checks[1].Detail != test.want {
				t.Fatalf("checks = %q, status = %+v; want the detail %q", checks, status, test.want)
			}
		})
	}
}

// TestCodexInspectionAddsNoWindowCheckWithoutATableOfAIGWs leaves the
// target's own check alone where AIGW wrote no table, where the file is no
// longer the one that it recorded, and where it cannot be parsed.
func TestCodexInspectionAddsNoWindowCheckWithoutATableOfAIGWs(t *testing.T) {
	t.Run("no table was written", func(t *testing.T) {
		path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
		cfg := windowConfiguration("gpt-6.2-sol-cdx", "gpt-6.2-sol", path)
		checks, status := inspectedChecks(cfg, projectForInspection(t, cfg, ""))
		if !status.Ready || strings.Join(checks, "; ") != "codex:target-1 true "+path {
			t.Fatalf("checks = %q, status = %+v", checks, status)
		}
	})
	// A table that another writer left is not read, even one that describes
	// the model; a table that AIGW recorded gives no check where it cannot be
	// parsed or its entry for the model carries no window. Each case names
	// the file's content, none for a removed file, whether AIGW's record holds
	// that content, and whether the target's own check passes.
	for name, test := range map[string]struct {
		table           string
		recorded, ready bool
	}{
		"the table changed outside AIGW":        {table: `{"models":[{"slug":"gpt-6.2-sol-cdx","context_window":1,"max_context_window":1}]}`},
		"the recorded table is no longer there": {},
		"the recorded table cannot be parsed":   {table: "not a table", recorded: true, ready: true},
		"the recorded entry carries no window":  {table: `{"models":[{"slug":"gpt-6.2-sol-cdx"}]}`, recorded: true, ready: true},
	} {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg := windowConfiguration("gpt-6.2-sol-cdx", "gpt-6.2-sol", path)
			runtime := projectForInspection(t, cfg, "codex")
			if err := os.Remove(codexCatalogPath(path)); err != nil {
				t.Fatal(err)
			}
			if test.table != "" {
				if err := os.WriteFile(codexCatalogPath(path), []byte(test.table), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.recorded {
				state := readCodexSidecar(t, path)
				state.CatalogHash = platform.SHA256([]byte(test.table))
				data, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(codexStatePath(path), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			checks, status := inspectedChecks(cfg, runtime)
			if status.Ready != test.ready || len(checks) != 1 || !strings.HasPrefix(checks[0], "codex:target-1 ") {
				t.Fatalf("checks = %q, status = %+v", checks, status)
			}
		})
	}
}

// TestCodexInspectionReportsEachTargetsWindow numbers a window check as its
// target's own check and gives it that target, after every target's check.
func TestCodexInspectionReportsEachTargetsWindow(t *testing.T) {
	stubWindowedTable(t)
	first := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	second := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	cfg := windowConfiguration("gpt-6.2-sol-cdx", "gpt-6.2-sol", first, second)
	checks, status := inspectedChecks(cfg, projectForInspection(t, cfg, "codex"))
	want := []string{"codex:target-1 true " + first, "codex:target-2 true " + second, "codex:window-1 true " + first, "codex:window-2 true " + second}
	if !status.Ready || strings.Join(checks, "; ") != strings.Join(want, "; ") {
		t.Fatalf("checks = %q, want %q; status = %+v", checks, want, status)
	}
}
