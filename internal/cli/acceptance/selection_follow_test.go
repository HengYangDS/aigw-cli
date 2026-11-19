package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

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

// followedRow runs a command with --json, which must succeed, and returns
// the client-file row of client.
func followedRow(t *testing.T, app *cli.App, output *bytes.Buffer, client string, command ...string) (followed, returns string) {
	t.Helper()
	output.Reset()
	if err := cli.Execute(app, command); err != nil {
		t.Fatalf("%v: %v\n%s", command, err, output)
	}
	var result struct {
		Files []struct {
			Client   string `json:"client"`
			Followed string `json:"followed_selection"`
			Returns  string `json:"returns_with"`
		} `json:"client_files"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("%v: %v\n%s", command, err, output)
	}
	for _, file := range result.Files {
		if file.Client == client {
			return file.Followed, file.Returns
		}
	}
	t.Fatalf("%v names no file of %s:\n%s", command, client, output)
	return "", ""
}

// A command other than sync that writes Codex's files, here an edit of its
// retry budget, follows the model that was chosen in Codex: it succeeds,
// Codex keeps the model, the configuration names it, and the result says so
// with the command that returns.
func TestACommandThatWritesCodexFilesFollowsAModelChosenInCodex(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	directory := filepath.Dir(target)

	// A preview names the followed model and writes nothing.
	before := ownedFileSnapshot(t, directory, app.Config.Path())
	followed, returns := followedRow(t, app, output, codex.ID, "client", "edit", "codex", "--request-retries", "2", "--dry-run", "--json")
	if followed != "gateway/gpt-terra" || returns != "aigw use codex gateway/gpt-sol" {
		t.Fatalf("preview row = %q, %q", followed, returns)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"client", "edit", "codex", "--request-retries", "2", "--dry-run"}); err != nil {
		t.Fatalf("text preview: %v\n%s", err, output)
	}
	if !strings.Contains(output.String(), "Follows gateway/gpt-terra, chosen in Codex; `aigw use codex gateway/gpt-sol` returns to the earlier model") {
		t.Fatalf("the text preview does not say what it follows:\n%s", output)
	}
	if after := ownedFileSnapshot(t, directory, app.Config.Path()); !equalSnapshots(after, before) {
		t.Fatal("the preview changed client files or the configuration")
	}

	followed, returns = followedRow(t, app, output, codex.ID, "client", "edit", "codex", "--request-retries", "2", "--json")
	if followed != "gateway/gpt-terra" || returns != "aigw use codex gateway/gpt-sol" {
		t.Fatalf("result row = %q, %q", followed, returns)
	}
	written := string(readFile(t, target))
	for _, want := range []string{`model = "gpt-terra" # managed by AIGW`, "request_max_retries = 2"} {
		if !strings.Contains(written, want) {
			t.Fatalf("Codex's file lacks %q:\n%s", want, written)
		}
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-terra") || cfg.Clients[codex.ID].RetryPolicy == nil {
		t.Fatalf("the configuration does not name the chosen model with the edit: %#v, %v", cfg.Clients[codex.ID], err)
	}
	syncAndCheckPass(t, app, output, codex.ID)

	// The command that the row names returns to the earlier model.
	if err := cli.Execute(app, strings.Fields(returns)[1:]); err != nil {
		t.Fatalf("%q: %v\n%s", returns, err, output)
	}
	if cfg, err := app.Config.Load(); err != nil || cfg.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-sol") {
		t.Fatalf("the returning command did not select the earlier model: %v", err)
	}
}

// A command that changes nothing in Codex's files leaves Codex's file and
// its recorded selection as they are.
func TestACommandThatWritesNoneOfCodexFilesLeavesItsChoice(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chosen := chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	output.Reset()
	if err := cli.Execute(app, []string{"provider", "edit", "spare", "--label", "Other", "--json"}); err != nil {
		t.Fatalf("an edit of a provider that Codex does not use: %v\n%s", err, output)
	}
	if string(readFile(t, target)) != chosen {
		t.Fatal("the command rewrote Codex's file")
	}
	if cfg, err := app.Config.Load(); err != nil || cfg.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-sol") {
		t.Fatalf("the command changed Codex's recorded selection: %v", err)
	}
}

// An explicit selection is not followed away: it writes the named model over
// the one that was chosen in Codex.
func TestAnExplicitSelectionWinsOverAModelChosenInCodex(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	if err := cli.Execute(app, []string{"use", "codex", "gateway/gpt-sol"}); err != nil {
		t.Fatalf("explicit selection: %v\n%s", err, output)
	}
	if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-sol" # managed by AIGW`) {
		t.Fatalf("the explicit selection was not written:\n%s", written)
	}
}

// Hermes: a command other than sync that writes Hermes' file follows the
// listed model that was chosen in Hermes.
func TestACommandThatWritesHermesFilesFollowsAModelChosenInHermes(t *testing.T) {
	app, output, _, target := shippedHermesApp(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	followed, returns := followedRow(t, app, output, hermes.ID, "provider", "edit", "dmxapi", "--label", "Renamed", "--json")
	if followed != "dmxapi/claude-opus-5-5" || returns != "aigw use hermes dmxapi/gpt-6.2-sol --protocol responses" {
		t.Fatalf("result row = %q, %q", followed, returns)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("Hermes lost its choice: %s %s", provider, model)
	}
	if cfg, err := app.Config.Load(); err != nil || cfg.SelectedModel(hermes.ID) != domain.NewModelRef("dmxapi", "claude-opus-5-5") {
		t.Fatalf("the configuration does not name the chosen model: %v", err)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}

// Synchronization words what it followed with the sentence that every other
// command uses, and keeps the field that scripts read.
func TestSyncSaysWhatItFollowedAndHowToReturn(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run"}); err != nil {
		t.Fatalf("preview: %v\n%s", err, output)
	}
	const sentence = "Follows gateway/gpt-terra, chosen in Codex; `aigw use codex gateway/gpt-sol` returns to the earlier model"
	if !strings.Contains(output.String(), sentence) {
		t.Fatalf("the preview does not say what it follows:\n%s", output)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, output)
	}
	if !strings.Contains(output.String(), sentence) {
		t.Fatalf("sync does not say what it followed:\n%s", output)
	}
	// Once followed, nothing is left to follow or to say.
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil || strings.Contains(output.String(), "adopted_selections") {
		t.Fatalf("a second sync still follows: %v\n%s", err, output)
	}
}
