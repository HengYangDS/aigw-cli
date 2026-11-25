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

// TestCodexInspectionSaysWhenCodexChoosesTheWindowItself reads the passing
// check of a model that the table has no entry for: its family is not in the
// table, it is its own family, or its alias had no name left. The text names
// no figure and no fallback: Codex matches a model ID to the entry whose slug
// is its longest prefix before it falls back, so the window is Codex's choice.
func TestCodexInspectionSaysWhenCodexChoosesTheWindowItself(t *testing.T) {
	const chooses = ", so Codex chooses the window itself, whatever the model declares"
	for name, test := range map[string]struct{ table, model, family, want string }{
		"the family is not in the table": {
			table: windowedTable, model: "muse-spark-1.3", family: "muse-spark-1.3",
			want: "Codex's model table has no entry for the family muse-spark-1.3 of gateway/muse-spark-1.3" + chooses,
		},
		"the model is its own family, and an entry's slug begins its ID": {
			table: windowedTable, model: "gpt-6.2-sol-cdx",
			want: "Codex's model table has no entry for the family gpt-6.2-sol-cdx of gateway/gpt-6.2-sol-cdx" + chooses,
		},
		"the alias has no name left": {
			table: `{"models":[{"slug":"gpt-6.2-sol","display_name":"Sol by the gateway"},{"slug":"other","display_name":"Sol by the gateway (gpt-6.2-sol-cdx)"}]}`,
			model: "gpt-6.2-sol-cdx", family: "gpt-6.2-sol",
			want: "Codex's model table has no entry for gateway/gpt-6.2-sol-cdx" + chooses,
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

// earlierTable stands for the table that 0.9.0 wrote for either model of
// windowModels: the client's own, in which a model's own entry keeps the
// client's two window fields, with an alias that carries its family's.
var earlierTable = strings.TrimSuffix(windowedTable, "]}") +
	`,{"slug":"gpt-6.2-sol-cdx","display_name":"Sol by the gateway","context_window":500000,"max_context_window":872000,"reasoning_effort":"high"}]}`

// recordTable writes table as the model table beside the Codex file at path,
// and into AIGW's record of that file as the table that AIGW wrote.
func recordTable(t *testing.T, path, table string) {
	t.Helper()
	if err := os.WriteFile(codexCatalogPath(path), []byte(table), 0o600); err != nil {
		t.Fatal(err)
	}
	state := readCodexSidecar(t, path)
	state.CatalogHash = platform.SHA256([]byte(table))
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(path), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCodexInspectionFailsATableThatDoesNotGiveTheDeclaredWindow reads the
// check of a target whose table holds an entry for the selected model that
// does not carry the declared window in both fields: the table that 0.9.0
// wrote, for an alias and for an entry; a declared window that no sync wrote;
// a ceiling that alone differs; and an entry without one field or both. The
// check and the adapter's status are not ready, name the model, what the table
// gives and what the model declares, and are repaired by a sync, while the
// target's own check passes.
func TestCodexInspectionFailsATableThatDoesNotGiveTheDeclaredWindow(t *testing.T) {
	const earlier = "a context window of 500000 tokens and a ceiling of 872000 tokens"
	for name, test := range map[string]struct {
		model, table, gives string
		declared            int
	}{
		"an alias as 0.9.0 wrote it":             {model: "gpt-6.2-sol-cdx", table: earlierTable, gives: earlier},
		"an entry as 0.9.0 wrote it":             {model: "gpt-6.2-sol", table: windowedTable, gives: earlier},
		"a declared window that was not written": {model: "gpt-6.2-sol-cdx", declared: 400_000, gives: "a context window of 1000000 tokens and a ceiling of 1000000 tokens"},
		"a ceiling that alone differs":           {model: "gpt-6.2-sol-cdx", table: earlierTable, declared: 500_000, gives: earlier},
		"an entry without a ceiling":             {model: "gpt-6.2-sol-cdx", table: `{"models":[{"slug":"gpt-6.2-sol-cdx","context_window":1000000}]}`, gives: "a context window of 1000000 tokens and no ceiling"},
		"an entry without the window fields":     {model: "gpt-6.2-sol-cdx", table: `{"models":[{"slug":"gpt-6.2-sol-cdx"}]}`, gives: "no context window and no ceiling"},
	} {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg := windowConfiguration(test.model, "gpt-6.2-sol", path)
			runtime := projectForInspection(t, cfg, "codex")
			if test.table != "" {
				recordTable(t, path, test.table)
			}
			want := domain.DefaultContextWindow
			if test.declared != 0 {
				// The declaration reaches the inspection and no file.
				cfg, want = declareWindow(cfg, test.declared), test.declared
				declaring, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
				if err != nil {
					t.Fatal(err)
				}
				declaring.CredentialCommand = runtime.CredentialCommand
				runtime = declaring
			}
			checks, status := inspectedChecks(cfg, runtime)
			detail := fmt.Sprintf("Codex's model table gives gateway/%s %s, but the model declares %d tokens", test.model, test.gives, want)
			if status.Ready || status.Issue != "Codex configuration drift: "+detail || status.RepairAction != "aigw sync" ||
				strings.Join(checks, "; ") != "codex:target-1 true "+path+"; codex:window-1 false "+path {
				t.Fatalf("checks = %q, status = %+v; want the issue %q", checks, status, detail)
			}
			if window := status.Checks[1]; window.Detail != detail || window.RepairAction != "aigw sync" {
				t.Fatalf("the window check = %+v, want the detail %q and a sync as its repair", window, detail)
			}
		})
	}
}

// TestACodexTargetsOwnFailureKeepsTheAdaptersStatus leaves the adapter's issue
// and repair to a target's own check that failed, here over an outside edit
// that a sync refuses: the table that does not give the declared window is
// reported by its own check, with a sync as that check's repair only.
func TestACodexTargetsOwnFailureKeepsTheAdaptersStatus(t *testing.T) {
	stubWindowedTable(t)
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	cfg := windowConfiguration("gpt-6.2-sol-cdx", "gpt-6.2-sol", path)
	runtime := projectForInspection(t, cfg, "codex")
	recordTable(t, path, earlierTable)
	written := string(mustReadFile(t, path))
	edited := strings.Replace(written, "https://gateway.test/v1", "https://elsewhere.test/v1", 1)
	if edited == written {
		t.Fatalf("no endpoint to edit in:\n%s", written)
	}
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	checks, status := inspectedChecks(cfg, runtime)
	if status.Ready || strings.Join(checks, "; ") != "codex:target-1 false "+path+"; codex:window-1 false "+path {
		t.Fatalf("checks = %q, status = %+v", checks, status)
	}
	own, window := status.Checks[0], status.Checks[1]
	if status.Issue != own.Detail || status.RepairAction != own.RepairAction || own.RepairAction == "aigw sync" || window.RepairAction != "aigw sync" {
		t.Fatalf("status = %+v, want the issue and the repair of the target's own check %+v, and a sync only for %+v", status, own, window)
	}
}

// TestCodexInspectionSaysWhereAIGWWritesNoTable holds the scenario "AIGW
// writes no table for a Codex target": the configuration names a model table
// of the user's own, or the model runs under a model provider of Codex's own,
// so Codex is not given the declared window. The passing check names the
// window, the model and the reason, beside the target's own check.
func TestCodexInspectionSaysWhereAIGWWritesNoTable(t *testing.T) {
	const model = "gpt-6.2-sol-cdx"
	for name, test := range map[string]struct {
		config func(t *testing.T) string
		edit   func(cfg domain.Config) domain.Config
		reason string
	}{
		"a model table of the user's own": {
			config: func(t *testing.T) string {
				own := writeCodexTestConfig(t, `{"models":[{"slug":"mine"}]}`)
				// A Windows path holds backslashes, which a TOML string must spell escaped; %q does.
				return fmt.Sprintf("model_catalog_json = %q\nmodel_provider = \"native\"\n", own)
			},
			edit:   func(cfg domain.Config) domain.Config { return declareWindow(cfg, 400_000) },
			reason: "this configuration names its own model table in model_catalog_json, which AIGW leaves alone",
		},
		"a model provider of Codex's own": {
			config: func(*testing.T) string { return "model_provider = \"native\"\n" },
			edit: func(cfg domain.Config) domain.Config {
				binding := cfg.Clients[ID]
				binding.ModelProvider, binding.Authentication = "amazon-bedrock", domain.AuthenticationClientNative
				cfg.Clients[ID] = binding
				return cfg
			},
			reason: "the model runs under Codex's model provider amazon-bedrock, for which AIGW writes no model table",
		},
	} {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, test.config(t))
			cfg := test.edit(windowConfiguration(model, "gpt-6.2-sol", path))
			checks, status := inspectedChecks(cfg, projectForInspection(t, cfg, "codex"))
			if _, err := os.Stat(codexCatalogPath(path)); !os.IsNotExist(err) {
				t.Fatalf("AIGW wrote a model table: %v", err)
			}
			declared := cfg.Models[cfg.Clients[ID].Model].Window()
			detail := fmt.Sprintf("Codex is not given the window of %d tokens that gateway/%s declares, because %s", declared, model, test.reason)
			if !status.Ready || strings.Join(checks, "; ") != "codex:target-1 true "+path+"; codex:window-1 true "+path || status.Checks[1].Detail != detail {
				t.Fatalf("checks = %q, status = %+v; want the detail %q", checks, status, detail)
			}
		})
	}
}

// TestCodexInspectionAddsNoWindowCheckWithoutATableOfAIGWs leaves the
// target's own check alone where AIGW wrote no table because the client
// printed none, where the file is no longer the one that it recorded, and
// where it cannot be parsed.
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
	// parsed. Each case names the file's content, none for a removed file,
	// whether AIGW's record holds that content, and whether the target's own
	// check passes.
	for name, test := range map[string]struct {
		table           string
		recorded, ready bool
	}{
		"the table changed outside AIGW":        {table: `{"models":[{"slug":"gpt-6.2-sol-cdx","context_window":1,"max_context_window":1}]}`},
		"the recorded table is no longer there": {},
		"the recorded table cannot be parsed":   {table: "not a table", recorded: true, ready: true},
	} {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg := windowConfiguration("gpt-6.2-sol-cdx", "gpt-6.2-sol", path)
			runtime := projectForInspection(t, cfg, "codex")
			if err := os.Remove(codexCatalogPath(path)); err != nil {
				t.Fatal(err)
			}
			switch {
			case test.recorded:
				recordTable(t, path, test.table)
			case test.table != "":
				if err := os.WriteFile(codexCatalogPath(path), []byte(test.table), 0o600); err != nil {
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
