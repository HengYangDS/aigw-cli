package hermes

// The tests in this file hold the requirement "Hermes runs each model at its
// declared context window": where AIGW writes a model's window and where it
// writes none, that the models keep AIGW's order, that a file of an earlier
// release is rewritten, and that a changed window is a changed projection.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// lowerWindow is a context window below the default, which wire-z of
// windowConfig declares.
const lowerWindow = 256000

// windowListing is the provider's models in windowConfig, in its order:
// wire-z, which declares lowerWindow, and wire-a, which declares none.
var windowListing = []domain.Model{{ID: "wire-z", ContextWindow: lowerWindow}, {ID: "wire-a"}}

// windowConfig is a configuration in which an enabled Hermes at target selects
// wire-a, which declares no context window, of a provider that lists wire-z
// before it, which declares lowerWindow.
func windowConfig(t *testing.T, target string) domain.Config {
	t.Helper()
	return listingConfig(t, target, windowListing)
}

// listingConfig is windowConfig with the provider's models in the order of
// models.
func listingConfig(t *testing.T, target string, models []domain.Model) domain.Config {
	t.Helper()
	cfg := domain.NewConfig(testAdmission())
	if err := cfg.AppendProvider("gateway", domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}); err != nil {
		t.Fatal(err)
	}
	for _, model := range models {
		model.Provider = "gateway"
		model.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	cfg.SetSelectedModel(ID, domain.NewModelRef("gateway", "wire-a"), "")
	cfg.SetClientActivation(ID, true, "/opt/hermes", []string{target})
	return cfg
}

// windowEntry is the ID of the provider entry that windowConfig projects.
func windowEntry() string { return hermesProviderID("gateway", domain.ProtocolResponses) }

func windowDependencies(t *testing.T) adapter.Dependencies {
	t.Helper()
	return adapter.Dependencies{Secrets: secret.NewMemoryStore(), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
}

// projectHermes applies Hermes' projection of the change from before to after.
func projectHermes(t *testing.T, deps adapter.Dependencies, before, after domain.Config) {
	t.Helper()
	if _, err := (hermesAdapter{}).Apply(t.Context(), deps, before, after); err != nil {
		t.Fatal(err)
	}
}

// plannedAction is the action that Hermes plans for its one file on the change
// from before to after.
func plannedAction(t *testing.T, deps adapter.Dependencies, before, after domain.Config) string {
	t.Helper()
	plans, err := (hermesAdapter{}).Plan(deps, before, after)
	if err != nil || len(plans) != 1 {
		t.Fatalf("Hermes plans = %+v, %v", plans, err)
	}
	return plans[0].Action
}

// Scenario "Every listed model carries its window".
func TestHermesWritesEachModelsDeclaredWindowUnderItsID(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.yaml")
	cfg, deps := windowConfig(t, target), windowDependencies(t)
	projectHermes(t, deps, domain.NewConfig(testAdmission()), cfg)
	root := projectedFile(t, target)
	entry := field(field(root, "providers"), windowEntry())
	want := []Model{{ID: "wire-z", ContextWindow: lowerWindow}, {ID: "wire-a", ContextWindow: domain.DefaultContextWindow}}
	if got := writtenModels(t, entry); !slices.Equal(got, want) {
		t.Fatalf("the entry's models = %+v, want %+v", got, want)
	}
	if field(entry, "context_length") != nil || field(field(root, "model"), "context_length") != nil {
		t.Fatal("AIGW wrote a window on a provider entry or in the model block")
	}
	if action := plannedAction(t, deps, cfg, cfg); action != "unchanged" {
		t.Fatalf("an unchanged configuration plans %s after the write", action)
	}
}

// A window in Hermes' own model block is the user's: a projection keeps it,
// as it keeps that block's other keys that AIGW does not own.
func TestProjectionLeavesAUsersOwnModelWindowAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	own := lines("model:", "  provider: original", "  default: previous-model", "  context_length: 64000")
	if err := os.WriteFile(path, []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	desired := testDesired()
	plan, err := prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	root := projectedFile(t, path)
	if window := field(field(root, "model"), "context_length"); window == nil || window.Value != "64000" {
		t.Fatalf("the user's own model window = %+v", window)
	}
	entry := field(field(root, "providers"), desired.Providers[0].ID)
	if got := writtenModels(t, entry); !slices.Equal(got, desired.Providers[0].Models) {
		t.Fatalf("the entry's models = %+v, want %+v", got, desired.Providers[0].Models)
	}
	if field(entry, "context_length") != nil {
		t.Fatal("AIGW wrote a window on its provider entry")
	}
	if plan, err = prepare(path, &desired); err != nil || plan.Action != "unchanged" {
		t.Fatalf("second pass = %s, %v", plan.Action, err)
	}
}

// The record's digest does not see the order of a mapping's keys, so an entry
// whose models change only their order is written, as provider entries that
// change only their order are.
func TestProjectionWritesAnOrderOnlyChangeOfAProvidersModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	desired := testDesired()
	desired.Providers[0].Models = append(desired.Providers[0].Models, Model{ID: "model-other", ContextWindow: 512000})
	plan, err := prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	slices.Reverse(desired.Providers[0].Models)
	if plan, err = prepare(path, &desired); err != nil || !plan.ChangesState() {
		t.Fatalf("an order-only change of an entry's models = %s, %v; want a write", plan.Action, err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	entry := field(field(projectedFile(t, path), "providers"), desired.Providers[0].ID)
	if got := writtenModels(t, entry); !slices.Equal(got, desired.Providers[0].Models) {
		t.Fatalf("the entry's models = %+v, want %+v", got, desired.Providers[0].Models)
	}
	if plan, err = prepare(path, &desired); err != nil || plan.Action != "unchanged" {
		t.Fatalf("second pass = %s, %v", plan.Action, err)
	}
}

func TestValidateRefusesAModelWhoseWindowIsNotAboveZero(t *testing.T) {
	if err := testDesired().Validate(); err != nil {
		t.Fatalf("a model with a window was refused: %v", err)
	}
	for _, window := range []int{0, -1} {
		desired := testDesired()
		desired.Providers[0].Models[0].ContextWindow = window
		if err := desired.Validate(); err == nil || !strings.Contains(err.Error(), desired.SelectedModel) {
			t.Errorf("a model with the window %d = %v, want a refusal that names the model", window, err)
		}
	}
}

// Scenario "A file that an earlier release wrote": the record proves the list
// of IDs to be AIGW's own, and the projection writes the mapping.
func TestProjectionRewritesTheListOfIDsOfAnEarlierRelease(t *testing.T) {
	desired := twoProviders()
	path := projectedSelectionIn(t, desired, earlierRelease, "", "")
	earlier, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(earlier), "models:\n      - model-test\n") {
		t.Fatalf("the file is not in the earlier shape:\n%s", earlier)
	}
	plan, err := prepare(path, &desired)
	if err != nil || plan.Action != "write" {
		t.Fatalf("the plan for a file of an earlier release = %s, %v", plan.Action, err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	root := projectedFile(t, path)
	for _, provider := range desired.Providers {
		if got := writtenModels(t, field(field(root, "providers"), provider.ID)); !slices.Equal(got, provider.Models) {
			t.Fatalf("the models of %s = %+v, want %+v", provider.ID, got, provider.Models)
		}
	}
	if plan, err = prepare(path, &desired); err != nil || plan.Action != "unchanged" {
		t.Fatalf("second pass = %s, %v", plan.Action, err)
	}
}

// Scenario "The declared window changes", for the selected model and for
// another model that the entry lists.
func TestAChangedWindowAloneIsAChangedProjection(t *testing.T) {
	for name, changed := range map[string]string{"the selected model": "wire-a", "another listed model": "wire-z"} {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "config.yaml")
			before, deps := windowConfig(t, target), windowDependencies(t)
			projectHermes(t, deps, domain.NewConfig(testAdmission()), before)
			after := before.Clone()
			ref := domain.NewModelRef("gateway", changed)
			model := after.Models[ref]
			model.ContextWindow = lowerWindow / 2
			after.Models[ref] = model
			if !(hermesAdapter{}).ProjectionChanged(before, after) {
				t.Fatal("a changed window is not a changed projection")
			}
			if action := plannedAction(t, deps, before, after); action != "write" {
				t.Fatalf("a changed window plans %s, want write", action)
			}
			projectHermes(t, deps, before, after)
			written := writtenModels(t, field(field(projectedFile(t, target), "providers"), windowEntry()))
			if index := slices.IndexFunc(written, func(model Model) bool { return model.ID == changed }); index < 0 || written[index].ContextWindow != lowerWindow/2 {
				t.Fatalf("the entry's models after the change = %+v", written)
			}
			if action := plannedAction(t, deps, after, after); action != "unchanged" || (hermesAdapter{}).ProjectionChanged(after, after) {
				t.Fatalf("an unchanged configuration plans %s after the write", action)
			}
		})
	}
}

// Scenario "An entry's models reordered outside AIGW", which the accepted
// "Other managed settings changed" asks for: the record's digest does not see
// the order of a mapping's keys, so the projection compares the file's order
// with the order that AIGW wrote, the one of the configuration before the
// change. A file whose models were reordered by hand is refused as an outside
// edit by a synchronization, by an edit of a window and by an inspection; a
// file that nobody edited plans nothing; and a configuration whose models
// change their order writes them in the new order.
func TestAHandReorderOfTheModelsOfAnEntryIsAnOutsideEdit(t *testing.T) {
	block := func(models ...domain.Model) string {
		text := "    models:\n"
		for _, model := range models {
			text += fmt.Sprintf("      %s:\n        context_length: %d\n", model.ID, model.Window())
		}
		return text
	}
	written, reordered := block(windowListing...), block(windowListing[1], windowListing[0])
	readTarget := func(t *testing.T, target string) string {
		t.Helper()
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	project := func(t *testing.T) (string, domain.Config, adapter.Dependencies) {
		t.Helper()
		target := filepath.Join(t.TempDir(), "config.yaml")
		cfg, deps := windowConfig(t, target), windowDependencies(t)
		projectHermes(t, deps, domain.NewConfig(testAdmission()), cfg)
		if text := readTarget(t, target); !strings.Contains(text, written) {
			t.Fatalf("the file does not list the models as AIGW writes them:\n%s", text)
		}
		return target, cfg, deps
	}
	t.Run("the models reordered by hand", func(t *testing.T) {
		target, cfg, deps := project(t)
		if err := os.WriteFile(target, []byte(strings.Replace(readTarget(t, target), written, reordered, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		edited := cfg.Clone()
		model := edited.Models[domain.NewModelRef("gateway", "wire-z")]
		model.ContextWindow = lowerWindow / 2
		edited.Models[model.Ref()] = model
		for name, after := range map[string]domain.Config{"a synchronization": cfg, "an edit of a window": edited} {
			if plans, err := (hermesAdapter{}).Plan(deps, cfg, after); !errors.Is(err, errExternalEdit) {
				t.Errorf("%s over a hand reorder = %+v, %v; want the outside-edit refusal", name, plans, err)
			}
		}
		runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
		if err != nil {
			t.Fatal(err)
		}
		if plan, err := hermesInspectionPlan(deps, cfg, runtime, target); !errors.Is(err, errExternalEdit) {
			t.Errorf("the inspection of a hand reorder = %s, %v; want the outside-edit refusal", plan.Action, err)
		}
		if text := readTarget(t, target); !strings.Contains(text, reordered) {
			t.Errorf("the refused plans changed the file:\n%s", text)
		}
	})
	t.Run("nobody edited the file", func(t *testing.T) {
		_, cfg, deps := project(t)
		if action := plannedAction(t, deps, cfg, cfg); action != "unchanged" {
			t.Fatalf("an unedited file plans %s", action)
		}
	})
	t.Run("the configuration reorders the models", func(t *testing.T) {
		target, cfg, deps := project(t)
		after := listingConfig(t, target, []domain.Model{windowListing[1], windowListing[0]})
		if action := plannedAction(t, deps, cfg, after); action != "write" {
			t.Fatalf("a reorder of the configuration plans %s, want write", action)
		}
		projectHermes(t, deps, cfg, after)
		if text := readTarget(t, target); !strings.Contains(text, reordered) {
			t.Fatalf("the file does not list the models in the new order:\n%s", text)
		}
		if action := plannedAction(t, deps, after, after); action != "unchanged" {
			t.Fatalf("an unchanged configuration plans %s after the reorder", action)
		}
	})
}
