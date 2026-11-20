package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// commandsAfterAChoice are commands of every group, none of which selects a
// model for the client or rolls the configuration back. passes says that the
// command succeeds in the fixture without a choice, so that its row compares
// two successes and not two refusals of its arguments.
type commandAfterAChoice struct {
	args   []string
	passes bool
}

var codexCommandsAfterAChoice = []commandAfterAChoice{
	{nil, true},
	{[]string{"status"}, true},
	{[]string{"status", "--json"}, true},
	{[]string{"check", "--depth", "local"}, true},
	{[]string{"check", "codex", "--depth", "local", "--json"}, true},
	{[]string{"check", "codex", "--depth", "client"}, true},
	{[]string{"check", "--depth", "client"}, true},
	{[]string{"sync"}, true},
	{[]string{"sync", "--dry-run", "--json"}, true},
	{[]string{"provider", "list"}, true},
	{[]string{"provider", "show", "gateway"}, true},
	{[]string{"provider", "edit", "spare", "--label", "Other"}, true},
	{[]string{"provider", "edit", "gateway", "--label", "Other", "--dry-run"}, true},
	{[]string{"provider", "edit", "gateway", "--label", "Other"}, true},
	{[]string{"model", "list"}, true},
	{[]string{"model", "show", "gateway/gpt-terra"}, true},
	{[]string{"model", "edit", "spare/gpt-spare", "--label", "Reserve"}, true},
	{[]string{"model", "remove", "spare/gpt-spare"}, true},
	{[]string{"client", "edit", "codex", "--request-retries", "2", "--dry-run"}, true},
	{[]string{"client", "edit", "codex", "--request-retries", "2"}, true},
	{[]string{"client", "disable", "codex", "--dry-run"}, true},
	{[]string{"client", "disable", "codex"}, true},
	{[]string{"config", "path"}, true},
	{[]string{"self", "uninstall", "--dry-run"}, true},
	{[]string{"version"}, true},
}

var hermesCommandsAfterAChoice = []commandAfterAChoice{
	{[]string{"status"}, true},
	{[]string{"check", "hermes", "--depth", "local"}, true},
	{[]string{"check", "hermes", "--depth", "client"}, true},
	{[]string{"sync"}, true},
	{[]string{"sync", "--dry-run", "--json"}, true},
	{[]string{"provider", "edit", "ucloud", "--label", "Other"}, true},
	{[]string{"model", "list"}, true},
	{[]string{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792"}, true},
	{[]string{"client", "disable", "hermes", "--dry-run"}, true},
	{[]string{"client", "disable", "hermes"}, true},
	{[]string{"self", "uninstall", "--dry-run"}, true},
}

// No command fails or refuses because a model was chosen in a client: each
// command of the table ends after a choice as it does without one, on a
// fresh home each time.
func TestEveryCommandEndsAfterAChoiceAsWithoutOne(t *testing.T) {
	end := func(app *cli.App, output *bytes.Buffer, args []string) string {
		output.Reset()
		if err := cli.Execute(app, args); err != nil {
			return "failed: " + err.Error()
		}
		// A check that passed and could not keep its snapshot did not end well.
		if strings.Contains(output.String(), "The verified snapshot was not written") {
			return "passed without its verified snapshot"
		}
		return "passed"
	}
	compare := func(t *testing.T, row commandAfterAChoice, without, after string) {
		t.Helper()
		if row.passes && without != "passed" {
			t.Errorf("%v does not pass without a choice, so its row compares nothing: %s", row.args, without)
		}
		if after != without {
			t.Errorf("%v ends differently after a choice:\n  without: %s\n  after:   %s", row.args, without, after)
		}
	}
	for _, row := range codexCommandsAfterAChoice {
		app, output, _ := codexChoiceApp(t)
		without := end(app, output, row.args)
		app, output, target := codexChoiceApp(t)
		chooseInCodex(t, target, "gpt-sol", "gpt-terra")
		compare(t, row, without, end(app, output, row.args))
	}
	for _, row := range hermesCommandsAfterAChoice {
		app, output, _, _ := shippedHermesApp(t)
		without := end(app, output, row.args)
		app, output, _, target := shippedHermesApp(t)
		hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
		compare(t, row, without, end(app, output, row.args))
	}
}

// The check that runs the client works from the chosen model: it runs Codex
// on the file that shows the choice, names the followed selection, and a
// check of every client keeps its verified snapshot, which holds the chosen
// model.
func TestTheClientDepthCheckWorksFromAChoice(t *testing.T) {
	app, output, targets := codexChoiceAppWith(t, "gpt-sol", 2)
	slices.Sort(targets)
	// The second file chose; the first still holds what AIGW last wrote.
	chooseInCodex(t, targets[1], "gpt-sol", "gpt-terra")
	output.Reset()
	if err := cli.Execute(app, []string{"check", "--depth", "client", "--json"}); err != nil {
		t.Fatalf("the check after a choice: %v\n%s", err, output)
	}
	var result struct {
		SnapshotWritten *bool      `json:"snapshot_written"`
		Followed        []followed `json:"followed_selections"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.SnapshotWritten == nil || !*result.SnapshotWritten || !slices.Equal(result.Followed, codexChoice) {
		t.Fatalf("the check wrote its snapshot %v and follows %+v: %v\n%s", result.SnapshotWritten, result.Followed, err, output)
	}
	verified, err := app.Config.LoadVerifiedCheckpoint()
	if err != nil || verified.Config.Clients[codex.ID].Model != domain.NewModelRef("gateway", "gpt-terra") {
		t.Fatalf("the verified snapshot does not hold the chosen model: %v, %#v", err, verified.Config.Clients[codex.ID])
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-sol") {
		t.Fatalf("a check saved the choice: %#v", binding)
	}
}

// A choice whose provider has no Token is not saved: it stays in the
// client's file, status shows it as waiting, and a command that saves leaves
// the stored selection and the file as they are. So the earlier model, chosen
// again because the first could not run, is what synchronization keeps once
// the Token is stored.
func TestAChoiceIsNotSavedWhileItsProviderWaitsForAToken(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	output.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil ||
		!strings.Contains(output.String(), "ucloud/claude-opus-5-5 · Deferred · The selected provider has no Token") || !strings.Contains(output.String(), "Followed selection") {
		t.Fatalf("status: %v\n%s", err, output)
	}
	for _, command := range [][]string{
		{"provider", "edit", "dmxapi", "--label", "Other", "--dry-run", "--json"},
		{"provider", "edit", "dmxapi", "--label", "Other", "--json"},
		{"sync", "--json"},
	} {
		if got := followedIn(t, app, output, command...); len(got) != 0 {
			t.Fatalf("%v names %+v as recorded while the client waits for its Token", command, got)
		}
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") {
		t.Fatalf("a choice was saved without AIGW's record of the client's file: %#v", binding)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("a command wrote the file of a client that waits for its Token")
	}

	// The pick could not run, so the person goes back to the earlier model.
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 0 {
		t.Fatalf("synchronization follows %+v", got)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("synchronization wrote over the model chosen last: %s %s", provider, model)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// A command that changes the settings of a client whose chosen model waits
// for its Token saves its own change and not the choice: the client's file
// keeps the choice, the stored selection stays the earlier model, and once
// the person goes back to that model and the Token is stored, synchronization
// keeps it.
func TestAnEditOfAClientThatWaitsForATokenSavesTheEditAlone(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	edit := []string{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"}
	if got := followedIn(t, app, output, edit...); len(got) != 0 {
		t.Fatalf("%v names %+v as recorded while the client waits for its Token", edit, got)
	}
	binding := selection(t, app, hermes.ID)
	if binding.Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") || binding.Protocol != domain.ProtocolResponses {
		t.Fatalf("an edit saved the choice of a client that waits for its Token: %#v", binding)
	}
	if binding.ForwardingEndpoint != "http://127.0.0.1:8792" {
		t.Fatalf("the edit's own change was not saved: %#v", binding)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the edit wrote the file of a client that waits for its Token")
	}

	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 0 {
		t.Fatalf("synchronization follows %+v", got)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("synchronization wrote over the model chosen last: %s %s", provider, model)
	}
}

// While a client's chosen model waits for its Token, an edit of the client's
// forwarding acts on the stored selection, which is what it saves: `--direct`
// ends the stored forwarding and `--via` forwards the stored selection, and
// the client's file keeps the choice. Once the Token is stored, the choice is
// recorded, and its upstream ends that forwarding as any choice's does, which
// the recording says.
func TestAForwardingEditOfAClientThatWaitsForATokenActsOnTheStoredSelection(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"client", "edit", "hermes", "--via", "http://127.0.0.1:9000"}); err != nil {
		t.Fatalf("forward Hermes: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	if got := followedIn(t, app, output, "client", "edit", "hermes", "--direct", "--json"); len(got) != 0 {
		t.Fatalf("--direct names %+v as recorded while the client waits for its Token", got)
	}
	if binding := selection(t, app, hermes.ID); binding.ForwardingEndpoint != "" || binding.Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") {
		t.Fatalf("--direct did not reach what is saved: %#v", binding)
	}
	if got := followedIn(t, app, output, "client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"); len(got) != 0 {
		t.Fatalf("--via names %+v as recorded while the client waits for its Token", got)
	}
	var edited struct {
		Model      string `json:"model"`
		Forwarding string `json:"forwarding"`
	}
	if err := json.Unmarshal(output.Bytes(), &edited); err != nil || edited.Model != "dmxapi/gpt-6.2-sol" || edited.Forwarding != "http://127.0.0.1:8792" {
		t.Fatalf("the edit's result does not name what it saved: %+v, %v\n%s", edited, err, output)
	}
	if binding := selection(t, app, hermes.ID); binding.ForwardingEndpoint != "http://127.0.0.1:8792" || binding.Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") {
		t.Fatalf("--via did not reach what is saved: %#v", binding)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("an edit wrote the file of a client that waits for its Token")
	}

	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 1 || !got[0].ForwardingCleared {
		t.Fatalf("synchronization after the Token follows %+v; want the choice, its forwarding ended", got)
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") || binding.ForwardingEndpoint != "" {
		t.Fatalf("the choice was not recorded with its own upstream: %#v", binding)
	}
}

// The command that stores a waiting client's Token itself, `aigw use` in a
// terminal, compares what is stored with the configuration that it read, in
// which the choice waited: the Token that it stores is not a change that
// refuses it, and it records the model that it selects.
func TestAUseThatStoresTheTokenOfAWaitingChoiceIsNotRefused(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	app.Interactive, app.Prompt = true, &scriptedPrompt{secrets: []string{"fixture-token"}}
	output.Reset()
	if err := cli.Execute(app, []string{"use", "hermes", "ucloud/claude-sonnet-5-5", "--protocol", "anthropic"}); err != nil {
		t.Fatalf("use that stores the Token: %v\n%s", err, output)
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("ucloud", "claude-sonnet-5-5") {
		t.Fatalf("use did not record its selection: %#v", binding)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-ucloud-anthropic" || model != "claude-sonnet-5-5" {
		t.Fatalf("use did not write Hermes's file: %s %s", provider, model)
	}
}

// A command about something else is not stopped by another file of the
// client whose model was chosen: where that file is missing, the choice
// stays in the file that shows it and the command succeeds as it does
// without the choice.
func TestACommandAboutSomethingElseIsNotStoppedByTheClientsOtherFile(t *testing.T) {
	app, output, targets := codexChoiceAppWith(t, "gpt-sol", 2)
	chooseInCodex(t, targets[0], "gpt-sol", "gpt-terra")
	if err := os.Remove(targets[1]); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json"); len(got) != 0 {
		t.Fatalf("the command names %+v as recorded", got)
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-sol") {
		t.Fatalf("a choice was saved without AIGW's record of the client's files: %#v", binding)
	}
	if written := string(readFile(t, targets[0])); !strings.Contains(written, `model = "gpt-terra"`+"\n") {
		t.Fatalf("the command wrote the file that shows the choice:\n%s", written)
	}
}

// A command that fails saved nothing, so it names no followed selection, in
// text or in JSON: here a selection for another client, whose model the
// provider's catalog does not list, after the command's own preview had
// planned to record Hermes' choice.
func TestACommandThatFailsNamesNoFollowedSelection(t *testing.T) {
	for _, mode := range [][]string{nil, {"--json"}} {
		app, output, _, target := shippedHermesApp(t)
		hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
		command := append([]string{"use", "claude", "dmxapi/claude-not-in-catalog"}, mode...)
		output.Reset()
		if err := cli.Execute(app, command); err == nil {
			t.Fatalf("%v succeeded:\n%s", command, output)
		}
		if strings.Contains(output.String(), "Followed selection") || strings.Contains(output.String(), "followed_selections") {
			t.Fatalf("%v failed and names a followed selection:\n%s", command, output)
		}
		if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") {
			t.Fatalf("the failed command saved Hermes' choice: %#v", binding)
		}
	}
}
