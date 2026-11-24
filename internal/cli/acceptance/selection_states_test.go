package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// The tests of this file cover the row "chosen" of the lane's state table
// where the provider of the model that Hermes's file selects has no Token
// that AIGW can use. The model that the client uses is the file's for every
// command, a saving command records it with AIGW's record of the file, and
// the file is left until the Token is stored.

// waitingHermes sets the state up: the stored selection is
// dmxapi/gpt-6.2-sol, Hermes's file selects ucloud/claude-opus-5-5, and
// ucloud has no Token.
func waitingHermes(t *testing.T) (*cli.App, *bytes.Buffer, string, []byte) {
	t.Helper()
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	return app, output, target, chosen
}

// run executes one command with --json and decodes its document.
func runDocument(t *testing.T, app *cli.App, output *bytes.Buffer, document any, args ...string) error {
	t.Helper()
	output.Reset()
	err := cli.Execute(app, args)
	if err == nil && document != nil {
		if decode := json.Unmarshal(output.Bytes(), document); decode != nil {
			t.Fatalf("%v: %v\n%s", args, decode, output)
		}
	}
	return err
}

// Every reader names the model that the client uses, the file's: status, the
// client list, and the lists and views of models and providers read one
// configuration, and none names the stored selection as Hermes's. A reader
// saves nothing. Holds the cell chosen/reports.
func TestEveryReaderNamesTheModelOfAClientThatWaitsForItsToken(t *testing.T) {
	t.Parallel()
	app, output, target, chosen := waitingHermes(t)
	const uses, stored = "ucloud/claude-opus-5-5", "dmxapi/gpt-6.2-sol"
	var clients struct {
		Clients []struct {
			ID, Model string
		} `json:"clients"`
	}
	if err := runDocument(t, app, output, &clients, "client", "list", "--json"); err != nil {
		t.Fatal(err)
	}
	for _, client := range clients.Clients {
		if client.ID == hermes.ID && client.Model != uses {
			t.Fatalf("client list names %s for Hermes; its file selects %s", client.Model, uses)
		}
	}
	type selected struct {
		ID         string   `json:"id"`
		SelectedBy []string `json:"selected_by"`
	}
	var models struct {
		Models []selected `json:"models"`
	}
	if err := runDocument(t, app, output, &models, "model", "list", "--json"); err != nil {
		t.Fatal(err)
	}
	for _, model := range models.Models {
		if by := slices.Contains(model.SelectedBy, hermes.ID); by != (model.ID == uses) {
			t.Fatalf("model list: %s selected by Hermes = %t; Hermes's file selects %s", model.ID, by, uses)
		}
	}
	for _, view := range []struct {
		args []string
		want bool
	}{
		{[]string{"model", "show", uses, "--json"}, true},
		{[]string{"model", "show", stored, "--json"}, false},
		{[]string{"provider", "show", "ucloud", "--json"}, true},
		{[]string{"provider", "show", "dmxapi", "--json"}, false},
	} {
		var shown selected
		if err := runDocument(t, app, output, &shown, view.args...); err != nil {
			t.Fatalf("%v: %v", view.args, err)
		}
		if by := slices.Contains(shown.SelectedBy, hermes.ID); by != view.want {
			t.Fatalf("%v: selected by Hermes = %t, want %t; Hermes's file selects %s", view.args, by, view.want, uses)
		}
	}
	output.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil || !strings.Contains(output.String(), uses) {
		t.Fatalf("status does not name the chosen model: %v\n%s", err, output)
	}
	// A reader records nothing: the stored selection and the file stay.
	if got := mustLoad(t, app).Clients[hermes.ID].Model.String(); got != stored {
		t.Fatalf("a reader saved %s as Hermes's selection", got)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("a reader wrote Hermes's file")
	}
}

// The model that a client's file selects is the client's selection for the
// commands' own guards: its removal, or its provider's, is refused with the
// client and the model named, and nothing changes. The model that was stored
// before is free to go: removing it records the choice in the same
// transaction, with no command that returns to a model that is gone, and the
// file is left. Holds the cell chosen/removes.
func TestTheChosenModelIsNotTakenAwayAndTheEarlierOneIsFreeToGo(t *testing.T) {
	t.Parallel()
	const chosenModel, storedModel = "ucloud/claude-opus-5-5", "dmxapi/gpt-6.2-sol"
	for _, command := range [][]string{
		{"model", "remove", chosenModel},
		{"model", "remove", chosenModel, "--dry-run"},
		{"provider", "remove", "ucloud"},
		{"provider", "remove", "ucloud", "--dry-run"},
	} {
		app, output, target, chosen := waitingHermes(t)
		before := readFile(t, app.Config.Path())
		output.Reset()
		err := cli.Execute(app, command)
		if err == nil || !strings.Contains(err.Error(), "Hermes") || !strings.Contains(err.Error(), chosenModel) || !strings.Contains(err.Error(), "cannot be removed") {
			t.Fatalf("%v = %v; want the refusal that Hermes uses %s\n%s", command, err, chosenModel, output)
		}
		if !bytes.Equal(readFile(t, app.Config.Path()), before) {
			t.Fatalf("%v changed the stored configuration", command)
		}
		if !bytes.Equal(readFile(t, target), chosen) {
			t.Fatalf("%v wrote Hermes's file", command)
		}
	}
	app, output, target, chosen := waitingHermes(t)
	got := followedIn(t, app, output, "model", "remove", storedModel, "--json")
	if len(got) != 1 || got[0].Client != hermes.ID || got[0].Model != chosenModel || got[0].Earlier != storedModel || got[0].ReturnsWith != "" {
		t.Fatalf("removing the earlier model follows %+v; want Hermes's choice, with no command back to a model that is gone", got)
	}
	stored := mustLoad(t, app)
	if _, held := stored.Models[domain.NewModelRef("dmxapi", "gpt-6.2-sol")]; held || stored.Clients[hermes.ID].Model.String() != chosenModel {
		t.Fatalf("after the removal the configuration holds the earlier model (%t) and selects %s for Hermes", held, stored.Clients[hermes.ID].Model)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the removal wrote the file of a client whose provider has no Token")
	}
}

// A withdrawal removes AIGW's part of the client's file and its record,
// whatever the file selects and whether or not its provider has a Token.
// Holds the cell chosen/withdraws.
func TestAWithdrawalOfAClientThatWaitsForItsTokenRemovesAIGWsPart(t *testing.T) {
	t.Parallel()
	app, output, target, _ := waitingHermes(t)
	if err := runDocument(t, app, output, nil, "client", "disable", "hermes", "--json"); err != nil {
		t.Fatalf("client disable: %v\n%s", err, output)
	}
	if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("AIGW's record of Hermes's file remains after the withdrawal: %v", err)
	}
	if left, err := os.ReadFile(target); err == nil && bytes.Contains(left, []byte("aigw-")) {
		t.Fatalf("Hermes's file keeps AIGW's provider entries after the withdrawal:\n%s", left)
	}
}

// An uninstall plans the same withdrawal: its preview names Hermes among the
// clients whose files it changes, and writes nothing. Holds the cell
// chosen/withdraws.
func TestAnUninstallPlansTheFileOfAClientThatWaitsForItsToken(t *testing.T) {
	t.Parallel()
	app, output, target, chosen := waitingHermes(t)
	var preview struct {
		Files []struct {
			Client, Action string
		} `json:"client_files"`
	}
	if err := runDocument(t, app, output, &preview, "self", "uninstall", "--dry-run", "--json"); err != nil {
		t.Fatalf("self uninstall --dry-run: %v\n%s", err, output)
	}
	planned := false
	for _, file := range preview.Files {
		planned = planned || file.Client == hermes.ID
	}
	if !planned {
		t.Fatalf("the uninstall's preview does not plan Hermes's file:\n%s", output)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the uninstall's preview wrote Hermes's file")
	}
}

// A rollback to a configuration that selects the model that Hermes's file
// already shows records that choice: the stored selection and AIGW's record
// of the file move to it, the result names it as followed, and the file is
// left while its provider has no Token. So the earlier model, chosen in
// Hermes again before the Token arrives, is a choice that synchronization
// keeps. Holds the cell chosen/restores.
func TestARollbackToTheChosenModelRecordsItAndKeepsTheNextChoice(t *testing.T) {
	t.Parallel()
	const chosenModel = "ucloud/claude-opus-5-5"
	app, output, _, target := shippedHermesAppWith(t, true)
	for _, args := range [][]string{
		{"use", "hermes", chosenModel, "--protocol", "anthropic"},
		{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses"},
	} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	if got := followedIn(t, app, output, "config", "rollback", "--last-change", "--json"); len(got) != 1 || got[0].Client != hermes.ID || got[0].Model != chosenModel {
		t.Fatalf("the rollback follows %+v; want Hermes's choice of %s", got, chosenModel)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the rollback wrote the file of a client that waits for its Token")
	}
	if stored := mustLoad(t, app).Clients[hermes.ID].Model.String(); stored != chosenModel {
		t.Fatalf("the rollback saved %s as Hermes's selection", stored)
	}
	if got := followedIn(t, app, output, "status", "--json"); len(got) != 0 {
		t.Fatalf("AIGW's record of the file stayed behind: status still follows %+v", got)
	}
	// The person goes back to the earlier model before the Token arrives.
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

func mustLoad(t *testing.T, app *cli.App) domain.Config {
	t.Helper()
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
