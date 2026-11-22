package codex

import (
	"context"
	"encoding/json"
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
func TestAChangedDeclaredWindowRewritesCodexsTable(t *testing.T) {
	for name, model := range windowModels {
		t.Run(name, func(t *testing.T) {
			stubWindowedTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			before, deps := windowConfiguration(model, "gpt-6.2-sol", path), windowDependencies(t, path)
			client := codexAdapter{}
			if _, err := client.Apply(context.Background(), deps, domain.NewConfig(testAdmission()), before); err != nil {
				t.Fatal(err)
			}
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
			plans, err = client.Plan(deps, after, after)
			if err != nil || len(plans) != 1 || plans[0].ChangesState || plans[0].Action != string(ProjectionActionAlreadyConverged) {
				t.Fatalf("the plan after the rewrite = %+v, %v; want nothing to do", plans, err)
			}
		})
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
	plan := codexCatalogProjection(target, selected, "", codexState{}, platform.FileSnapshot{})
	document, err := modelcatalog.Parse(plan.data)
	if err != nil {
		t.Fatal(err)
	}
	got, ceiling := document.ContextWindow(wire)
	if plan.state != catalogStateProjected || strings.Join(catalogSlugList(t, plan.data), ",") != wire+",gpt-6-luna" || got != window || ceiling != window {
		t.Fatalf("existing native wire model was not kept as one entry with its window: plan = %+v", plan)
	}
}
