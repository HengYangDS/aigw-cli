package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
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
	// Installed Windows Hermes ignores the verification-only launcher opt-out, so
	// AIGW refuses this check there, with a choice and without one.
	{[]string{"check", "hermes", "--depth", "client"}, runtime.GOOS != "windows"},
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

// A choice whose provider has no Token is recorded as any other: status
// shows it as the client's selection, waiting for the Token, and a command
// that saves, or its preview, names it; the stored selection and AIGW's
// record of the file move to it, and the file is left. Holds the cells
// chosen/reports and chosen/other.
func TestAChoiceIsRecordedWhileItsProviderWaitsForAToken(t *testing.T) {
	app, output, target, chosen := waitingHermes(t)
	record := readFile(t, target+".aigw-state.json")
	choice := waitingHermesChoice()
	output.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil ||
		!strings.Contains(output.String(), "ucloud/claude-opus-5-5 · Deferred · The selected provider has no Token") || !strings.Contains(output.String(), "Followed selection") {
		t.Fatalf("status: %v\n%s", err, output)
	}
	edit := []string{"provider", "edit", "dmxapi", "--label", "Other", "--json"}
	if got := followedIn(t, app, output, append(edit, "--dry-run")...); !slices.Equal(got, choice) {
		t.Fatalf("the preview follows %+v", got)
	}
	if !bytes.Equal(readFile(t, target+".aigw-state.json"), record) || selection(t, app, hermes.ID).Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") {
		t.Fatal("the preview saved the choice or wrote AIGW's record of Hermes's file")
	}
	output.Reset()
	if err := cli.Execute(app, edit); err != nil {
		t.Fatalf("%v: %v\n%s", edit, err, output)
	}
	type editFile struct {
		Client string `json:"client"`
		Action string `json:"action"`
	}
	var result struct {
		Followed []followed `json:"followed_selections"`
		Files    []editFile `json:"client_files"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || !slices.Equal(result.Followed, choice) {
		t.Fatalf("the run follows %+v: %v\n%s", result.Followed, err, output)
	}
	// The result names what it wrote of Hermes: AIGW's record, not the file.
	for _, file := range result.Files {
		if file.Client == hermes.ID && file.Action != "record" {
			t.Fatalf("the run names %q for the file of a client whose provider has no Token", file.Action)
		}
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") || binding.Protocol != domain.ProtocolAnthropic {
		t.Fatalf("the choice was not saved: %#v", binding)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("a command wrote the file of a client that waits for its Token")
	}
	if bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
		t.Fatal("the choice was saved without AIGW's record of the client's file")
	}
	// Recorded once: the next command follows nothing, and status still
	// names the model as waiting for its Token.
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 0 {
		t.Fatalf("a second command follows %+v", got)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil ||
		!strings.Contains(output.String(), "ucloud/claude-opus-5-5 · Deferred · The selected provider has no Token") || strings.Contains(output.String(), "Followed selection") {
		t.Fatalf("status after the record: %v\n%s", err, output)
	}
}

// waitingHermesChoice is what a command that records the choice of
// waitingHermes follows, as its document names it.
func waitingHermesChoice() []followed {
	return []followed{{
		Client: "hermes", Model: "ucloud/claude-opus-5-5", Protocol: "anthropic", Earlier: "dmxapi/gpt-6.2-sol", EarlierProtocol: "responses",
		ReturnsWith: "aigw use hermes dmxapi/gpt-6.2-sol --protocol responses",
	}}
}

// Once the choice is recorded, the model that AIGW held before, chosen in
// Hermes again because the first could not run, is a choice like any other:
// the Token arrives with it, and synchronization writes the model chosen
// last. Holds the cell chosen/other.
func TestTheModelChosenAgainAfterARecordIsKept(t *testing.T) {
	app, output, target, _ := waitingHermes(t)
	if got := followedIn(t, app, output, "sync", "--json"); !slices.Equal(got, waitingHermesChoice()) {
		t.Fatalf("synchronization follows %+v; want Hermes's choice recorded", got)
	}
	// The pick could not run, so the person goes back to the earlier model.
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 1 || got[0].Model != "dmxapi/gpt-6.2-sol" || got[0].Earlier != "ucloud/claude-opus-5-5" {
		t.Fatalf("synchronization follows %+v; want the earlier model, chosen again", got)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("synchronization wrote %s %s over the model chosen last", provider, model)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// A command that changes the settings of a client whose chosen model waits
// for its Token records the choice with its own change, as it does with the
// Token: the stored selection is the chosen model with the edit, AIGW's
// record of the file moves with it, and the file is left. The synchronization
// after the Token is stored writes the file. Holds the cell chosen/edits.
func TestAnEditOfAClientThatWaitsForATokenRecordsTheChoiceWithTheEdit(t *testing.T) {
	app, output, target, chosen := waitingHermes(t)
	edit := []string{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"}
	if got := followedIn(t, app, output, edit...); len(got) != 1 || got[0].Client != hermes.ID || got[0].Model != "ucloud/claude-opus-5-5" {
		t.Fatalf("%v follows %+v; want Hermes's choice", edit, got)
	}
	var edited struct {
		Model      string `json:"model"`
		Forwarding string `json:"forwarding"`
	}
	if err := json.Unmarshal(output.Bytes(), &edited); err != nil || edited.Model != "ucloud/claude-opus-5-5" || edited.Forwarding != "http://127.0.0.1:8792" {
		t.Fatalf("the edit's result does not name what it saved: %+v, %v\n%s", edited, err, output)
	}
	binding := selection(t, app, hermes.ID)
	if binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") || binding.Protocol != domain.ProtocolAnthropic || binding.ForwardingEndpoint != "http://127.0.0.1:8792" {
		t.Fatalf("the edit did not save the choice with its own change: %#v", binding)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the edit wrote the file of a client that waits for its Token")
	}
	if got := followedIn(t, app, output, "status", "--json"); len(got) != 0 {
		t.Fatalf("AIGW's record of the file stayed behind: status still follows %+v", got)
	}

	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 0 {
		t.Fatalf("synchronization after the Token follows %+v; the choice is recorded", got)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-ucloud-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("synchronization after the Token wrote %s %s", provider, model)
	}
	if written := string(readFile(t, target)); !strings.Contains(written, "http://127.0.0.1:8792") {
		t.Fatalf("synchronization after the Token did not write the edit's forwarding:\n%s", written)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// editedClient is what `aigw client edit --json` says of the client.
type editedClient struct {
	Changed    bool   `json:"changed"`
	Model      string `json:"model"`
	Forwarding string `json:"forwarding"`
}

// endsTheStoredForwarding runs `aigw client edit hermes --direct` after the
// choice of ucloud/claude-opus-5-5 ended a stored forwarding, and checks that
// the edit is judged against what is stored: it follows the choice and says
// that the choice ended the forwarding, saves the chosen model without a
// forwarding, and finds nothing to change when it runs again.
func endsTheStoredForwarding(t *testing.T, app *cli.App, output *bytes.Buffer) {
	t.Helper()
	var edited editedClient
	got := followedIn(t, app, output, "client", "edit", "hermes", "--direct", "--json")
	if len(got) != 1 || got[0].Model != "ucloud/claude-opus-5-5" || !got[0].ForwardingCleared {
		t.Fatalf("--direct follows %+v; want the choice, which ended the earlier forwarding", got)
	}
	if err := json.Unmarshal(output.Bytes(), &edited); err != nil || !edited.Changed || edited.Model != "ucloud/claude-opus-5-5" || edited.Forwarding != "" {
		t.Fatalf("--direct reports %+v, %v; want the choice saved without a forwarding\n%s", edited, err, output)
	}
	if binding := selection(t, app, hermes.ID); binding.ForwardingEndpoint != "" || binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") {
		t.Fatalf("--direct did not reach what is saved: %#v", binding)
	}
	if again := followedIn(t, app, output, "client", "edit", "hermes", "--direct", "--json"); len(again) != 0 {
		t.Fatalf("a second --direct follows %+v; the choice is saved", again)
	}
	if err := json.Unmarshal(output.Bytes(), &edited); err != nil || edited.Changed {
		t.Fatalf("a second --direct reports %+v, %v; nothing is left to change\n%s", edited, err, output)
	}
}

// An edit of the forwarding of such a client acts on the model chosen in it,
// which is the client's selection, and is judged against what is stored: the
// stored selection still forwards, though the choice of another upstream
// ended that forwarding in the configuration that the command works from. So
// `--direct` saves the choice without the forwarding and says that the choice
// ended it, and a `--via` after it forwards the chosen model. Before the
// tenth review `--direct` found nothing to change here and saved nothing, so
// the stored model, picked again, forwarded again. Holds the cell
// chosen/edits.
func TestAForwardingEditOfAClientThatWaitsForATokenActsOnTheChosenModel(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"client", "edit", "hermes", "--via", "http://127.0.0.1:9000"}); err != nil {
		t.Fatalf("forward Hermes: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	endsTheStoredForwarding(t, app, output)
	var edited editedClient
	if err := runDocument(t, app, output, &edited, "client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"); err != nil {
		t.Fatalf("--via: %v\n%s", err, output)
	}
	if !edited.Changed || edited.Model != "ucloud/claude-opus-5-5" || edited.Forwarding != "http://127.0.0.1:8792" {
		t.Fatalf("the edit's result does not name what it saved: %+v\n%s", edited, output)
	}
	if binding := selection(t, app, hermes.ID); binding.ForwardingEndpoint != "http://127.0.0.1:8792" || binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") {
		t.Fatalf("--via did not reach what is saved: %#v", binding)
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("an edit wrote the file of a client that waits for its Token")
	}

	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 0 {
		t.Fatalf("synchronization after the Token follows %+v; the choice is recorded", got)
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") || binding.ForwardingEndpoint != "http://127.0.0.1:8792" {
		t.Fatalf("synchronization after the Token changed what the edit saved: %#v", binding)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-ucloud-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("synchronization after the Token wrote %s %s", provider, model)
	}
}

// The command that stores the Token itself, `aigw use` in a terminal for the
// client whose chosen model waited for it, works from a configuration that
// does not depend on any Token: the Token that it stores is not a change
// that refuses it, and it records and writes the model that it selects.
// Holds the cell chosen/selects.
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
