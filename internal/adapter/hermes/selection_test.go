package hermes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"

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
	if _, changed, err := ObserveSelection(unchanged); err != nil || changed {
		t.Fatalf("unchanged projection observed as a choice: %v, %v", changed, err)
	}
	if _, changed, err := ObserveSelection(filepath.Join(t.TempDir(), "config.yaml")); err != nil || changed {
		t.Fatalf("absent projection observed as a choice: %v, %v", changed, err)
	}
	chosen := projectedSelection(t, desired, "other", "other-model")
	selection, changed, err := ObserveSelection(chosen)
	if err != nil || !changed || selection != (Selection{Provider: "other", Model: "other-model"}) {
		t.Fatalf("native choice = %+v, %v, %v", selection, changed, err)
	}
	if err := os.WriteFile(chosen+adapter.StateSuffix, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ObserveSelection(chosen); err == nil {
		t.Fatal("an invalid ownership record was accepted")
	}
}

// editModel rewrites Hermes' own model block in the file at path and returns
// path. A nil block removes it.
func editModel(t *testing.T, path string, edit func(model *yaml.Node) *yaml.Node) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	setField(root, "model", edit(field(root, "model")))
	edited, err := encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Every state of Hermes' own model block, with everything else that AIGW owns
// as recorded: what following makes of it, and whether an explicit selection
// and a transition that starts from the file's selection may replace it.
func TestTheModelBlockDecidesWhatFollowingAndAnExplicitSelectionDo(t *testing.T) {
	desired := twoProviders()
	set := func(key, value string) func(*yaml.Node) *yaml.Node {
		return func(model *yaml.Node) *yaml.Node { setString(model, key, value); return model }
	}
	drop := func(key string) func(*yaml.Node) *yaml.Node {
		return func(model *yaml.Node) *yaml.Node { setField(model, key, nil); return model }
	}
	scalarModel := func(*yaml.Node) *yaml.Node {
		node := &yaml.Node{}
		node.SetString("other-model")
		return node
	}
	for _, test := range []struct {
		name            string
		file            func(t *testing.T) string
		followed        Selection
		unfollowed      *Selection
		owned           bool
		explicit, shown bool
	}{
		{name: "a listed model of another owned entry", file: func(t *testing.T) string { return projectedSelection(t, desired, "other", "other-model") },
			followed: Selection{Provider: "other", Model: "other-model"}, explicit: true, shown: true},
		{name: "a model that the entry does not list", file: func(t *testing.T) string { return projectedSelection(t, desired, "other", "unlisted-model") },
			unfollowed: &Selection{Provider: "other", Model: "unlisted-model"}, owned: true, explicit: true},
		{name: "another entry's model with the earlier address", file: func(t *testing.T) string {
			return editModel(t, editModel(t, projectedSelection(t, desired, "", ""), set("provider", "other")), set("default", "other-model"))
		}},
		{name: "an edited address", file: func(t *testing.T) string {
			return editModel(t, projectedSelection(t, desired, "", ""), set("base_url", "http://127.0.0.1:8792/v1"))
		}},
		{name: "a removed mode", file: func(t *testing.T) string {
			return editModel(t, projectedSelection(t, desired, "", ""), drop("api_mode"))
		}},
		{name: "a removed model", file: func(t *testing.T) string {
			return editModel(t, projectedSelection(t, desired, "", ""), drop("default"))
		}},
		{name: "a provider that AIGW does not own", file: func(t *testing.T) string {
			return editModel(t, projectedSelection(t, desired, "", ""), set("provider", "foreign"))
		}, unfollowed: &Selection{Provider: "foreign", Model: desired.SelectedModel}, explicit: true},
		{name: "no model block", file: func(t *testing.T) string {
			return editModel(t, projectedSelection(t, desired, "", ""), func(*yaml.Node) *yaml.Node { return nil })
		}, unfollowed: &Selection{}, explicit: true},
		{name: "a model that is one word", file: func(t *testing.T) string { return editModel(t, projectedSelection(t, desired, "", ""), scalarModel) },
			unfollowed: &Selection{}, explicit: true},
		{name: "a listed model beside an edited provider entry", file: func(t *testing.T) string {
			return editedProvider(t, projectedSelection(t, desired, "other", "other-model"), "other")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := test.file(t)
			selection, changed, err := ObserveSelection(path)
			unfollowed, refused := errors.AsType[*unfollowedSelectionError](err)
			switch {
			case test.followed != (Selection{}):
				if err != nil || !changed || selection != test.followed {
					t.Fatalf("following = %+v, %t, %v; want %+v", selection, changed, err, test.followed)
				}
			case test.unfollowed != nil:
				if !refused || changed || unfollowed.Selection != *test.unfollowed || unfollowed.owned != test.owned {
					t.Fatalf("following = %t, %v; want the unfollowed selection %+v (owned %t)", changed, err, *test.unfollowed, test.owned)
				}
			default:
				if !errors.Is(err, errExternalEdit) || changed {
					t.Fatalf("following = %t, %v; want the outside edit", changed, err)
				}
			}
			before, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			// Synchronization that does not start from the file's selection refuses every such file.
			if _, err := Prepare(path, &desired); !errors.Is(err, errExternalEdit) {
				t.Fatalf("a transition from the record = %v, want the outside edit", err)
			}
			_, err = PrepareTransition(path, &desired, nil, true)
			if test.explicit != (err == nil) || (err != nil && !errors.Is(err, errExternalEdit)) {
				t.Fatalf("an explicit selection = %v, want admitted: %t", err, test.explicit)
			}
			// A transition, and a withdrawal, that start from the file's own
			// selection are admitted only for a choice that AIGW follows.
			shown := classifiedSelection(t, path)
			for name, projection := range map[string]*Desired{"transition": &desired, "withdrawal": nil} {
				_, err = PrepareTransition(path, projection, &shown, false)
				if test.shown != (err == nil) || (err != nil && !errors.Is(err, errExternalEdit)) {
					t.Fatalf("a %s from the file's selection = %v, want admitted: %t", name, err, test.shown)
				}
			}
			if after, readErr := os.ReadFile(path); readErr != nil || string(after) != string(before) {
				t.Fatalf("planning changed the file: %v", readErr)
			}
		})
	}
}

// classifiedSelection reads the selection that the file at path names.
func classifiedSelection(t *testing.T, path string) Selection {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	model := field(root, "model")
	return Selection{Provider: scalar(field(model, "provider")), Model: scalar(field(model, "default"))}
}

// A transition that starts from Hermes' own choice keeps the file's bytes
// when it selects the same model, and replaces the selection when it selects
// another: the configuration, not the record, says where the file stands.
func TestATransitionFromTheFilesChoiceKeepsOrReplacesIt(t *testing.T) {
	previous := twoProviders()
	chosen := projectedSelection(t, previous, "other", "other-model")
	shown := Selection{Provider: "other", Model: "other-model"}
	followed := previous
	followed.SelectedProvider, followed.SelectedModel = shown.Provider, shown.Model
	plan, err := PrepareTransition(chosen, &followed, &shown, false)
	if err != nil || plan.Action != "write" || plan.removeConfig || !plan.keepConfig {
		t.Fatalf("followed plan = %s, keeps the file: %t, %v", plan.Action, plan.keepConfig, err)
	}
	before, err := os.ReadFile(chosen)
	if err != nil {
		t.Fatal(err)
	}
	if string(plan.configAfter) != string(before) {
		t.Fatal("following would rewrite Hermes' own file")
	}
	// A selection that the file does not show is no starting point.
	other := Selection{Provider: previous.SelectedProvider, Model: previous.SelectedModel}
	if _, err := PrepareTransition(chosen, &followed, &other, false); !errors.Is(err, errExternalEdit) {
		t.Fatalf("a transition from a selection that the file does not show = %v", err)
	}
	// From the file's choice, a transition to another selection writes it.
	plan, err = PrepareTransition(chosen, &previous, &shown, false)
	if err != nil || plan.Action != "write" || plan.keepConfig {
		t.Fatalf("a transition away from the choice = %s, keeps the file: %t, %v", plan.Action, plan.keepConfig, err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if settled, err := Prepare(chosen, &previous); err != nil || settled.Action != "unchanged" {
		t.Fatalf("the written selection did not settle: %s, %v", settled.Action, err)
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

// An explicit selection replaces a selection that names an unlisted model,
// and the file then matches the record again.
func TestExplicitSelectionReplacesAnUnlistedModel(t *testing.T) {
	previous := twoProviders()
	unlisted := projectedSelection(t, previous, "other", "unlisted-model")
	plan, err := PrepareTransition(unlisted, &previous, nil, true)
	if err != nil || plan.Action != "write" {
		t.Fatalf("explicit selection over an unlisted model = %s, %v", plan.Action, err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if restored, err := Prepare(unlisted, &previous); err != nil || restored.Action != "unchanged" {
		t.Fatalf("the explicit selection was not written: %s, %v", restored.Action, err)
	}
}
