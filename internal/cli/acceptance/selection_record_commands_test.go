package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"

	"go.yaml.in/yaml/v4"
)

// F1c: the Token is there when the unrelated command commits.
func TestRecordHermesPendingPickedWithTokenThenUnrelatedCommand(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"provider", "edit", "ucloud", "--label", "Other"}); err != nil {
		t.Fatalf("change the configuration: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("dmxapi"); err != nil {
		t.Fatal(err)
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if err := cli.Execute(app, []string{"config", "rollback", "--last-change", "--json"}); err != nil {
		t.Fatal(err)
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	record := readFile(t, target+".aigw-state.json")
	if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "client", "edit", codex.ID, "--request-retries", "2", "--json"); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
		t.Errorf("with the Token, a committed transaction about another client left the record behind the file (S=%s)", selection(t, app, hermes.ID).Model)
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Errorf("synchronization wrote %s %s over the model chosen last; S=%s", provider, model, selection(t, app, hermes.ID).Model)
	}
}

// An explicit selection of the stored model after a choice of another
// provider's model: what becomes of the stored forwarding?
func TestRecordExplicitReturnAndTheStoredForwarding(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"client", "edit", hermes.ID, "--via", "http://127.0.0.1:9000"}); err != nil {
		t.Fatalf("forward Hermes: %v\n%s", err, output)
	}
	back := []string{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses", "--json"}
	if err := recRun(t, app, output, back...); err != nil {
		t.Fatal(err)
	}
	if got := selection(t, app, hermes.ID).ForwardingEndpoint; got != "http://127.0.0.1:9000" {
		t.Fatalf("without a choice, selecting the stored model changed the forwarding to %q", got)
	}
	hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	_ = recRun(t, app, output, append(back, "--dry-run")...)
	if err := recRun(t, app, output, back...); err != nil {
		t.Fatal(err)
	}
	said := strings.Contains(output.String(), "forwarding_cleared")
	if got := selection(t, app, hermes.ID).ForwardingEndpoint; got != "http://127.0.0.1:9000" {
		t.Errorf("after a choice in Hermes, `aigw use` of the stored model left forwarding %q (was http://127.0.0.1:9000); the result says forwarding_cleared: %t", got, said)
	}
	t.Logf("file after the return:\n%s", readFile(t, target)[:400])
}

// Reports and other commands' own results while Hermes's file shows a choice
// whose provider has no Token.
func TestRecordHermesReportsWithAChoiceAndNoToken(t *testing.T) {
	app, output, target, chosen := waitingHermes(t)
	record := readFile(t, target+".aigw-state.json")
	for _, command := range [][]string{
		nil,
		{"check", "hermes", "--depth", "local"},
		{"client", "list"},
		{"provider", "list"},
		{"sync", "--dry-run"},
		{"config", "rollback", "--last-change", "--dry-run"},
		{"use", "claude", "ucloud/claude-opus-5-5", "--dry-run", "--json"},
		{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--dry-run"},
		{"model", "remove", "dmxapi/gpt-6.2-sol", "--dry-run"},
	} {
		_ = recRun(t, app, output, command...)
		if !bytes.Equal(readFile(t, target+".aigw-state.json"), record) || !bytes.Equal(readFile(t, target), chosen) || selection(t, app, hermes.ID).Model.String() != "dmxapi/gpt-6.2-sol" {
			t.Errorf("%v wrote the record, the file or the stored selection", command)
		}
	}
	// A selection for another client records Hermes's choice.
	if err := recRun(t, app, output, "use", "claude", "dmxapi/claude-sonnet-5-5", "--json"); err != nil {
		t.Errorf("use claude: %v", err)
	}
	t.Logf("after use claude: S(hermes)=%s recordMoved=%t fileKept=%t", selection(t, app, hermes.ID).Model, !bytes.Equal(readFile(t, target+".aigw-state.json"), record), bytes.Equal(readFile(t, target), chosen))
	for _, command := range [][]string{{"status"}, {"sync"}, {"check", "hermes", "--depth", "local"}} {
		_ = recRun(t, app, output, command...)
	}
	// The Token arrives.
	if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"status"}, {"check", "hermes", "--depth", "local"}, {"sync", "--dry-run", "--json"}, {"sync", "--json"}, {"check", "hermes", "--depth", "local", "--json"}} {
		if err := recRun(t, app, output, command...); err != nil {
			t.Errorf("%v after the Token: %v", command, err)
		}
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-ucloud-anthropic" || model != "claude-opus-5-5" {
		t.Errorf("after the Token, the file selects %s %s", provider, model)
	}
}

// setup --from with a recommendation of another model for Hermes.
func TestRecordTeamSetupWithAChoiceAndNoToken(t *testing.T) {
	app, output, target, chosen := waitingHermes(t)
	record := readFile(t, target+".aigw-state.json")
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	_ = recRun(t, app, output, "setup", "--from", manifest, "--dry-run")
	if !bytes.Equal(readFile(t, target+".aigw-state.json"), record) || selection(t, app, hermes.ID).Model.String() != "dmxapi/gpt-6.2-sol" {
		t.Errorf("the setup preview wrote the record or the selection")
	}
	if err := recRun(t, app, output, "setup", "--from", manifest); err != nil {
		t.Errorf("setup: %v", err)
	}
	t.Logf("after setup: S(hermes)=%s recordMoved=%t fileKept=%t", selection(t, app, hermes.ID).Model, !bytes.Equal(readFile(t, target+".aigw-state.json"), record), bytes.Equal(readFile(t, target), chosen))
	if selection(t, app, hermes.ID).Model.String() != "ucloud/claude-opus-5-5" || bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
		t.Errorf("a team setup did not record the choice with the record")
	}
}

// The row unknown, and an outside edit, while the stored selection's provider
// has no Token: nothing may be recorded.
func TestRecordUnknownAndOutsideEditWithoutAToken(t *testing.T) {
	for name, pick := range map[string]func(t *testing.T, target string){
		"unlisted": func(t *testing.T, target string) {
			hermesNativeSelection(t, target, "aigw-dmxapi-responses", "zz-not-listed", nil)
		},
		"outside": func(t *testing.T, target string) {
			hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", func(provider *yaml.Node) {
				for index := 0; index+1 < len(provider.Content); index += 2 {
					if provider.Content[index].Value == "base_url" {
						provider.Content[index+1].Value = "https://foreign.test"
					}
				}
			})
		},
	} {
		app, output, _, target := shippedHermesAppWith(t, true)
		if err := app.Secrets.Delete("dmxapi"); err != nil {
			t.Fatal(err)
		}
		pick(t, target)
		file, record := readFile(t, target), readFile(t, target+".aigw-state.json")
		for _, command := range [][]string{
			{"status"},
			{"sync", "--json"},
			{"provider", "edit", "ucloud", "--label", "Other", "--json"},
			{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"},
			{"client", "disable", "hermes", "--dry-run", "--json"},
		} {
			err := recRun(t, app, output, command...)
			t.Logf("%s %v: err=%v S=%s", name, command, err, selection(t, app, hermes.ID).Model)
			if !bytes.Equal(readFile(t, target+".aigw-state.json"), record) || !bytes.Equal(readFile(t, target), file) {
				t.Errorf("%s: %v wrote the record or the file", name, command)
			}
		}
	}
}

// Codex, one file, no Token: previews and refusals write nothing; removing the
// earlier model records.
func TestRecordCodexOneFileWithoutAToken(t *testing.T) {
	app, output, targets := recCodexFiles(t, 1)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	file := readFile(t, targets[0])
	for _, command := range [][]string{
		{"provider", "edit", "spare", "--label", "Other", "--dry-run", "--json"},
		{"use", "codex", "gateway/gpt-luna", "--dry-run", "--json"},
		{"use", "codex", "gateway/gpt-luna", "--json"},
		{"use", "codex", "gateway/gpt-terra", "--json"},
		{"model", "remove", "gateway/gpt-terra", "--json"},
		{"client", "edit", "codex", "--target", filepath.Join(t.TempDir(), "config.toml"), "--json"},
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
	} {
		err := recRun(t, app, output, command...)
		if recCodexRecorded(t, targets[0]) != "gpt-sol" || selection(t, app, codex.ID).Model.String() != "gateway/gpt-sol" || !bytes.Equal(readFile(t, targets[0]), file) {
			t.Errorf("%v (err=%v) wrote the record, the selection or the file", command, err)
		}
	}
	if err := recRun(t, app, output, "model", "remove", "gateway/gpt-sol", "--json"); err != nil {
		t.Errorf("removing the earlier model: %v", err)
	}
	recCodexState(t, app, targets, "after removing the earlier model")
	if !bytes.Equal(readFile(t, targets[0]), file) {
		t.Errorf("the removal wrote Codex's file without a Token")
	}
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"sync", "--json"}, {"check", "codex", "--depth", "local", "--json"}} {
		if err := recRun(t, app, output, command...); err != nil {
			t.Errorf("%v after the Token: %v", command, err)
		}
	}
	recCodexState(t, app, targets, "after the Token and sync")
}
