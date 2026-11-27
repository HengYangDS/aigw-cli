package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// The tests of this file are the seventh review's eight runs, each against
// the row "chosen" of the lane's state table where the provider of the model
// that the client's file selects has no Token that AIGW can use. A choice is
// recorded there as anywhere: the stored selection and AIGW's record of the
// client's file move to it, and the file is left until the Token is stored.

// chosenWithoutTokenAfter runs the commands on the shipped Hermes fixture,
// deletes the Token of lost, and selects model of the provider entry in
// Hermes's file, as Hermes's own picker does.
func chosenWithoutTokenAfter(t *testing.T, lost, entry, model string, commands ...[]string) (*cli.App, *bytes.Buffer, string, []byte) {
	t.Helper()
	app, output, _, target := shippedHermesAppWith(t, true)
	for _, args := range commands {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	if err := app.Secrets.Delete(lost); err != nil {
		t.Fatal(err)
	}
	return app, output, target, hermesNativeSelection(t, target, entry, model, nil)
}

// followUp is the next step of a command's JSON document.
type followUp struct {
	NextAction string `json:"next_action"`
	NextDetail string `json:"next_detail"`
}

// rolledBack is what a rollback's document says of the clients' selections:
// those that it restores, and the choices that it records.
type rolledBack struct {
	Selections []restoredSelection `json:"selections"`
	Followed   followedModels      `json:"followed_selections"`
}

// restores reports whether the rollback names a restored selection of client.
func (result rolledBack) restores(client string) bool {
	return slices.ContainsFunc(result.Selections, func(entry restoredSelection) bool { return entry.Client == client })
}

// A stored selection may name no protocol, as a team setup stores it. A
// rollback to such a selection of the model that Codex's file already shows
// meets that choice: it records it with AIGW's record of the file, names it
// as a followed selection and not as a restored one, and leaves the file
// while the provider has no Token. The model that the record named before,
// chosen in Codex again, is then a choice that synchronization keeps. Holds
// the cell chosen/restores where the restored configuration selects the
// file's model (the review's F1).
func TestARollbackToAnUnpinnedSelectionOfTheChosenModelRecordsTheChoice(t *testing.T) {
	t.Parallel()
	app, output, secretStore, _, _ := testApp(t, "")
	bindCredentialReader(t, app)
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	sol, terra := domain.NewModelRef("gateway", "gpt-sol"), domain.NewModelRef("gateway", "gpt-terra")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Models[sol] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolResponses)
	cfg.Models[terra] = qualifiedModel("Terra", "gateway", "gpt-terra", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, sol, "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	// As a team setup stores a recommendation that names no protocol.
	unpinned := cfg.Clients[codex.ID]
	unpinned.Protocol = ""
	cfg.Clients[codex.ID] = unpinned
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"sync"}, {"use", "codex", terra.String()}} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	if err := secretStore.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	chosen := chooseInCodex(t, target, "gpt-terra", "gpt-sol")
	for _, command := range [][]string{
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
		{"config", "rollback", "--last-change", "--json"},
	} {
		var result rolledBack
		if err := runDocument(t, app, output, &result, command...); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		if result.restores(codex.ID) || result.Followed[codex.ID] != sol.String() {
			t.Fatalf("%v restores %+v and follows %v; want Codex's choice of %s followed and not restored", command, result.Selections, result.Followed, sol)
		}
	}
	if string(readFile(t, target)) != chosen {
		t.Fatal("the rollback wrote the file of a client whose provider has no Token")
	}
	if binding := selection(t, app, codex.ID); binding.Model != sol {
		t.Fatalf("the rollback saved %s; Codex's file selects %s, which the restored configuration holds", binding.Model, sol)
	}
	// The pick cannot run, so the person goes back to the model that AIGW's
	// record of the file named before.
	writeFile(t, target, []byte(strings.Replace(chosen, `model = "gpt-sol"`, `model = "gpt-terra"`, 1)), 0o600)
	if err := secretStore.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := runDocument(t, app, output, nil, "sync", "--json"); err != nil {
		t.Fatalf("sync after the Token: %v\n%s", err, output)
	}
	if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-terra"`) {
		t.Errorf("synchronization wrote over the model chosen last:\n%s", written)
	}
}

// A rollback to another model than the one that Hermes's file shows writes
// the file with it while that model's provider has its Token, so AIGW's
// record moves with the stored selection, and the model chosen in Hermes
// afterwards is a choice that synchronization keeps. Holds the cell
// chosen/restores where the restored configuration selects another model
// (the review's F2).
func TestARollbackOverAChoiceDoesNotLoseTheNextChoice(t *testing.T) {
	t.Parallel()
	app, output, target, _ := chosenWithoutTokenAfter(t, "ucloud", "aigw-ucloud-anthropic", "claude-opus-5-5",
		[]string{"use", "hermes", "dmxapi/claude-opus-5-5", "--protocol", "anthropic"},
		[]string{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses"})
	var result rolledBack
	if err := runDocument(t, app, output, &result, "config", "rollback", "--last-change", "--json"); err != nil {
		t.Fatalf("rollback: %v\n%s", err, output)
	}
	if !result.restores(hermes.ID) || len(result.Followed) != 0 {
		t.Fatalf("the rollback restores %+v and follows %v; want Hermes's selection restored over its choice", result.Selections, result.Followed)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("the rollback left %s %s in Hermes's file; the restored model's provider has its Token", provider, model)
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := runDocument(t, app, output, nil, "sync", "--json"); err != nil {
		t.Fatalf("sync after the Token: %v\n%s", err, output)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("synchronization wrote %s %s over the model chosen last", provider, model)
	}
}

// A rollback names as restored only a selection that it saves in place of
// another: where the restored configuration selects the model that Hermes's
// file already shows, the preview and the result name that choice as
// followed, the result saves it, and neither lists it among the restored
// selections. Holds the cell chosen/restores (the review's F3).
func TestARollbackNamesOnlyTheSelectionThatItSaves(t *testing.T) {
	t.Parallel()
	const chosenModel = "ucloud/claude-opus-5-5"
	app, output, _, _ := chosenWithoutTokenAfter(t, "ucloud", "aigw-ucloud-anthropic", "claude-opus-5-5",
		[]string{"use", "hermes", chosenModel, "--protocol", "anthropic"},
		[]string{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses"})
	for _, command := range [][]string{
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
		{"config", "rollback", "--last-change", "--json"},
	} {
		var result rolledBack
		if err := runDocument(t, app, output, &result, command...); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		stored := selection(t, app, hermes.ID).Model.String()
		for _, restored := range result.Selections {
			if restored.Client == hermes.ID {
				t.Errorf("%v names %s as Hermes's restored selection, in place of %s; the stored selection is %s", command, restored.Model, restored.Replaces, stored)
			}
		}
		if result.Followed[hermes.ID] != chosenModel {
			t.Errorf("%v follows %v; Hermes's file selects %s, which the restored configuration holds", command, result.Followed, chosenModel)
		}
	}
	if stored := selection(t, app, hermes.ID).Model.String(); stored != chosenModel {
		t.Errorf("the rollback saved %s as Hermes's selection, and named %s as followed", stored, chosenModel)
	}
}

// An edit of the configuration files of a client whose provider has no Token
// is saved on the chosen model, as with the Token: AIGW withdraws its
// settings and record from the file that the edit drops, which needs no
// Token, so nothing of AIGW's stays behind in a file that the configuration
// no longer names. The file that Hermes moves to waits for the Token, which
// the edit's next step names. Holds the cell chosen/edits (the review's F4,
// changed by the Change one-token-usability).
func TestATargetEditOfAClientWithoutItsTokenLeavesNothingBehind(t *testing.T) {
	t.Parallel()
	const chosenModel = "ucloud/claude-opus-5-5"
	app, output, target, _ := waitingHermes(t)
	moved := filepath.Join(t.TempDir(), "hermes", "config.yaml")
	var edited followUp
	if err := runDocument(t, app, output, &edited, "client", "edit", "hermes", "--target", moved, "--json"); err != nil {
		t.Fatalf("the edit = %v\n%s", err, output)
	}
	if binding := mustLoad(t, app).Clients[hermes.ID]; !slices.Equal(binding.Targets, []string{moved}) || binding.Model.String() != chosenModel {
		t.Fatalf("the edit saved Hermes on %q with %s; want the new file and the chosen %s", binding.Targets, binding.Model, chosenModel)
	}
	for _, path := range []string{target, target + adapter.StateSuffix, moved} {
		if data, err := os.ReadFile(path); err == nil && (path != target || bytes.Contains(data, []byte("aigw-"))) {
			t.Errorf("after the edit %s holds AIGW's part:\n%s", path, data)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if next := edited.NextAction + edited.NextDetail; !strings.Contains(next, "ucloud") {
		t.Errorf("the edit's next step is %q; Hermes waits for the Token of ucloud", next)
	}
}

// An edit of another setting of such a client is saved, and its next step
// names the Token that the client waits for; the model chosen in Hermes is no
// outside edit. Holds the cell chosen/edits (the review's F5).
func TestAnEditOfAClientWithoutItsTokenNamesTheTokenAsItsNextStep(t *testing.T) {
	t.Parallel()
	for _, edit := range [][]string{
		{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"},
		{"client", "edit", "hermes", "--program", executableFixture(t, "hermes"), "--json"},
	} {
		app, output, _, _ := waitingHermes(t)
		var edited followUp
		if err := runDocument(t, app, output, &edited, edit...); err != nil {
			t.Fatalf("%v: %v\n%s", edit[:4], err, output)
		}
		if next := edited.NextAction + edited.NextDetail; strings.Contains(next, "outside edit") || !strings.Contains(next, "ucloud") {
			t.Errorf("%v: the next step is %q; Hermes waits for the Token of ucloud, and its file holds a model chosen in Hermes", edit[:4], next)
		}
	}
}

// A rollback to the model that Hermes's file shows is not refused for the
// model that was stored before, which the restored configuration does not
// hold: the choice is recorded, so nothing is left that names that model. The
// file is left, and no command returns to a model that is gone. Holds the
// cell chosen/restores (the review's F6, which expected a refusal with a
// step; without the stored selection put back there is none).
func TestARollbackToTheChosenModelLetsTheEarlierModelGo(t *testing.T) {
	t.Parallel()
	const chosenModel = "ucloud/claude-opus-5-5"
	gone := domain.NewModelRef("dmxapi", "zz-new")
	app, output, target, chosen := chosenWithoutTokenAfter(t, "ucloud", "aigw-ucloud-anthropic", "claude-opus-5-5",
		[]string{"use", "hermes", chosenModel, "--protocol", "anthropic"},
		[]string{"use", "hermes", gone.String(), "--protocol", "responses", "--unlisted"})
	var result rolledBack
	if err := runDocument(t, app, output, &result, "config", "rollback", "--last-change", "--json"); err != nil {
		t.Fatalf("the rollback: %v\n%s", err, output)
	}
	if result.restores(hermes.ID) || result.Followed[hermes.ID] != chosenModel || strings.Contains(output.String(), "returns_with") {
		t.Fatalf("the rollback restores %+v and follows %v; want Hermes's choice followed, with no command that returns to a model that is gone\n%s", result.Selections, result.Followed, output)
	}
	if got := followedIn(t, app, output, "status", "--json"); len(got) != 0 {
		t.Fatalf("after the rollback, status still follows %+v: the choice was not recorded", got)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the rollback wrote the file of a client whose provider has no Token")
	}
	stored := mustLoad(t, app)
	if _, held := stored.Models[gone]; held || stored.Clients[hermes.ID].Model.String() != chosenModel {
		t.Fatalf("the rollback saved %s for Hermes and holds %s: %t", stored.Clients[hermes.ID].Model, gone, held)
	}
}

// After the Token that a chosen model waited for is stored, the next step is
// one that the working configuration gives, which follows the choice. Holds
// the cell chosen/other for a command that stores a Token (the review's F7).
func TestStoringTheTokenOfAChosenModelNamesAStep(t *testing.T) {
	t.Parallel()
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	for _, command := range [][]string{
		{"provider", "rotate", "ucloud", "--token-stdin", "--json"},
		{"setup", "--from", manifest, "--provider", "ucloud", "--token-stdin", "--json"},
	} {
		app, output, _, _ := waitingHermes(t)
		app.In = strings.NewReader("fixture-token\n")
		var stored followUp
		if err := runDocument(t, app, output, &stored, command...); err != nil {
			t.Fatalf("%v: %v\n%s", command[:2], err, output)
		}
		if stored.NextAction == "aigw use --help" || stored.NextAction+stored.NextDetail == "" {
			t.Errorf("%v: the next step after storing the Token that Hermes's chosen model waited for is %q", command[:2], stored.NextAction)
		}
	}
}

// Synchronization and its preview name for a client the model that status
// names: there is one configuration. Holds the cell chosen/reports (the
// review's F8).
func TestSyncNamesTheModelThatStatusNames(t *testing.T) {
	t.Parallel()
	app, output, _, _ := waitingHermes(t)
	var status struct {
		Clients map[string]struct {
			Model, State string
		} `json:"clients"`
	}
	if err := runDocument(t, app, output, &status, "status", "--json"); err != nil {
		t.Fatalf("status: %v\n%s", err, output)
	}
	for _, command := range [][]string{{"sync", "--dry-run", "--json"}, {"sync", "--json"}} {
		var synced struct {
			Selections map[string]string `json:"selections"`
		}
		if err := runDocument(t, app, output, &synced, command...); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		if synced.Selections[hermes.ID] != status.Clients[hermes.ID].Model {
			t.Errorf("%v names %s for Hermes; status names %s, %s", command, synced.Selections[hermes.ID], status.Clients[hermes.ID].Model, status.Clients[hermes.ID].State)
		}
	}
}
