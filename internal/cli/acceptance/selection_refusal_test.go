package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"

	"go.yaml.in/yaml/v4"
)

// refusedWithRestoringCommand runs each command, which must fail without
// changing a file in directory or the configuration, and requires its next
// action to be restore, a command that then runs as printed.
func refusedWithRestoringCommand(t *testing.T, app *cli.App, output *bytes.Buffer, client, directory, restore string, commands [][]string) {
	t.Helper()
	refused := ownedFileSnapshot(t, directory, app.Config.Path())
	for _, command := range commands {
		output.Reset()
		err := cli.Execute(app, command)
		var result struct {
			OK         bool   `json:"ok"`
			NextAction string `json:"next_action"`
		}
		if decodeErr := json.Unmarshal(output.Bytes(), &result); decodeErr != nil || err == nil || result.OK {
			t.Fatalf("%v was not refused: %v, %v\n%s", command, err, decodeErr, output)
		}
		if result.NextAction != restore {
			t.Errorf("%v next action = %q, want the restoring command %q\n%s", command, result.NextAction, restore, output)
		}
		if after := ownedFileSnapshot(t, directory, app.Config.Path()); !equalSnapshots(after, refused) {
			t.Fatalf("%v changed client files or owned state while refusing", command)
		}
	}
	// The reports name the same step, and write nothing.
	for _, command := range [][]string{{"check", client, "--depth", "local", "--json"}, {"status", "--json"}} {
		output.Reset()
		_ = cli.Execute(app, command)
		var report struct {
			NextAction string       `json:"next_action"`
			Clients    clientStates `json:"clients"`
		}
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		if entry := report.Clients[client]; entry.NextAction != restore {
			t.Errorf("%v names %q for %s, want the restoring command %q\n%s", command, entry.NextAction, client, restore, output)
		}
		if after := ownedFileSnapshot(t, directory, app.Config.Path()); !equalSnapshots(after, refused) {
			t.Fatalf("%v changed client files or owned state", command)
		}
	}
	output.Reset()
	if err := cli.Execute(app, strings.Fields(restore)[1:]); err != nil {
		t.Fatalf("the restoring command %q: %v\n%s", restore, err, output)
	}
}

// Codex's file names a model that its provider has not added. Nothing is
// followed or overwritten; the Fix restores the recorded selection, and the
// refusal names the command that adds the model that Codex names.
func TestCodexModelThatWasNotAddedIsRefusedWithTheRestoringCommand(t *testing.T) {
	app, output, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Models[domain.NewModelRef("gateway", "gpt-sol")] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-sol"), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project the Codex selection: %v\n%s", err, output)
	}
	projected := string(readFile(t, target))
	writeFile(t, target, []byte(strings.Replace(projected, `model = "gpt-sol" # managed by AIGW`, `model = "gpt-ghost"`, 1)), 0o600)

	refusedWithRestoringCommand(t, app, output, codex.ID, filepath.Dir(target), "aigw use codex gateway/gpt-sol", [][]string{
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"client", "edit", codex.ID, "--request-retries", "2", "--json"},
		{"client", "disable", codex.ID, "--json"},
	})
	if string(readFile(t, target)) != projected {
		t.Fatalf("the restoring command did not write the recorded model:\n%s", readFile(t, target))
	}
	output.Reset()
	writeFile(t, target, []byte(strings.Replace(projected, `model = "gpt-sol" # managed by AIGW`, `model = "gpt-ghost"`, 1)), 0o600)
	err := cli.Execute(app, []string{"sync"})
	if err == nil || !strings.Contains(err.Error(), `Codex selects model "gpt-ghost", which provider "gateway" has not added on OpenAI Responses`) ||
		!strings.Contains(err.Error(), "`aigw use codex gateway/gpt-ghost` adds and selects the model that Codex names") {
		t.Fatalf("the refusal does not name the adding command: %v", err)
	}
	if err := cli.Execute(app, []string{"use", "codex", "gateway/gpt-sol"}); err != nil {
		t.Fatal(err)
	}
	syncAndCheckPass(t, app, output, codex.ID)
}

// Hermes' file names a model that its provider entry does not list. Before
// this rule no command restored the selection: synchronization and an
// explicit selection both refused it as an outside edit.
func TestHermesModelThatIsNotListedIsRefusedWithTheRestoringCommand(t *testing.T) {
	app, output, _, target := shippedHermesApp(t)
	projected := readFile(t, target)
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-unadded", nil)

	refusedWithRestoringCommand(t, app, output, hermes.ID, filepath.Dir(target), "aigw use hermes dmxapi/gpt-6.2-sol --protocol responses", [][]string{
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"provider", "edit", "dmxapi", "--label", "Renamed", "--json"},
		{"client", "disable", hermes.ID, "--json"},
	})
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("the restoring command wrote %s %s", provider, model)
	}
	if !bytes.Equal(readFile(t, target), projected) {
		t.Fatal("the restored Hermes file differs from the one that AIGW wrote")
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// An edited address of Hermes' own selection is an edit of an endpoint, not
// a choice of a model: nothing follows it, and an explicit selection does not
// write over it either.
func TestHermesEditedSelectionAddressStaysAnOutsideEdit(t *testing.T) {
	const recovery = "Undo the outside edit to the AIGW-managed Hermes settings, then run `aigw sync`"
	app, output, _, target := shippedHermesApp(t)
	var document yaml.Node
	if err := yaml.Unmarshal(readFile(t, target), &document); err != nil {
		t.Fatal(err)
	}
	model := document.Content[0].Content[1]
	for index := 0; index+1 < len(model.Content); index += 2 {
		if model.Content[index].Value == "base_url" {
			model.Content[index+1].Value = "http://127.0.0.1:8792/v1"
		}
	}
	edited, err := yaml.Marshal(&document)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, target, edited, 0o600)
	assertRecoveryOnEverySurface(t, app, output, hermes.ID, recovery, filepath.Dir(target), [][]string{
		{"sync", "--json"},
		{"use", hermes.ID, "dmxapi/gpt-6.2-sol", "--protocol", "responses", "--json"},
		{"check", hermes.ID, "--depth", "local", "--json"},
		{"status", "--json"},
	})
}

// A selection that AIGW does not follow reads the same wherever it is
// refused: the problem in the refusal's own words, that nothing changed, and
// the restoring command as the fix; status shows it on the client's row.
func TestARefusedSelectionReadsTheSameOnEverySurface(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-ghost")
	const refusal = `Codex selects model "gpt-ghost", which provider "gateway" has not added on OpenAI Responses; ` +
		"AIGW's selection for Codex is gateway/gpt-sol; `aigw use codex gateway/gpt-ghost` adds and selects the model that Codex names"
	problem := "Problem\n  " + refusal + "\n\nImpact\n  Configuration and client files are unchanged.\n\nFix\n  aigw use codex gateway/gpt-sol\n"
	for _, command := range [][]string{{"sync"}, {"sync", "--dry-run"}, {"client", "edit", codex.ID, "--request-retries", "2"}} {
		output.Reset()
		if err := cli.Execute(app, command); err == nil || !strings.Contains(strings.Join(strings.Fields(output.String()), " "), strings.Join(strings.Fields(problem), " ")) {
			t.Errorf("%v: %v\n%s", command, err, output)
		}
	}
	output.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil ||
		!strings.Contains(output.String(), "gateway/gpt-sol · Invalid · "+refusal) || !strings.Contains(output.String(), "Next\n  aigw use codex gateway/gpt-sol\n") {
		t.Errorf("status: %v\n%s", err, output)
	}
}

// Codex has two files: one names a model that was not added, the other holds
// an outside edit of the settings that AIGW manages. The restoring command
// would be refused for that edit, so no surface names it, in either order of
// the files: each one says to undo the edit. With the edit undone, the
// restoring command is named, and it runs.
func TestCodexFilesWithAnOutsideEditAreToldToUndoItFirst(t *testing.T) {
	const undo = "Undo the outside edit to the AIGW-managed Codex settings, then run `aigw sync`"
	for _, order := range []struct {
		name          string
		model, edited int
	}{{"the edit in the second file", 0, 1}, {"the edit in the first file", 1, 0}} {
		t.Run(order.name, func(t *testing.T) {
			app, output, targets := codexChoiceAppWith(t, "gpt-sol", 2)
			projected := string(readFile(t, targets[order.edited]))
			chooseInCodex(t, targets[order.model], "gpt-sol", "gpt-ghost")
			writeFile(t, targets[order.edited], []byte(strings.Replace(projected, `base_url = "https://gateway.test/v1"`, `base_url = "https://drift.test/v1"`, 1)), 0o600)
			for _, command := range [][]string{
				{"sync", "--json"},
				{"sync", "--dry-run", "--json"},
				{"use", codex.ID, "gateway/gpt-sol", "--json"},
				{"status", "--json"},
				{"check", codex.ID, "--depth", "local", "--json"},
			} {
				output.Reset()
				_ = cli.Execute(app, command)
				var result struct {
					NextAction string `json:"next_action"`
					NextDetail string `json:"next_detail"`
				}
				if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.NextAction != "" || result.NextDetail != undo {
					t.Errorf("%v names %q and %q, want the undoing of the outside edit: %v\n%s", command, result.NextAction, result.NextDetail, err, output)
				}
			}
			writeFile(t, targets[order.edited], []byte(projected), 0o600)
			refusedWithRestoringCommand(t, app, output, codex.ID, filepath.Dir(targets[order.model]), "aigw use codex gateway/gpt-sol", [][]string{{"sync", "--json"}})
			syncAndCheckPass(t, app, output, codex.ID)
		})
	}
}

// Two Codex files that name one added model are one choice, and a command
// that records it writes both files; two that name different added models
// are no choice that AIGW can follow.
func TestCodexFilesAreFollowedOnlyWhenTheyAgree(t *testing.T) {
	app, output, targets := codexChoiceAppWith(t, "gpt-sol", 2)
	chooseInCodex(t, targets[0], "gpt-sol", "gpt-terra")
	// Until a command records the choice, the file that still holds what
	// AIGW last wrote is no fault: the reports say configured.
	for _, command := range [][]string{{"status", "--json"}, {"check", codex.ID, "--depth", "local", "--json"}} {
		if got := followedIn(t, app, output, command...); !slices.Equal(got, codexChoice) {
			t.Fatalf("%v follows %+v\n%s", command, got, output)
		}
		var report struct {
			Clients clientStates `json:"clients"`
		}
		if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Clients[codex.ID].State != "configured" && report.Clients[codex.ID].State != "passed" {
			t.Fatalf("%v reports Codex as %q: %v\n%s", command, report.Clients[codex.ID].State, err, output)
		}
	}
	if got := followedIn(t, app, output, "sync", "--json"); !slices.Equal(got, codexChoice) {
		t.Fatalf("one file's choice follows %+v", got)
	}
	for _, target := range targets {
		if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-terra" # managed by AIGW`) {
			t.Fatalf("%s does not hold the followed model:\n%s", target, written)
		}
	}
	chooseInCodex(t, targets[0], "gpt-terra", "gpt-sol")
	chooseInCodex(t, targets[1], "gpt-terra", "gpt-spare")
	output.Reset()
	err := cli.Execute(app, []string{"sync", "--json"})
	if err == nil || !strings.Contains(output.String(), "has files that select different models") {
		t.Fatalf("files that disagree: %v\n%s", err, output)
	}
}
