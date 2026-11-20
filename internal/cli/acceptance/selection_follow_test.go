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
	app, output, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Providers["spare"] = domain.Provider{Label: "Spare", Endpoints: domain.Endpoints{Responses: "https://spare.test/v1"}}
	cfg.Models[domain.NewModelRef("gateway", "gpt-sol")] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("gateway", "gpt-terra")] = qualifiedModel("Terra", "gateway", "gpt-terra", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("spare", "gpt-spare")] = qualifiedModel("Spare", "spare", "gpt-spare", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-sol"), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
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
	return app, output, target
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
	Client      string `json:"client"`
	Model       string `json:"model"`
	Earlier     string `json:"earlier"`
	ReturnsWith string `json:"returns_with"`
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
	hermesChoice = []followed{{Client: "hermes", Model: "dmxapi/claude-opus-5-5", Earlier: "dmxapi/gpt-6.2-sol", ReturnsWith: "aigw use hermes dmxapi/gpt-6.2-sol --protocol responses"}}
)

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
// the document says so with the command that returns.
func TestACommandThatWritesCodexFilesFollowsAModelChosenInCodex(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	directory := filepath.Dir(target)

	// A preview names the followed model and writes nothing.
	before := ownedFileSnapshot(t, directory, app.Config.Path())
	if got := followedIn(t, app, output, "client", "edit", "codex", "--request-retries", "2", "--dry-run", "--json"); !slices.Equal(got, codexChoice) {
		t.Fatalf("preview follows %+v", got)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"client", "edit", "codex", "--request-retries", "2", "--dry-run"}); err != nil {
		t.Fatalf("text preview: %v\n%s", err, output)
	}
	for _, want := range []string{"Followed selection", "gateway/gpt-terra, chosen in Codex", "`aigw use codex gateway/gpt-sol` returns to gateway/gpt-sol"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("the text preview lacks %q:\n%s", want, output)
		}
	}
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

// A command that changes nothing in Codex's files records the choice, says
// so, and leaves Codex's file as it is; the checks pass on that state.
func TestACommandThatWritesNoneOfCodexFilesRecordsItsChoice(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chosen := chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	if got := followedIn(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json"); !slices.Equal(got, codexChoice) {
		t.Fatalf("the run follows %+v", got)
	}
	if string(readFile(t, target)) != chosen {
		t.Fatal("the command rewrote Codex's file")
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-terra") {
		t.Fatalf("the configuration does not name the chosen model: %#v", binding)
	}
	for _, command := range [][]string{{"check", codex.ID, "--depth", "local", "--json"}, {"status", "--json"}} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil || strings.Contains(output.String(), "drift") {
			t.Fatalf("%v after a recorded choice: %v\n%s", command, err, output)
		}
	}
}

// An explicit selection is not followed away: it writes the named model over
// the one that was chosen in Codex, and its document follows nothing; its
// preview names the choice that it would replace.
func TestAnExplicitSelectionWinsOverAModelChosenInCodex(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	if got := followedIn(t, app, output, "use", "codex", "gateway/gpt-sol", "--dry-run", "--json"); !slices.Equal(got, codexChoice) {
		t.Fatalf("the preview follows %+v", got)
	}
	if got := followedIn(t, app, output, "use", "codex", "gateway/gpt-sol", "--json"); len(got) != 0 {
		t.Fatalf("an explicit selection follows %+v", got)
	}
	if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-sol" # managed by AIGW`) {
		t.Fatalf("the explicit selection was not written:\n%s", written)
	}
}

// Hermes: a command that writes Hermes' file works from the listed model that
// was chosen in Hermes, and keeps its own edit of Hermes' binding.
func TestACommandThatWritesHermesFilesFollowsAModelChosenInHermes(t *testing.T) {
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

// Synchronization says what it followed in the same words, text and JSON.
func TestSyncSaysWhatItFollowedAndHowToReturn(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	for _, command := range [][]string{{"sync", "--dry-run"}, {"sync"}} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		for _, want := range []string{"Followed selection", "gateway/gpt-terra, chosen in Codex", "`aigw use codex gateway/gpt-sol` returns to gateway/gpt-sol"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("%v lacks %q:\n%s", command, want, output)
			}
		}
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil || strings.Contains(output.String(), "Followed selection") {
		t.Fatalf("a second sync still follows: %v\n%s", err, output)
	}
}

// Disabling a client after a model was chosen in it withdraws what AIGW
// wrote: the choice is no outside edit to undo.
func TestAClientIsDisabledAfterAModelWasChosenInIt(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	for _, command := range [][]string{{"client", "disable", codex.ID, "--dry-run", "--json"}, {"client", "disable", codex.ID, "--json"}} {
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
// that returns, and write nothing.
func TestStatusAndCheckReportAModelChosenInTheClient(t *testing.T) {
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
		for _, want := range []string{"Followed selection", "gateway/gpt-terra, chosen in Codex", "`aigw use codex gateway/gpt-sol` returns to gateway/gpt-sol"} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("%v lacks %q:\n%s", command, want, output)
			}
		}
	}
	if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, before) {
		t.Fatal("a report changed client files or the configuration")
	}
	// A command that neither saves nor reports selections says nothing of it.
	output.Reset()
	if err := cli.Execute(app, []string{"model", "list", "--json"}); err != nil || strings.Contains(output.String(), "followed_selections") {
		t.Fatalf("model list names followed selections: %v\n%s", err, output)
	}
}

// Claude Code's own model is not a selection that AIGW records: status names
// the value beside AIGW's selection, in text and JSON, and no command but an
// explicit selection changes it.
func TestStatusNamesTheModelThatClaudeCodeItselfSelects(t *testing.T) {
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
