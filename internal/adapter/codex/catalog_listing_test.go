package codex

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// listingTable is a stand-in for Codex's table with its visibility field.
const listingTable = `{"models":[` +
	`{"slug":"gpt-6.2-sol","display_name":"GPT-6.2-Sol","visibility":"list","context_window":272000},` +
	`{"slug":"gpt-6.1-terra","display_name":"GPT-6.1 Terra","visibility":"list","context_window":272000},` +
	`{"slug":"gpt-5.5","display_name":"GPT-5.5","visibility":"list"},` +
	`{"slug":"codex-auto-review","display_name":"Codex Auto Review","visibility":"hide"}]}`

// listingConfiguration selects gateway/gpt-6.2-sol-cdx for Codex at targets,
// beside the family's own model, a model of another family that Codex's table
// holds, and one that it does not describe.
func listingConfiguration(t *testing.T, targets ...string) domain.Config {
	t.Helper()
	cfg := domain.NewConfig(testAdmission())
	if err := cfg.AppendProvider("gateway", domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}); err != nil {
		t.Fatal(err)
	}
	for _, model := range []struct{ id, family, label string }{
		{"gpt-6.2-sol-cdx", "gpt-6.2-sol", "Gateway · GPT-6.2 Sol · CDX"},
		{"gpt-6.2-sol", "gpt-6.2-sol", "Gateway · GPT-6.2 Sol"},
		{"gpt-6.1-terra", "gpt-6.1-terra", "Gateway · GPT-6.1 Terra"},
		{"deepseek-v4.1", "deepseek-v4.1", "Gateway · DeepSeek"},
	} {
		entry := qualifiedModel(model.label, "gateway", model.id, domain.ProtocolResponses)
		entry.Family = model.family
		if err := cfg.AppendModel(entry); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Clients[ID] = domain.ClientBinding{Model: domain.NewModelRef("gateway", "gpt-6.2-sol-cdx"), Enabled: true, Program: "/opt/codex", Targets: targets}
	return cfg
}

func stubListingTable(t *testing.T) {
	t.Helper()
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, []byte(listingTable), nil
	}
}

// Codex's table lists the selection's family models that it describes, the
// selected channel by its alias and the other family by its own entry, and
// hides every other entry that the client lists.
func TestCodexTableListsTheProvidersFamilyModels(t *testing.T) {
	stubListingTable(t)
	cfg := listingConfiguration(t)
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	target := TargetRef{Path: writeCodexTestConfig(t, "model_provider = \"native\"\n"), Executable: "codex"}
	plan, err := codexCatalogProjection(target, runtime, "", codexState{}, platform.FileSnapshot{})
	if err != nil {
		t.Fatal(err)
	}
	table, err := modelcatalog.Parse(plan.data)
	if err != nil {
		t.Fatal(err)
	}
	if listed := table.Listed(); !slices.Equal(listed, []string{"gpt-6.1-terra", "gpt-6.2-sol-cdx"}) {
		t.Fatalf("listed entries = %q, want the other family's entry and the selected channel's alias", listed)
	}
	want := map[string]string{"gpt-6.2-sol": "hide", "gpt-6.1-terra": "list", "gpt-5.5": "hide", "codex-auto-review": "hide", "gpt-6.2-sol-cdx": "list"}
	if shown := listingVisibilities(t, plan.data); !maps.Equal(shown, want) {
		t.Fatalf("visibilities = %v, want %v", shown, want)
	}
	for _, slug := range []string{"gpt-6.1-terra", "gpt-6.2-sol-cdx"} {
		if window, ceiling := table.ContextWindow(slug); window != domain.DefaultContextWindow || ceiling != domain.DefaultContextWindow {
			t.Errorf("%s carries %d and %d, want the declared window", slug, window, ceiling)
		}
	}
}

// A change of the listed models counts as a changed projection where the table
// that AIGW wrote describes a model that changed, and not where it describes
// none of them.
func TestAListingChangeReachesCodexWhereTheTableDescribesIt(t *testing.T) {
	stubListingTable(t)
	target := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	cfg, client := listingConfiguration(t, target), codexAdapter{}
	projectForInspection(t, cfg, "codex")
	edit := func(id string, change func(*domain.Model)) domain.Config {
		edited := cfg.Clone()
		model := edited.Models[domain.NewModelRef("gateway", id)]
		change(&model)
		edited.Models[model.Ref()] = model
		return edited
	}
	for name, test := range map[string]struct {
		edited  domain.Config
		reaches bool
	}{
		"a described model's window": {edited: edit("gpt-6.1-terra", func(model *domain.Model) { model.ContextWindow = 500_000 }), reaches: true},
		// Counted although Terra's own entry keeps the client's name: the
		// table that AIGW wrote holds aliases beside own entries and cannot
		// tell them apart, so the cost is one more read of Codex's table.
		"a described model's label, own entry": {edited: edit("gpt-6.1-terra", func(model *domain.Model) { model.Label = "Gateway · Terra" }), reaches: true},
		"an undescribed model's label":         {edited: edit("deepseek-v4.1", func(model *domain.Model) { model.Label = "Gateway · DS" })},
		"an unlisted model's window":           {edited: edit("gpt-6.2-sol", func(model *domain.Model) { model.ContextWindow = 500_000 })},
		"a described family added":             {edited: withModel(t, cfg, "gpt-5.5", "Gateway · GPT-5.5"), reaches: true},
		"an undescribed family added":          {edited: withModel(t, cfg, "kimi-k3", "Gateway · Kimi")},
		"a described family removed":           {edited: without(cfg, "gpt-6.1-terra"), reaches: true},
		"an undescribed family removed":        {edited: without(cfg, "deepseek-v4.1")},
		"nothing":                              {edited: cfg.Clone()},
	} {
		t.Run(name, func(t *testing.T) {
			if got := client.ProjectionChanged(cfg, test.edited); got != test.reaches {
				t.Fatalf("ProjectionChanged = %t, want %t", got, test.reaches)
			}
		})
	}
}

// withModel returns cfg with the gateway's model id added as a family of its
// own, after the others.
func withModel(t *testing.T, cfg domain.Config, id, label string) domain.Config {
	t.Helper()
	edited := cfg.Clone()
	model := qualifiedModel(label, "gateway", id, domain.ProtocolResponses)
	model.Family = id
	if err := edited.AppendModel(model); err != nil {
		t.Fatal(err)
	}
	return edited
}

// without returns cfg without the gateway's model id.
func without(cfg domain.Config, id string) domain.Config {
	edited := cfg.Clone()
	delete(edited.Models, domain.NewModelRef("gateway", id))
	return edited
}

// A selection whose models Codex's table describes by neither entry lists
// nothing and keeps every entry of the client's table, hidden.
func TestCodexTableOfUndescribedModelsListsNothing(t *testing.T) {
	stubListingTable(t)
	cfg := listingConfiguration(t)
	binding := cfg.Clients[ID]
	binding.Model = domain.NewModelRef("gateway", "deepseek-v4.1")
	cfg.Clients[ID] = binding
	for _, id := range []string{"gpt-6.2-sol-cdx", "gpt-6.2-sol", "gpt-6.1-terra"} {
		cfg = without(cfg, id)
	}
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	target := TargetRef{Path: writeCodexTestConfig(t, "model_provider = \"native\"\n"), Executable: "codex"}
	plan, err := codexCatalogProjection(target, runtime, "", codexState{}, platform.FileSnapshot{})
	if err != nil {
		t.Fatal(err)
	}
	table, err := modelcatalog.Parse(plan.data)
	if err != nil {
		t.Fatal(err)
	}
	if listed := table.Listed(); len(listed) != 0 {
		t.Fatalf("listed entries = %q, want none", listed)
	}
	want := map[string]string{"gpt-6.2-sol": "hide", "gpt-6.1-terra": "hide", "gpt-5.5": "hide", "codex-auto-review": "hide"}
	if shown := listingVisibilities(t, plan.data); !maps.Equal(shown, want) {
		t.Fatalf("visibilities = %v, want every entry of the client's table, hidden: %v", shown, want)
	}
}

// A changed window of a listed model other than the selected one rewrites
// Codex's table, from the client's table and from the copy that AIGW wrote,
// after which nothing is left to do.
func TestAListedModelsWindowRewritesCodexsTable(t *testing.T) {
	for client, reused := range map[string]bool{"the client prints its table": false, "the written copy is reused": true} {
		t.Run(client, func(t *testing.T) {
			stubListingTable(t)
			path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
			before, deps, adapter := listingConfiguration(t, path), windowDependencies(t, path), codexAdapter{}
			if _, err := adapter.Apply(context.Background(), deps, domain.NewConfig(testAdmission()), before); err != nil {
				t.Fatal(err)
			}
			if reused {
				stubUnprintableTable(t)
			}
			const lowered = 400_000
			model := before.Models[domain.NewModelRef("gateway", "gpt-6.1-terra")]
			after := before.Clone()
			model.ContextWindow = lowered
			after.Models[model.Ref()] = model
			plans, err := adapter.Plan(deps, before, after)
			if err != nil || len(plans) != 1 || !plans[0].ChangesState || !slices.Contains(plans[0].Effects, codexCatalogPath(path)) {
				t.Fatalf("the plan of a changed window = %+v, %v; want a write of the table", plans, err)
			}
			if _, err := adapter.Apply(context.Background(), deps, before, after); err != nil {
				t.Fatal(err)
			}
			written := writtenTable(t, path)
			if window, ceiling := written.ContextWindow("gpt-6.1-terra"); window != lowered || ceiling != lowered {
				t.Fatalf("the table gives gpt-6.1-terra %d and %d, want %d in both", window, ceiling, lowered)
			}
			if listed := written.Listed(); !slices.Equal(listed, []string{"gpt-6.1-terra", "gpt-6.2-sol-cdx"}) {
				t.Fatalf("listed entries after the rewrite = %q", listed)
			}
			plans, err = adapter.Plan(deps, after, after)
			if err != nil || len(plans) != 1 || plans[0].ChangesState || plans[0].Action != string(ProjectionActionAlreadyConverged) {
				t.Fatalf("the plan after the rewrite = %+v, %v; want nothing to do", plans, err)
			}
		})
	}
}

func listingVisibilities(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var parsed struct {
		Models []map[string]json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	shown := map[string]string{}
	for _, entry := range parsed.Models {
		var slug, visibility string
		_ = json.Unmarshal(entry["slug"], &slug)
		_ = json.Unmarshal(entry["visibility"], &visibility)
		shown[slug] = visibility
	}
	return shown
}
