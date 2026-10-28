package hermes

import (
	"os"
	"path/filepath"
	"testing"
)

// projectedSelection projects desired, then rewrites Hermes' own selection to
// the named provider and model, as Hermes' model picker does.
func projectedSelection(t *testing.T, desired Desired, provider, model string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if provider == "" {
		return path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
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
	return path
}

func twoProviders() Desired {
	desired := testDesired()
	other := desired.Providers[0]
	other.ID, other.Endpoint, other.Models = "other", "https://other.test/v1", []string{"other-model"}
	desired.Providers = append(desired.Providers, other)
	return desired
}

func TestObserveSelectionReportsOnlyAProvedNativeChoice(t *testing.T) {
	desired := twoProviders()
	unchanged := projectedSelection(t, desired, "", "")
	if _, changed, err := ObserveSelection(unchanged, desired); err != nil || changed {
		t.Fatalf("unchanged projection observed as a choice: %v, %v", changed, err)
	}
	if _, changed, err := ObserveSelection(filepath.Join(t.TempDir(), "config.yaml"), desired); err != nil || changed {
		t.Fatalf("absent projection observed as a choice: %v, %v", changed, err)
	}
	chosen := projectedSelection(t, desired, "other", "other-model")
	selection, changed, err := ObserveSelection(chosen, desired)
	if err != nil || !changed || selection != (Selection{Provider: "other", Model: "other-model"}) {
		t.Fatalf("native choice = %+v, %v, %v", selection, changed, err)
	}
	foreign := projectedSelection(t, desired, "other", "unlisted-model")
	if _, _, err := ObserveSelection(foreign, desired); err == nil {
		t.Fatal("a model outside the projected catalogue was observed as a choice")
	}
	invalid := desired
	invalid.SelectedModel = ""
	if _, _, err := ObserveSelection(chosen, invalid); err == nil {
		t.Fatal("an invalid current projection was accepted")
	}
	if err := os.WriteFile(chosen+stateSuffix, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ObserveSelection(chosen, desired); err == nil {
		t.Fatal("an invalid ownership record was accepted")
	}
}

func TestAdoptedSelectionNeverReplacesTheNativeChoice(t *testing.T) {
	previous := twoProviders()
	chosen := projectedSelection(t, previous, "other", "other-model")
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
	if string(plan.configAfter) != string(before) {
		t.Fatal("adoption would rewrite Hermes' own file")
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
}
