package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// windowedTable stands for Codex's own table: its entries carry both window
// fields, as the client's do, and one of them a field that AIGW does not model.
const windowedTable = `{"models":[` +
	`{"slug":"gpt-6.2-sol","display_name":"GPT-6.2 Sol","context_window":500000,"max_context_window":872000,"reasoning_effort":"high"},` +
	`{"slug":"gpt-6-luna","display_name":"GPT-6 Luna","context_window":272000,"max_context_window":872000}]}`

// windowModels are the two ways in which Codex's table describes a selected
// model: by an alias of its family's entry, and by the model's own entry.
var windowModels = map[string]string{"an alias": "gpt-6.2-sol-cdx", "an entry": "gpt-6.2-sol"}

func stubWindowedTable(t *testing.T) {
	t.Helper()
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, []byte(windowedTable), nil
	}
}

// stubUnprintableTable stands for the build that printed windowedTable and
// now fails to print its table, so that AIGW reuses the copy that it wrote.
func stubUnprintableTable(t *testing.T) {
	t.Helper()
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, nil, errors.New("codex debug models timed out")
	}
}

// windowConfiguration selects gateway/model, of family, for Codex at targets.
func windowConfiguration(model, family string, targets ...string) domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	described := qualifiedModel("Gateway · Sol by the gateway", "gateway", model, domain.ProtocolResponses)
	described.Family = family
	cfg.Models[described.Ref()] = described
	cfg.Clients[ID] = domain.ClientBinding{Model: described.Ref(), Enabled: true, Executable: "/opt/codex", Targets: targets}
	return cfg
}

// declareWindow returns cfg with the selected model declaring window.
func declareWindow(cfg domain.Config, window int) domain.Config {
	edited := cfg.Clone()
	model := edited.Models[edited.Clients[ID].Model]
	model.ContextWindow = window
	edited.Models[model.Ref()] = model
	return edited
}

// windowDependencies are what the adapter needs to plan and apply, with home
// as the configuration file of Codex's default home.
func windowDependencies(t *testing.T, home string) adapter.Dependencies {
	t.Helper()
	return adapter.Dependencies{
		Discovery: fixedDiscoverer{result: discovery.Result{
			Executables: map[string]string{ID: "/opt/codex"},
			Surfaces:    []discovery.Surface{{ID: string(HomeDefault), ConfigPath: home, AutoManaged: true}},
		}},
		AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
	}
}

// writtenTable reads the table that AIGW wrote beside the Codex file at path.
func writtenTable(t *testing.T, path string) modelcatalog.Document {
	t.Helper()
	data, err := os.ReadFile(codexCatalogPath(path))
	if err != nil {
		t.Fatalf("no model table beside %s: %v", path, err)
	}
	document, err := modelcatalog.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// requireTheClientsTableBut fails unless the table at path describes model
// with window in both window fields and holds every entry of windowedTable as
// the client has it, apart from those two fields of the model's own entry.
func requireTheClientsTableBut(t *testing.T, path, model string, window int) {
	t.Helper()
	written := writtenTable(t, path)
	if got, ceiling := written.ContextWindow(model); got != window || ceiling != window {
		t.Fatalf("the table gives %s the windows %d / %d, want %d in both fields", model, got, ceiling, window)
	}
	client, err := modelcatalog.Parse([]byte(windowedTable))
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"gpt-6.2-sol", "gpt-6-luna"} {
		got, want := written.Model(slug), client.Model(slug)
		if slug == model {
			for _, field := range []string{"context_window", "max_context_window"} {
				delete(got, field)
				delete(want, field)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("the table's entry %s = %s, want the client's %s", slug, got, want)
		}
	}
	described, family := written.Model(model), client.Model("gpt-6.2-sol")
	if len(described) != len(family) {
		t.Fatalf("the entry of %s = %s, want the fields of the client's %s", model, described, family)
	}
	for field, value := range family {
		if !slices.Contains([]string{"slug", "display_name", "context_window", "max_context_window"}, field) && string(described[field]) != string(value) {
			t.Fatalf("the entry of %s has %s = %s, want the client's %s", model, field, described[field], value)
		}
	}
}

// TestCodexTableGivesTheSelectedModelItsDeclaredWindow projects through the
// adapter: the entry that describes the selected model, its alias or its own,
// carries the model's window, the default for a model that declares none, and
// the rest of the table is the client's.
func TestCodexTableGivesTheSelectedModelItsDeclaredWindow(t *testing.T) {
	for name, model := range windowModels {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg, deps := windowConfiguration(model, "gpt-6.2-sol", path), windowDependencies(t, path)
			if _, err := (codexAdapter{}).Apply(context.Background(), deps, domain.NewConfig(testAdmission()), cfg); err != nil {
				t.Fatal(err)
			}
			requireTheClientsTableBut(t, path, model, domain.DefaultContextWindow)
			wantEntries := 2
			if model != "gpt-6.2-sol" {
				wantEntries = 3
			}
			var table struct {
				Models []json.RawMessage `json:"models"`
			}
			if err := json.Unmarshal(mustReadFile(t, codexCatalogPath(path)), &table); err != nil || len(table.Models) != wantEntries {
				t.Fatalf("the table holds %d entries, want %d: %v", len(table.Models), wantEntries, err)
			}
		})
	}
}

// TestAChangedDeclaredWindowRewritesCodexsTable is the adapter's part of a
// model edit: two configurations that differ only in the selected model's
// declared window count as a changed projection, plan a write of the table,
// write the new window into it, and leave a following plan with nothing to do.
// It holds while the client prints its table, and while the client cannot and
// AIGW reuses the copy that it wrote, whose entry for the model takes the
// window; an unchanged window gives that copy's bytes back, so the target
// stays converged.
func TestAChangedDeclaredWindowRewritesCodexsTable(t *testing.T) {
	for name, model := range windowModels {
		for client, reused := range map[string]bool{"the client prints its table": false, "the written copy is reused": true} {
			t.Run(name+", "+client, func(t *testing.T) {
				stubWindowedTable(t)
				path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
				before, deps := windowConfiguration(model, "gpt-6.2-sol", path), windowDependencies(t, path)
				client := codexAdapter{}
				if _, err := client.Apply(context.Background(), deps, domain.NewConfig(testAdmission()), before); err != nil {
					t.Fatal(err)
				}
				if reused {
					stubUnprintableTable(t)
				}
				requireNothingToDo := func(cfg domain.Config) {
					t.Helper()
					plans, err := client.Plan(deps, cfg, cfg)
					if err != nil || len(plans) != 1 || plans[0].ChangesState || plans[0].Action != string(ProjectionActionAlreadyConverged) {
						t.Fatalf("the plan of an unchanged window = %+v, %v; want nothing to do", plans, err)
					}
				}
				requireNothingToDo(before)
				// Below the default, and not a window of the client's own entries.
				const lowered = 400_000
				after := declareWindow(before, lowered)
				if !client.ProjectionChanged(before, after) || client.ProjectionChanged(after, after.Clone()) {
					t.Fatalf("a changed declared window must change the projection, and an equal one must not")
				}
				plans, err := client.Plan(deps, before, after)
				if err != nil || len(plans) != 1 || !plans[0].ChangesState || !slices.Contains(plans[0].Effects, codexCatalogPath(path)) {
					t.Fatalf("the plan of a changed window = %+v, %v; want a write of the table", plans, err)
				}
				if _, err := client.Apply(context.Background(), deps, before, after); err != nil {
					t.Fatal(err)
				}
				requireTheClientsTableBut(t, path, model, lowered)
				runtime, err := after.ResolveRuntime(ID, domain.ModelRef{})
				if err != nil {
					t.Fatal(err)
				}
				runtime.CredentialCommand = deps.AIGWExecutable
				if err := ValidateConfig(path, runtime); err != nil {
					t.Fatalf("the rewritten projection does not validate: %v", err)
				}
				requireNothingToDo(after)
			})
		}
	}
}

// TestAWindowThatCannotReachCodexIsNoChangedProjection holds an edit of the
// selected model's window to the clients whose files it can change. Codex
// counts as changed before AIGW wrote a table, and while the table that AIGW
// wrote for a target describes the model, by its ID or by its family, or
// cannot be read; it does not count while every target's table can be read
// and describes the model by neither. Another difference beside the window
// counts as before.
func TestAWindowThatCannotReachCodexIsNoChangedProjection(t *testing.T) {
	const model, family = "grok-4.7-fast", "grok-4.7"
	// Each case names what the second target's table becomes after the sync,
	// none for the client's table as written, which has no entry of either ID.
	for name, test := range map[string]struct {
		table            string
		removed, reaches bool
	}{
		"every table describes the model by neither": {},
		"a table holds an entry for the model":       {table: `{"models":[{"slug":"` + model + `"}]}`, reaches: true},
		"a table holds the model's family":           {table: `{"models":[{"slug":"` + family + `"}]}`, reaches: true},
		"a table cannot be parsed":                   {table: "not a table", reaches: true},
		"a table is no longer there":                 {removed: true, reaches: true},
	} {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			first := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			second := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			cfg, client := windowConfiguration(model, family, first, second), codexAdapter{}
			edited := declareWindow(cfg, 500_000)
			if !client.ProjectionChanged(cfg, edited) {
				t.Fatal("a changed window must count before AIGW wrote a table that says whether it describes the model")
			}
			projectForInspection(t, cfg, "codex")
			switch {
			case test.removed:
				if err := os.Remove(codexCatalogPath(second)); err != nil {
					t.Fatal(err)
				}
			case test.table != "":
				recordTable(t, second, test.table)
			}
			if got := client.ProjectionChanged(cfg, edited); got != test.reaches {
				t.Fatalf("a changed window alone counts as a changed projection = %t, want %t", got, test.reaches)
			}
			moved := edited.Clone()
			provider := moved.Providers["gateway"]
			provider.Endpoints.Responses = "https://elsewhere.test/v1"
			moved.Providers["gateway"] = provider
			if !client.ProjectionChanged(cfg, moved) || client.ProjectionChanged(edited, edited.Clone()) {
				t.Fatal("another difference beside the window must count, and no difference must not")
			}
		})
	}
}

// TestAWindowIsNoChangedProjectionWhereAIGWWritesNoTable holds the same edit
// to a file that names its own model table: AIGW leaves that table alone, so
// no window reaches the file, and another difference still counts.
func TestAWindowIsNoChangedProjectionWhereAIGWWritesNoTable(t *testing.T) {
	const model, family = "grok-4.7-fast", "grok-4.7"
	stubWindowedTable(t)
	target := writeCodexTestConfig(t, "model_catalog_json = \"/elsewhere/own-models.json\"\n")
	cfg, client := windowConfiguration(model, family, target), codexAdapter{}
	edited := declareWindow(cfg, 500_000)
	if client.ProjectionChanged(cfg, edited) {
		t.Fatal("a changed window counts as a changed projection of a file whose own model table AIGW leaves alone")
	}
	moved := edited.Clone()
	provider := moved.Providers["gateway"]
	provider.Endpoints.Responses = "https://elsewhere.test/v1"
	moved.Providers["gateway"] = provider
	if !client.ProjectionChanged(cfg, moved) {
		t.Fatal("another difference beside the window must still count")
	}
}

// TestCodexCatalogProjectionReusesOnlyTheSameClient is the last-known-good gate.
// A copy describes exactly one build, so reuse after an upgrade would override
// the newer bundled table: a worse outcome than the fallback it prevents.
func TestCodexCatalogProjectionReusesOnlyTheSameClient(t *testing.T) {
	dir := t.TempDir()
	target := HomeTarget(filepath.Join(dir, "configuration.toml"))
	target.Executable = filepath.Join(dir, "codex")
	owned := []byte(`{"models":[{"slug":"openai.gpt-5.5"}]}` + "\n")
	before := platform.FileSnapshot{Exists: true, Data: owned, SHA256: platform.SHA256(owned), Mode: 0o600}
	state := codexState{
		CatalogState:         catalogStateProjected,
		CatalogHash:          platform.SHA256(owned),
		CatalogClientVersion: "1.0.0",
		CatalogClientSHA256:  "aaaa",
	}
	original := codexBundledCatalog
	defer func() { codexBundledCatalog = original }()

	// Same identity, regeneration unavailable: the copy still describes the
	// installed build, so it is reused. Its bytes stay for a model that it has
	// no entry for, even one whose family it holds, and its entry for the
	// selected model takes the declared window.
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, nil, fmt.Errorf("dump failed")
	}
	selected := domain.Runtime{UpstreamModel: "openai.gpt-5.5", ContextWindow: domain.DefaultContextWindow}
	variant := domain.Runtime{UpstreamModel: "openai.gpt-5.5-cdx", FamilyID: selected.UpstreamModel, ContextWindow: domain.DefaultContextWindow}
	plan, _ := codexCatalogProjection(target, variant, "", state, before)
	if plan.state != catalogStateProjected || string(plan.data) != string(owned) {
		t.Fatalf("same client, a model without an entry: plan = %+v", plan)
	}
	plan, _ = codexCatalogProjection(target, selected, "", state, before)
	reused, err := modelcatalog.Parse(plan.data)
	if err != nil {
		t.Fatalf("same client, the model's own entry: plan = %+v, %v", plan, err)
	}
	if window, ceiling := reused.ContextWindow(selected.UpstreamModel); plan.state != catalogStateProjected || window != selected.ContextWindow || ceiling != selected.ContextWindow {
		t.Fatalf("same client, the model's own entry: plan = %+v, want the declared window in both fields", plan)
	}

	// Upgraded client, regeneration unavailable: refuse the copy and stop the
	// client, keeping its table, with the fix that names another Codex.
	for _, live := range []ExecutableIdentity{
		{Version: "1.1.0", SHA256: "aaaa"},
		{Version: "1.0.0", SHA256: "bbbb"},
		{},
	} {
		codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
			return live, nil, fmt.Errorf("dump failed")
		}
		var err error
		plan, err = codexCatalogProjection(target, selected, "", state, before)
		if stop, stopped := errors.AsType[*adapter.StopError](err); !stopped || stop.NextAction() != tableFix || !emptyCatalogPlan(plan) {
			t.Fatalf("changed client %+v: plan = %+v, %v; want a stop", live, plan, err)
		}
	}

	// Same identity but the file on disk is no longer the one AIGW recorded
	// writing: it is not AIGW's to reuse.
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, nil, fmt.Errorf("dump failed")
	}
	edited := platform.FileSnapshot{Exists: true, Data: []byte("{}"), SHA256: platform.SHA256([]byte("{}")), Mode: 0o600}
	if _, err := codexCatalogProjection(target, selected, "", state, edited); !errors.As(err, new(*adapter.StopError)) {
		t.Fatalf("edited catalog: %v, want a stop", err)
	}

	// Never owned a catalog here: nothing was lost, so nothing is reported.
	if plan, err := codexCatalogProjection(target, selected, "", codexState{}, platform.FileSnapshot{}); err != nil || !emptyCatalogPlan(plan) {
		t.Fatalf("never owned: plan = %+v, %v", plan, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestKnownBaseModelUsesBundledCatalogForCustomProvider pins the client's
// complete table for a model that the table holds: its own entry carries the
// declared window, and nothing else of the table changes.
func TestKnownBaseModelUsesBundledCatalogForCustomProvider(t *testing.T) {
	const model = "gpt-6.1-sol"
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, model, "gpt-6-luna")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := HomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime(model)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig, runtimeConfig, false); err != nil {
		t.Fatal(err)
	}
	projected := string(mustReadFile(t, path))
	if !strings.Contains(projected, "model_catalog_json = ") || !strings.Contains(projected, "# managed by AIGW") {
		t.Fatalf("known model did not pin client-owned metadata: %s", projected)
	}
	data := mustReadFile(t, codexCatalogPath(path))
	written := writtenTable(t, path)
	window, ceiling := written.ContextWindow(model)
	if got := strings.Join(catalogSlugList(t, data), ","); got != model+",gpt-6-luna" || window != runtimeConfig.ContextWindow || ceiling != runtimeConfig.ContextWindow ||
		len(written.Model(model)) != 3 || len(written.Model("gpt-6-luna")) != 1 {
		t.Fatalf("known model catalog = %s, want the client's table with the model's window %d", data, runtimeConfig.ContextWindow)
	}
	if err := ValidateConfig(path, runtimeConfig); err != nil {
		t.Fatal(err)
	}
}

// TestCodexCatalogProjectionKeepsAnExistingNativeWireModel gives a model that
// the table holds no second entry, whatever family it has in AIGW: its own
// entry carries the window.
func TestCodexCatalogProjectionKeepsAnExistingNativeWireModel(t *testing.T) {
	const (
		canonical = "gpt-6.1-sol"
		wire      = "gpt-6.1-sol-cdx"
		window    = 640_000
	)
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "0.160.0", SHA256: "client-catalog"}, projectionCatalog(wire, "gpt-6-luna"), nil
	}
	target := TargetRef{Path: filepath.Join(t.TempDir(), "config.toml"), Executable: "codex"}
	selected := domain.Runtime{UpstreamModel: wire, FamilyID: canonical, ContextWindow: window}
	plan, _ := codexCatalogProjection(target, selected, "", codexState{}, platform.FileSnapshot{})
	document, err := modelcatalog.Parse(plan.data)
	if err != nil {
		t.Fatal(err)
	}
	got, ceiling := document.ContextWindow(wire)
	if plan.state != catalogStateProjected || strings.Join(catalogSlugList(t, plan.data), ",") != wire+",gpt-6-luna" || got != window || ceiling != window {
		t.Fatalf("existing native wire model was not kept as one entry with its window: plan = %+v", plan)
	}
}
