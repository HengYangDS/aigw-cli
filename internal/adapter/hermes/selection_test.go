package hermes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
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
	// A model that the entry does not list is no choice to follow, and it is
	// told apart from an edit to another owned setting.
	foreign := projectedSelection(t, desired, "other", "unlisted-model")
	_, changed, err = ObserveSelection(foreign, desired)
	if unlisted, typed := errors.AsType[*unlistedSelectionError](err); !typed || changed || unlisted.Selection != (Selection{Provider: "other", Model: "unlisted-model"}) {
		t.Fatalf("a model outside the projected catalog = %v, %v", changed, err)
	}
	edited := editedProvider(t, projectedSelection(t, desired, "other", "unlisted-model"), "other")
	if _, _, err := ObserveSelection(edited, desired); !errors.Is(err, errExternalEdit) {
		t.Fatalf("an unlisted model beside an edited provider entry = %v, want the outside edit", err)
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

// editedProvider changes the credential command of an owned provider entry
// in the file at path, an edit outside the selection, and returns path.
func editedProvider(t *testing.T, path, provider string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	setString(field(field(root, "providers"), provider), "key_cmd", "foreign-reader")
	edited, err := encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// An explicit selection needs only that nothing but the selection changed: it
// replaces a selection that names an unlisted model, which following refuses.
func TestExplicitSelectionReplacesAnUnlistedModel(t *testing.T) {
	previous := twoProviders()
	unlisted := projectedSelection(t, previous, "other", "unlisted-model")
	if _, err := Prepare(unlisted, &previous); !errors.Is(err, errExternalEdit) {
		t.Fatalf("ordinary synchronization over an unlisted model = %v", err)
	}
	if _, err := PrepareAdoptedSelection(unlisted, previous, previous); err == nil {
		t.Fatal("following replaced an unlisted model")
	}
	plan, err := PrepareAuthorizedSelection(unlisted, previous, previous)
	if err != nil || plan.Action != "write" {
		t.Fatalf("explicit selection over an unlisted model = %s, %v", plan.Action, err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if restored, err := Prepare(unlisted, &previous); err != nil || restored.Action != "unchanged" {
		t.Fatalf("the explicit selection was not written: %s, %v", restored.Action, err)
	}
	edited := editedProvider(t, projectedSelection(t, previous, "other", "unlisted-model"), "other")
	if _, err := PrepareAuthorizedSelection(edited, previous, previous); !errors.Is(err, errExternalEdit) {
		t.Fatalf("explicit selection beside an edited provider entry = %v, want the outside edit", err)
	}
}
