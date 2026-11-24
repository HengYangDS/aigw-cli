package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

func recRun(t *testing.T, app *cli.App, output *bytes.Buffer, args ...string) error {
	t.Helper()
	output.Reset()
	err := cli.Execute(app, args)
	text := output.String()
	if len(text) > 2500 {
		text = text[:2500] + "\n...[cut]"
	}
	t.Logf("$ aigw %s\n  err=%v\n%s", strings.Join(args, " "), err, text)
	return err
}

// recCodexModelLine returns the lines of a Codex file and the index of its
// root model assignment, or -1. The line is found by the TOML parser, so that
// every spelling of the assignment is read: it is the line before the first
// table that parses into the one key model with a string value.
func recCodexModelLine(t *testing.T, target string) ([]string, int) {
	t.Helper()
	lines := strings.Split(string(readFile(t, target)), "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			break
		}
		var assignment map[string]any
		if err := toml.Unmarshal([]byte(strings.TrimSuffix(line, "\r")), &assignment); err != nil || len(assignment) != 1 {
			continue
		}
		if _, selects := assignment["model"].(string); selects {
			return lines, index
		}
	}
	return lines, -1
}

// recCodexModel returns the root model line of a Codex file as it is written.
func recCodexModel(t *testing.T, target string) string {
	t.Helper()
	if lines, index := recCodexModelLine(t, target); index >= 0 {
		return lines[index]
	}
	return "<none>"
}

func recPick(t *testing.T, target, to string) {
	t.Helper()
	recPickAs(t, target, `model = "`+to+`"`)
}

// recPickAs writes line in place of the root model line of a Codex file, as a
// person who edits the file does, in whatever spelling line has.
func recPickAs(t *testing.T, target, line string) {
	t.Helper()
	lines, index := recCodexModelLine(t, target)
	if index < 0 {
		t.Fatalf("no root model in %s", target)
	}
	lines[index] = line
	writeFile(t, target, []byte(strings.Join(lines, "\n")), 0o600)
}

func recCodexRecorded(t *testing.T, target string) string {
	t.Helper()
	paths := []string{target + ".aigw-state.json"}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		paths = append(paths, resolved+".aigw-state.json")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var state struct {
			ProjectedModel string `json:"projected_model"`
		}
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		return state.ProjectedModel
	}
	return "<no record>"
}

// recCodexFiles projects gateway/gpt-sol for Codex into count files, with a
// credential reader, so that a missing Token defers Codex.
func recCodexFiles(t *testing.T, count int) (*cli.App, *bytes.Buffer, []string) {
	t.Helper()
	app, output, secretStore, _, _ := testApp(t, "")
	bindCredentialReader(t, app)
	targets := make([]string, 0, count)
	for range count {
		target := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
		targets = append(targets, target)
	}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Providers["spare"] = domain.Provider{Label: "Spare", Endpoints: domain.Endpoints{Responses: "https://spare.test/v1"}}
	for _, model := range []string{"gpt-sol", "gpt-terra", "gpt-luna"} {
		cfg.Models[domain.NewModelRef("gateway", model)] = qualifiedModel(model, "gateway", model, domain.ProtocolResponses)
	}
	cfg.Models[domain.NewModelRef("spare", "gpt-spare")] = qualifiedModel("Spare", "spare", "gpt-spare", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-sol"), "")
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

func recCodexState(t *testing.T, app *cli.App, targets []string, at string) {
	t.Helper()
	line := at + ": S=" + selection(t, app, codex.ID).Model.String()
	for index, target := range targets {
		line += " | file" + string(rune('A'+index)) + " F=[" + recCodexModel(t, target) + "] R=" + recCodexRecorded(t, target)
	}
	t.Log(line)
}

// H1: Hermes. A pending selection (after a rollback over a choice without the
// Token), picked in Hermes itself, then the earlier model picked again.
func TestRecordHermesPickOfThePendingSelection(t *testing.T) {
	t.Parallel()
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"provider", "edit", "ucloud", "--label", "Other"}); err != nil {
		t.Fatalf("change the configuration: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("dmxapi"); err != nil {
		t.Fatal(err)
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if err := recRun(t, app, output, "config", "rollback", "--last-change", "--json"); err != nil {
		t.Fatal(err)
	}
	t.Logf("after rollback: S=%s", selection(t, app, hermes.ID).Model)
	// The person picks in Hermes the model that AIGW names as selected.
	picked := hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	record := readFile(t, target+".aigw-state.json")
	if err := recRun(t, app, output, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, target), picked) {
		t.Fatal("sync wrote the file without a Token")
	}
	if bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
		t.Errorf("a committed transaction left AIGW's record behind the model that Hermes's file shows (S=%s)", selection(t, app, hermes.ID).Model)
	}
	// The person goes back to the model picked before.
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if err := recRun(t, app, output, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Errorf("synchronization wrote %s %s over the model chosen last (claude-opus-5-5); S=%s", provider, model, selection(t, app, hermes.ID).Model)
	}
}

// H2: Codex, two files, no Token. One file chooses; the record-only write;
// the other file chooses the same model; then goes back.
func TestRecordCodexSecondFilePicksTheRecordedChoice(t *testing.T) {
	t.Parallel()
	app, output, targets := recCodexFiles(t, 2)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "after A picked terra and sync")
	for _, command := range [][]string{
		{"status"}, {"check", "--depth", "local"}, {"sync", "--dry-run", "--json"}, {"client", "list", "--json"},
		{"client", "disable", "codex", "--dry-run", "--json"}, {"self", "uninstall", "--dry-run", "--json"},
	} {
		_ = recRun(t, app, output, command...)
	}
	recPick(t, targets[1], "gpt-terra")
	_ = recRun(t, app, output, "status", "--json")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "after B picked terra and sync")
	if recorded := recCodexRecorded(t, targets[1]); recorded != "gpt-terra" {
		t.Errorf("a committed transaction left the record of file B at %s while the file shows gpt-terra", recorded)
	}
	recPick(t, targets[1], "gpt-sol")
	_ = recRun(t, app, output, "status", "--json")
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "after B picked sol, Token, sync")
	if !strings.Contains(recCodexModel(t, targets[1]), "gpt-sol") {
		t.Errorf("synchronization wrote %s over the model chosen last in file B (gpt-sol)", recCodexModel(t, targets[1]))
	}
	_ = recRun(t, app, output, "check", "codex", "--depth", "local", "--json")
}

// Codex, two files, no Token: after the record-only write the Token arrives.
// The file that the choice was not made in is written at the next
// synchronization, the ordinary state of a selection that waited for its Token.
func TestRecordCodexRecordsThenTokenArrives(t *testing.T) {
	t.Parallel()
	app, output, targets := recCodexFiles(t, 2)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	if err := recRun(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "after A picked terra and an unrelated edit")
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	// The choice is recorded and its files wait for the Token, as any
	// selection's do: once the Token is stored the local check names the
	// step that writes them, and that step converges the client.
	if err := recRun(t, app, output, "status"); err != nil {
		t.Errorf("status after the Token arrived: %v", err)
	}
	if err := recRun(t, app, output, "check", "codex", "--depth", "local"); err == nil || !strings.Contains(output.String(), "aigw sync") {
		t.Errorf("the local check before synchronization = %v, want the pending files with `aigw sync` as the step\n%s", err, output)
	}
	for _, command := range [][]string{{"sync", "--dry-run", "--json"}, {"sync", "--json"}, {"check", "codex", "--depth", "local", "--json"}} {
		if err := recRun(t, app, output, command...); err != nil {
			t.Errorf("%v after the Token arrived: %v", command, err)
		}
	}
	recCodexState(t, app, targets, "after Token and sync")
}

// Codex, two files, no Token: after the record-only write the other file
// picks a third model; then the withdrawal.
func TestRecordCodexSecondFilePicksAThirdModel(t *testing.T) {
	t.Parallel()
	app, output, targets := recCodexFiles(t, 2)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[1], "gpt-luna")
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Errorf("sync after B picked luna: %v", err)
	}
	recCodexState(t, app, targets, "after B picked luna and sync")
	_ = recRun(t, app, output, "check", "codex", "--depth", "local")
	if err := recRun(t, app, output, "client", "disable", "codex", "--json"); err != nil {
		t.Errorf("client disable: %v", err)
	}
	for index, target := range targets {
		t.Logf("file %d after disable:\n%s", index, readFile(t, target))
	}
}
