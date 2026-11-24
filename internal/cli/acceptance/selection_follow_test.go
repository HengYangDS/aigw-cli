package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// codexChoiceApp projects gateway/gpt-sol for Codex. The gateway also has
// gpt-terra, which Codex can choose, and a second provider serves no client.
func codexChoiceApp(t *testing.T) (*cli.App, *bytes.Buffer, string) {
	t.Helper()
	app, output, targets := codexChoiceAppWith(t, "gpt-sol", 1)
	return app, output, targets[0]
}

// codexChoiceAppWith projects the gateway's model selected for Codex into
// count Codex files.
func codexChoiceAppWith(t *testing.T, selected string, count int) (*cli.App, *bytes.Buffer, []string) {
	t.Helper()
	app, output, secretStore, _, _ := testApp(t, "")
	targets := make([]string, 0, count)
	for range count {
		target := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
		targets = append(targets, target)
	}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Providers["spare"] = domain.Provider{Label: "Spare", Endpoints: domain.Endpoints{Responses: "https://spare.test/v1"}}
	// The selected model's ID may need quoting in a shell; its family never does.
	sol := qualifiedModel("Sol", "gateway", selected, domain.ProtocolResponses)
	sol.Family = "gpt-sol"
	cfg.Models[sol.Ref()] = sol
	cfg.Models[domain.NewModelRef("gateway", "gpt-terra")] = qualifiedModel("Terra", "gateway", "gpt-terra", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("spare", "gpt-spare")] = qualifiedModel("Spare", "spare", "gpt-spare", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", selected), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), targets)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"gateway", "spare"} {
		if err := secretStore.Set(provider, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project the Codex selection: %v\n%s", err, output)
	}
	return app, output, targets
}

// chooseInCodex rewrites Codex's root model as Codex's own picker does: the
// value alone, without AIGW's comment.
func chooseInCodex(t *testing.T, target, from, to string) string {
	t.Helper()
	chosen := strings.Replace(string(readFile(t, target)), `model = "`+from+`" # managed by AIGW`, `model = "`+to+`"`, 1)
	if !strings.Contains(chosen, `model = "`+to+`"`) {
		t.Fatalf("Codex's file does not select %s as AIGW wrote it:\n%s", from, readFile(t, target))
	}
	writeFile(t, target, []byte(chosen), 0o600)
	return chosen
}

// followed is one entry of a document's followed_selections.
type followed struct {
	Client            string `json:"client"`
	Model             string `json:"model"`
	Protocol          string `json:"protocol"`
	Earlier           string `json:"earlier"`
	EarlierProtocol   string `json:"earlier_protocol"`
	ReturnsWith       string `json:"returns_with"`
	ForwardingCleared bool   `json:"forwarding_cleared"`
	RetriesCleared    bool   `json:"retries_cleared"`
}

// followedModels decodes a document's followed_selections as the model that
// each client chose.
type followedModels map[string]string

func (models *followedModels) UnmarshalJSON(data []byte) error {
	var entries []followed
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	*models = followedModels{}
	for _, entry := range entries {
		(*models)[entry.Client] = entry.Model
	}
	return nil
}

// followedIn runs a command with --json, which must succeed, and returns the
// followed selections of its document.
func followedIn(t *testing.T, app *cli.App, output *bytes.Buffer, command ...string) []followed {
	t.Helper()
	output.Reset()
	if err := cli.Execute(app, command); err != nil {
		t.Fatalf("%v: %v\n%s", command, err, output)
	}
	var result struct {
		Followed []followed `json:"followed_selections"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("%v: %v\n%s", command, err, output)
	}
	return result.Followed
}

var (
	codexChoice  = []followed{{Client: "codex", Model: "gateway/gpt-terra", Earlier: "gateway/gpt-sol", ReturnsWith: "aigw use codex gateway/gpt-sol"}}
	hermesChoice = []followed{{
		Client: "hermes", Model: "dmxapi/claude-opus-5-5", Protocol: "anthropic", Earlier: "dmxapi/gpt-6.2-sol", EarlierProtocol: "responses",
		ReturnsWith: "aigw use hermes dmxapi/gpt-6.2-sol --protocol responses",
	}}
)

// codexChoiceText is the human section for codexChoice: a sentence, then the
// returning command on a line of its own.
const codexChoiceText = "Followed selection\n" +
	"  Codex uses gateway/gpt-terra, chosen in Codex; AIGW held gateway/gpt-sol. To return to it:\n" +
	"  aigw use codex gateway/gpt-sol\n"

// saysCodexChoice requires human output to name codexChoice once, set off by
// a blank line, and ahead of the follow-up command when there is one.
func saysCodexChoice(t *testing.T, command []string, output string) {
	t.Helper()
	at := strings.Index(output, "\n\n"+codexChoiceText)
	if at < 0 || strings.Count(output, "Followed selection") != 1 {
		t.Fatalf("%v does not name the followed selection once, after a blank line:\n%s", command, output)
	}
	if next := strings.Index(output, "\nNext\n"); next >= 0 && next < at {
		t.Fatalf("%v names the followed selection after its follow-up command:\n%s", command, output)
	}
}

func selection(t *testing.T, app *cli.App, client string) domain.ClientBinding {
	t.Helper()
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Clients[client]
}

// A command other than sync that writes Codex's files, here an edit of its
// retry budget, works from the model that was chosen in Codex: it succeeds
// with its own edit, Codex keeps the model, the configuration names it, and
// the result says so with the command that returns.
func TestACommandThatWritesCodexFilesFollowsAModelChosenInCodex(t *testing.T) {
	t.Parallel()
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	directory := filepath.Dir(target)

	// A preview names the followed model and writes nothing.
	before := ownedFileSnapshot(t, directory, app.Config.Path())
	preview := []string{"client", "edit", "codex", "--request-retries", "2", "--dry-run"}
	if got := followedIn(t, app, output, append(preview, "--json")...); !slices.Equal(got, codexChoice) {
		t.Fatalf("preview follows %+v", got)
	}
	output.Reset()
	if err := cli.Execute(app, preview); err != nil {
		t.Fatalf("text preview: %v\n%s", err, output)
	}
	saysCodexChoice(t, preview, output.String())
	if after := ownedFileSnapshot(t, directory, app.Config.Path()); !equalSnapshots(after, before) {
		t.Fatal("the preview changed client files or the configuration")
	}

	if got := followedIn(t, app, output, "client", "edit", "codex", "--request-retries", "2", "--json"); !slices.Equal(got, codexChoice) {
		t.Fatalf("the run follows %+v", got)
	}
	written := string(readFile(t, target))
	for _, want := range []string{`model = "gpt-terra" # managed by AIGW`, "request_max_retries = 2"} {
		if !strings.Contains(written, want) {
			t.Fatalf("Codex's file lacks %q:\n%s", want, written)
		}
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-terra") || binding.RetryPolicy == nil {
		t.Fatalf("the configuration does not name the chosen model with the edit: %#v", binding)
	}
	// Once recorded, nothing is left to follow.
	if got := followedIn(t, app, output, "sync", "--json"); len(got) != 0 {
		t.Fatalf("a second command still follows %+v", got)
	}
	syncAndCheckPass(t, app, output, codex.ID)

	// The command that the document names returns to the earlier model.
	if err := cli.Execute(app, strings.Fields(codexChoice[0].ReturnsWith)[1:]); err != nil {
		t.Fatalf("%q: %v\n%s", codexChoice[0].ReturnsWith, err, output)
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-sol") {
		t.Fatalf("the returning command did not select the earlier model: %#v", binding)
	}
}

// An explicit selection is not followed away: it writes the named model over
// the one that was chosen in Codex, and neither its preview nor its result
// names a followed selection. Naming the chosen model itself records it, and
// says that the configuration changed.
func TestAnExplicitSelectionWinsOverAModelChosenInCodex(t *testing.T) {
	t.Parallel()
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	for _, command := range [][]string{
		{"use", "codex", "gateway/gpt-sol", "--dry-run", "--json"},
		{"use", "codex", "gateway/gpt-sol", "--json"},
	} {
		if got := followedIn(t, app, output, command...); len(got) != 0 {
			t.Fatalf("%v follows %+v", command, got)
		}
	}
	if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-sol" # managed by AIGW`) {
		t.Fatalf("the explicit selection was not written:\n%s", written)
	}
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	for _, command := range [][]string{
		{"use", "codex", "gateway/gpt-terra", "--dry-run", "--json"},
		{"use", "codex", "gateway/gpt-terra", "--json"},
	} {
		if got := followedIn(t, app, output, command...); len(got) != 0 {
			t.Fatalf("%v, naming the chosen model, follows %+v", command, got)
		}
		var result struct {
			Changed bool `json:"changed"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil || !result.Changed {
			t.Fatalf("%v saves the named model without saying that the configuration changes: %v\n%s", command, err, output)
		}
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-terra") {
		t.Fatalf("the named model was not recorded: %#v", binding)
	}
}

// Hermes: a command that writes Hermes' file works from the listed model that
// was chosen in Hermes, and keeps its own edit of Hermes' binding.
func TestACommandThatWritesHermesFilesFollowsAModelChosenInHermes(t *testing.T) {
	t.Parallel()
	app, output, _, target := shippedHermesApp(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if got := followedIn(t, app, output, "client", "edit", hermes.ID, "--via", "http://127.0.0.1:8792", "--json"); !slices.Equal(got, hermesChoice) {
		t.Fatalf("the run follows %+v\n%s", got, output)
	}
	binding := selection(t, app, hermes.ID)
	if binding.Model != domain.NewModelRef("dmxapi", "claude-opus-5-5") || binding.ForwardingEndpoint != "http://127.0.0.1:8792" {
		t.Fatalf("the saved binding lost the choice or the command's own edit: %#v", binding)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("Hermes lost its choice: %s %s", provider, model)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// A choice of another provider's model in Hermes ends the forwarding that
// belonged to the earlier upstream endpoint, and the result says so.
func TestAFollowedChoiceSaysThatForwardingEnded(t *testing.T) {
	t.Parallel()
	app, output, _, target := shippedHermesApp(t)
	if err := cli.Execute(app, []string{"client", "edit", hermes.ID, "--via", "http://127.0.0.1:8792"}); err != nil {
		t.Fatalf("forward Hermes: %v\n%s", err, output)
	}
	hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	want := []followed{{
		Client: "hermes", Model: "ucloud/claude-opus-5-5", Protocol: "anthropic", Earlier: "dmxapi/gpt-6.2-sol", EarlierProtocol: "responses",
		ReturnsWith: "aigw use hermes dmxapi/gpt-6.2-sol --protocol responses", ForwardingCleared: true,
	}}
	if got := followedIn(t, app, output, "status", "--json"); !slices.Equal(got, want) {
		t.Fatalf("status follows %+v", got)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, output)
	}
	for _, text := range []string{
		"Hermes uses ucloud/claude-opus-5-5 on Anthropic Messages, chosen in Hermes; AIGW held dmxapi/gpt-6.2-sol on OpenAI Responses.",
		"Its forwarding ended, because the client's upstream endpoint changed.",
		"  aigw use hermes dmxapi/gpt-6.2-sol --protocol responses\n",
	} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("sync lacks %q:\n%s", text, output)
		}
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("ucloud", "claude-opus-5-5") || binding.ForwardingEndpoint != "" {
		t.Fatalf("the saved binding: %#v", binding)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// Synchronization says what it followed in the same words, ahead of its
// follow-up command, and nothing once the choice is recorded.
func TestSyncSaysWhatItFollowedAndHowToReturn(t *testing.T) {
	t.Parallel()
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	for _, command := range [][]string{{"sync", "--dry-run"}, {"sync"}} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		saysCodexChoice(t, command, output.String())
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil || strings.Contains(output.String(), "Followed selection") {
		t.Fatalf("a second sync still follows: %v\n%s", err, output)
	}
}

// Disabling a client after a model was chosen in it withdraws what AIGW
// wrote, and so does the preview of an uninstall: the choice is no outside
// edit to undo.
func TestAClientIsWithdrawnAfterAModelWasChosenInIt(t *testing.T) {
	t.Parallel()
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	for _, command := range [][]string{
		{"self", "uninstall", "--dry-run", "--json"},
		{"client", "disable", codex.ID, "--dry-run", "--json"},
		{"client", "disable", codex.ID, "--json"},
	} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
	}
	if written := string(readFile(t, target)); strings.Contains(written, "managed by AIGW") || strings.Contains(written, "gpt-terra") {
		t.Fatalf("the withdrawal left AIGW's values:\n%s", written)
	}
	happ, houtput, _, htarget := shippedHermesApp(t)
	hermesNativeSelection(t, htarget, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if err := cli.Execute(happ, []string{"client", "disable", hermes.ID, "--json"}); err != nil {
		t.Fatalf("disable Hermes after a choice: %v\n%s", err, houtput)
	}
}

// Status and check report a model chosen in a client as that client's
// selection, name it as followed with the earlier selection and the command
// that returns, and write nothing; a check that fails says it too.
func TestStatusAndCheckReportAModelChosenInTheClient(t *testing.T) {
	t.Parallel()
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	before := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path())
	for _, command := range [][]string{{"status", "--json"}, {"check", codex.ID, "--depth", "local", "--json"}} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		var report struct {
			Followed []followed   `json:"followed_selections"`
			Clients  clientStates `json:"clients"`
		}
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		if !slices.Equal(report.Followed, codexChoice) {
			t.Errorf("%v follows %+v", command, report.Followed)
		}
		if entry := report.Clients[codex.ID]; strings.Contains(entry.Detail, "drift") || entry.State == "invalid" {
			t.Errorf("%v reports the choice as a fault: %+v", command, entry)
		}
		if !strings.Contains(output.String(), `"gateway/gpt-terra"`) {
			t.Errorf("%v does not name the chosen model as the selection:\n%s", command, output)
		}
	}
	for _, command := range [][]string{{"status"}, {"check", codex.ID, "--depth", "local"}, nil} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		saysCodexChoice(t, command, output.String())
	}
	// A command that neither saves nor reports selections says nothing of it.
	output.Reset()
	if err := cli.Execute(app, []string{"model", "list", "--json"}); err != nil || strings.Contains(output.String(), "followed_selections") {
		t.Fatalf("model list names followed selections: %v\n%s", err, output)
	}
	// A check that fails, here for a Token that is gone, names the choice in
	// text as its document does.
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	failing := []string{"check", codex.ID, "--depth", "local"}
	output.Reset()
	if err := cli.Execute(app, failing); err == nil || strings.Count(output.String(), codexChoiceText) != 1 {
		t.Fatalf("a failing check: %v\n%s", err, output)
	}
	if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, before) {
		t.Fatal("a report changed client files or the configuration")
	}
}

// The command that returns to the earlier model is named only while it can
// run: not when the command itself removed that model, and as prose when the
// model would need quoting in a shell.
func TestTheReturningCommandIsNamedOnlyWhileItRuns(t *testing.T) {
	t.Parallel()
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	removed := []followed{{Client: "codex", Model: "gateway/gpt-terra", Earlier: "gateway/gpt-sol"}}
	if got := followedIn(t, app, output, "model", "remove", "gateway/gpt-sol", "--dry-run", "--json"); !slices.Equal(got, removed) {
		t.Fatalf("removing the earlier model, previewed, follows %+v", got)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"model", "remove", "gateway/gpt-sol"}); err != nil ||
		!strings.Contains(output.String(), "AIGW held gateway/gpt-sol. The configuration no longer holds that model.") || strings.Contains(output.String(), "aigw use codex") {
		t.Fatalf("removing the earlier model: %v\n%s", err, output)
	}

	quoted, quotedOutput, targets := codexChoiceAppWith(t, "gpt-sol[1m]", 1)
	chosen := strings.Replace(string(readFile(t, targets[0])), `model = "gpt-sol[1m]" # managed by AIGW`, `model = "gpt-terra"`, 1)
	writeFile(t, targets[0], []byte(chosen), 0o600)
	prose := []followed{{Client: "codex", Model: "gateway/gpt-terra", Earlier: "gateway/gpt-sol[1m]"}}
	if got := followedIn(t, quoted, quotedOutput, "status", "--json"); !slices.Equal(got, prose) {
		t.Fatalf("an earlier model that needs quoting follows %+v\n%s", got, quotedOutput)
	}
	quotedOutput.Reset()
	if err := cli.Execute(quoted, []string{"status"}); err != nil ||
		!strings.Contains(quotedOutput.String(), "AIGW held gateway/gpt-sol[1m]. Select gateway/gpt-sol[1m] for Codex again with `aigw use`, quoting the model for your shell.") {
		t.Fatalf("status with an earlier model that needs quoting: %v\n%s", err, quotedOutput)
	}
}

// Claude Code's own model is not a selection that AIGW records: status names
// the value beside AIGW's selection, in text and JSON. A command that writes
// none of Claude Code's settings leaves it; what a rewrite of the settings
// does to it belongs to a separate Change.
func TestStatusNamesTheModelThatClaudeCodeItselfSelects(t *testing.T) {
	t.Parallel()
	app, output, credentials, _, _ := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	if err := credentials.Set("one", "test-token"); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	synchronizeClaudeProjection(t, app, cfg)
	path := claudeSettingsPath(t, app.Host)
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(t, path), &settings); err != nil {
		t.Fatal(err)
	}
	settings["model"] = json.RawMessage(`"opus[1m]"`)
	own, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, own, 0o600)

	output.Reset()
	if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
		t.Fatalf("status: %v\n%s", err, output)
	}
	var report struct {
		Followed []followed `json:"followed_selections"`
		Clients  map[string]struct {
			Model       string `json:"model"`
			ClientModel string `json:"client_model"`
			Native      bool   `json:"native_model_preference"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if entry := report.Clients[claude.ID]; entry.Model != "one/claude-test" || entry.ClientModel != "opus[1m]" || !entry.Native || len(report.Followed) != 0 {
		t.Fatalf("status = %+v, followed %+v\n%s", entry, report.Followed, output)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil || !strings.Contains(strings.Join(strings.Fields(output.String()), " "), "Claude Code uses a native model preference, opus[1m];") {
		t.Fatalf("status text: %v\n%s", err, output)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil || !bytes.Equal(readFile(t, path), own) {
		t.Fatalf("sync changed Claude Code's own model: %v", err)
	}
}
