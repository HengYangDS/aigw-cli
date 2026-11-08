package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// editClaudeSettings changes the settings file as a person would, outside AIGW.
func editClaudeSettings(edit func(document, environment map[string]any)) func(*testing.T, string) {
	return func(t *testing.T, settings string) {
		t.Helper()
		var document map[string]any
		if err := json.Unmarshal(readFile(t, settings), &document); err != nil {
			t.Fatal(err)
		}
		environment, _ := document["env"].(map[string]any)
		edit(document, environment)
		data, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, settings, data, 0o600)
	}
}

// AIGW neither overwrites nor adopts an outside edit to the settings that it
// manages for Claude Code. Every surface that reports the refusal names the one
// action that can succeed on the refused state, so no fix leads to a command
// that refuses again, and that action lets synchronization and the local checks
// pass.
func TestClaudeOutsideEditRecoveryDoesNotLoop(t *testing.T) {
	const undo = "Undo the outside edit to the AIGW-managed Claude Code settings, then run `aigw sync`"
	const correct = "Correct the JSON in Claude Code's settings file, then run the same command again."
	for _, test := range []struct {
		name     string
		recovery string
		edit     func(*testing.T, string)
	}{
		{"managed endpoint", undo, editClaudeSettings(func(_, environment map[string]any) {
			environment["ANTHROPIC_BASE_URL"] = "https://user.example.test"
		})},
		{"credential helper", undo, editClaudeSettings(func(document, _ map[string]any) {
			document["apiKeyHelper"] = "foreign-helper"
		})},
		{"deleted file", "Restore settings.json, or delete settings.json.aigw-state.json in its folder if settings.json is gone for good, then run `aigw sync`", func(t *testing.T, settings string) {
			t.Helper()
			if err := os.Remove(settings); err != nil {
				t.Fatal(err)
			}
		}},
		{"malformed file", correct, func(t *testing.T, settings string) {
			t.Helper()
			writeFile(t, settings, []byte("{"), 0o600)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertClaudeOutsideEditRecovery(t, test.recovery, test.edit)
		})
	}
}

func assertClaudeOutsideEditRecovery(t *testing.T, recovery string, edit func(*testing.T, string)) {
	t.Helper()
	app, output, secretStore, _, _ := testApp(t, "")
	bindCredentialReader(t, app)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["opus"] = qualifiedModel("Opus", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, "opus", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project the Claude Code selection: %v\n%s", err, output)
	}
	settings := claudeSettingsPath(t, app.Host)
	projected := readFile(t, settings)
	edit(t, settings)
	assertRecoveryOnEverySurface(t, app, output, claude.ID, recovery, filepath.Dir(settings), [][]string{
		{"use", claude.ID, "gateway/claude-test", "--dry-run", "--json"},
		{"client", "disable", claude.ID, "--dry-run", "--json"},
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"repair", "--json"},
		{"check", claude.ID, "--depth", "local", "--json"},
		{"status", "--json"},
		{"doctor", "--json"},
	})

	writeFile(t, settings, projected, 0o600)
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
		t.Fatalf("sync after the recovery: %v\n%s", err, output)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"check", claude.ID, "--depth", "local", "--json"}); err != nil {
		t.Fatalf("check after the recovery: %v\n%s", err, output)
	}
	if checked := decodeCheck(t, output); !checked.OK {
		t.Fatalf("check after the recovery = %+v\n%s", checked, output)
	}
}
