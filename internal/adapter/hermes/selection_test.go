package hermes

import (
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
)

// projectedSelection projects desired, turns the file and its record into the
// shape that rewrite gives, then rewrites Hermes' own selection to the named
// provider and model, as Hermes' model picker does.
func projectedSelection(t *testing.T, desired Desired, rewrite func(*testing.T, string), provider, model string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	rewrite(t, path)
	if provider != "" {
		selectInHermes(t, path, provider, model)
	}
	return path
}

// selectInHermes rewrites the selection of the Hermes configuration at path to
// the named provider entry and model, as Hermes' model picker does.
func selectInHermes(t *testing.T, path, provider, model string) {
	t.Helper()
	root := projectedFile(t, path)
	selected := field(field(root, "providers"), provider)
	setString(field(root, "model"), "provider", provider)
	setString(field(root, "model"), "default", model)
	setString(field(root, "model"), "base_url", field(selected, "base_url").Value)
	setString(field(root, "model"), "api_mode", field(selected, "transport").Value)
	drifted, err := encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, drifted, 0o600); err != nil {
		t.Fatal(err)
	}
}

func twoProviders() Desired {
	desired := testDesired()
	other := desired.Providers[0]
	other.ID, other.Endpoint, other.Models = "other", "https://other.test/v1", []Model{{ID: "other-model", ContextWindow: 256000}}
	desired.Providers = append(desired.Providers, other)
	return desired
}

func TestObserveSelectionReportsOnlyAProvedNativeChoice(t *testing.T) {
	for _, shape := range modelShapes {
		t.Run(shape.name, func(t *testing.T) {
			desired := twoProviders()
			unchanged := projectedSelection(t, desired, shape.rewrite, "", "")
			if _, changed, err := ObserveSelection(unchanged, desired); err != nil || changed {
				t.Fatalf("unchanged projection observed as a choice: %v, %v", changed, err)
			}
			if _, changed, err := ObserveSelection(filepath.Join(t.TempDir(), "config.yaml"), desired); err != nil || changed {
				t.Fatalf("absent projection observed as a choice: %v, %v", changed, err)
			}
			chosen := projectedSelection(t, desired, shape.rewrite, "other", "other-model")
			selection, changed, err := ObserveSelection(chosen, desired)
			if err != nil || !changed || selection != (Selection{Provider: "other", Model: "other-model"}) {
				t.Fatalf("native choice = %+v, %v, %v", selection, changed, err)
			}
			// The empty ID is the value of every node that is not a scalar.
			for _, unlisted := range []string{"unlisted-model", ""} {
				foreign := projectedSelection(t, desired, shape.rewrite, "other", unlisted)
				if _, _, err := ObserveSelection(foreign, desired); err == nil {
					t.Fatalf("the model %q outside the projected catalog was observed as a choice", unlisted)
				}
			}
			invalid := desired
			invalid.SelectedModel = ""
			if _, _, err := ObserveSelection(chosen, invalid); err == nil {
				t.Fatal("an invalid current projection was accepted")
			}
			if err := os.WriteFile(chosen+adapter.StateSuffix, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := ObserveSelection(chosen, desired); err == nil {
				t.Fatal("an invalid ownership record was accepted")
			}
		})
	}
}

func TestAdoptedSelectionNeverReplacesTheNativeChoice(t *testing.T) {
	for _, shape := range modelShapes {
		t.Run(shape.name, func(t *testing.T) {
			previous := twoProviders()
			chosen := projectedSelection(t, previous, shape.rewrite, "other", "other-model")
			adopted := previous
			adopted.SelectedProvider, adopted.SelectedModel = "other", "other-model"
			plan, err := PrepareAdoptedSelection(chosen, adopted, previous)
			if err != nil || plan.Action != "write" || plan.removeConfig {
				t.Fatalf("adopted plan = %s, %v", plan.Action, err)
			}
			before, err := os.ReadFile(chosen)
			if err != nil {
				t.Fatal(err)
			}
			if shape.kept && string(plan.configAfter) != string(before) {
				t.Fatal("adoption would rewrite Hermes' own file")
			}
			planned, err := parse(plan.configAfter)
			if err != nil || !adopted.selects(planned) {
				t.Fatalf("adoption would replace the native choice: %v\n%s", err, plan.configAfter)
			}
			// A desired selection that differs from the observed native one is refused.
			if _, err := PrepareAdoptedSelection(chosen, previous, previous); err == nil {
				t.Fatal("adoption replaced the native choice with another selection")
			}
			invalid := previous
			invalid.SelectedModel = ""
			if _, err := PrepareAdoptedSelection(chosen, adopted, invalid); err == nil {
				t.Fatal("adoption accepted an invalid previous projection")
			}
		})
	}
}
