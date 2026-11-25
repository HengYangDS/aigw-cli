package cli_test

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"strconv"
	"testing"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"

	"go.yaml.in/yaml/v4"
)

// hermesWindows reads Hermes' file at target: the context window that the
// provider entry holds for each model that it lists, by model ID.
func hermesWindows(t *testing.T, target, entry string) map[string]int {
	t.Helper()
	var projected struct {
		Providers map[string]struct {
			Models map[string]struct {
				ContextLength int `yaml:"context_length"`
			} `yaml:"models"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(t, target), &projected); err != nil {
		t.Fatalf("Hermes' file does not list each entry's models as a mapping: %v\n%s", err, readFile(t, target))
	}
	windows := make(map[string]int)
	for id, model := range projected.Providers[entry].Models {
		windows[id] = model.ContextLength
	}
	return windows
}

// A model chosen in Hermes, then an edit of a window: the command works from
// the configuration that follows the choice, so the window lands under the
// chosen model and Hermes keeps its own selection.
func TestModelEditAfterAModelChosenInHermesKeepsTheChoice(t *testing.T) {
	t.Parallel()
	app, output, _, target := shippedHermesApp(t)
	const entry = "aigw-dmxapi-anthropic"
	// The chosen model is the first model, by ID, that the entry lists.
	chosen := ""
	for id := range hermesWindows(t, target, entry) {
		if chosen == "" || id < chosen {
			chosen = id
		}
	}
	if chosen == "" {
		t.Fatalf("Hermes' entry %s lists no model", entry)
	}
	hermesNativeSelection(t, target, entry, chosen, nil)
	output.Reset()
	if err := cli.Execute(app, []string{"model", "edit", "dmxapi/" + chosen, "--context-window", "512000"}); err != nil {
		t.Fatalf("model edit after a model chosen in Hermes: %v\n%s", err, output)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != entry || model != chosen {
		t.Fatalf("the edit replaced the model chosen in Hermes: %s %s", provider, model)
	}
	if got := hermesWindows(t, target, entry)[chosen]; got != 512000 {
		t.Fatalf("Hermes' window for the chosen model = %d, want 512000", got)
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.SelectedModel(hermes.ID) != domain.NewModelRef("dmxapi", chosen) {
		t.Fatalf("Hermes binding after the edit: %#v, %v", cfg.Clients, err)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatalf("sync --dry-run --json after the edit: %v\n%s", err, output)
	}
	var preview struct {
		Files []struct {
			Client string `json:"client"`
			Action string `json:"action"`
		} `json:"client_files"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatalf("the preview: %v\n%s", err, output)
	}
	planned := 0
	for _, file := range preview.Files {
		if file.Client != hermes.ID {
			continue
		}
		planned++
		if file.Action != "unchanged" {
			t.Fatalf("the preview after the edit plans %s for Hermes' file", file.Action)
		}
	}
	if planned != 1 {
		t.Fatalf("the preview names %d files of Hermes, want one:\n%s", planned, output)
	}
}

// Scenario "The declared window changes" of the requirement "Hermes runs each
// model at its declared context window": `aigw model edit --context-window`
// writes the new number under the model's ID in the file of an enabled Hermes,
// for the selected model and for another model that the entry lists, and the
// preview of a synchronization then plans no change.
func TestModelEditWritesTheWindowIntoHermesFile(t *testing.T) {
	t.Parallel()
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "hermes", "config.yaml")
	executable := executableFixture(t, "hermes")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: executable},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", ConfigPath: target}},
	}}
	cfg := domain.NewConfig(clients.Admission())
	endpoints := domain.Endpoints{Anthropic: "https://team.test"}
	selected := addProviderModel(&cfg, "team", "Team", endpoints, hermes.ID, "selected")
	addProviderModel(&cfg, "team", "Team", endpoints, hermes.ID, "listed")
	cfg.SetSelectedModel(hermes.ID, selected, "")
	cfg.SetClientActivation(hermes.ID, true, executable, []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	const entry = "aigw-team-anthropic"
	want := map[string]int{"selected": domain.DefaultContextWindow, "listed": domain.DefaultContextWindow}
	if got := hermesWindows(t, target, entry); !maps.Equal(got, want) {
		t.Fatalf("Hermes' windows after the first synchronization = %v, want %v", got, want)
	}
	for _, edit := range []struct {
		model  string
		window int
	}{{"selected", 512000}, {"listed", 256000}} {
		out.Reset()
		if err := cli.Execute(app, []string{"model", "edit", "team/" + edit.model, "--context-window", strconv.Itoa(edit.window)}); err != nil {
			t.Fatalf("model edit team/%s --context-window %d: %v\n%s", edit.model, edit.window, err, out)
		}
		want[edit.model] = edit.window
		if got := hermesWindows(t, target, entry); !maps.Equal(got, want) {
			t.Fatalf("Hermes' windows after the edit of team/%s = %v, want %v", edit.model, got, want)
		}
		out.Reset()
		if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
			t.Fatalf("sync --dry-run --json: %v\n%s", err, out)
		}
		var preview struct {
			Files []struct {
				Client string `json:"client"`
				Action string `json:"action"`
			} `json:"client_files"`
		}
		if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
			t.Fatalf("the preview: %v\n%s", err, out)
		}
		planned := 0
		for _, file := range preview.Files {
			if file.Client != hermes.ID {
				continue
			}
			planned++
			if file.Action != "unchanged" {
				t.Fatalf("the preview after the edit of team/%s plans %s for Hermes' file", edit.model, file.Action)
			}
		}
		if planned != 1 {
			t.Fatalf("the preview names %d files of Hermes, want one:\n%s", planned, out)
		}
	}
}
