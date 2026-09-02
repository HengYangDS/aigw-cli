package cli_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestRollbackReportsUnavailableRecoveryWithoutChangingCurrentConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		prepare func(*testing.T, string)
	}{
		{name: "absent recovery sources"},
		{name: "absent previous configuration", args: []string{"--last-change"}},
		{
			name: "invalid recovery sources",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				for _, recoveryPath := range []string{path + ".verified.json", path + ".bak"} {
					if err := os.WriteFile(recoveryPath, []byte("invalid\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _ := testApp(t, "")
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
			if test.prepare != nil {
				test.prepare(t, app.Config.Path())
			}
			before, readErr := os.ReadFile(app.Config.Path())
			if readErr != nil {
				t.Fatal(readErr)
			}
			err := execute(t, app, append([]string{"rollback"}, test.args...)...)
			if err == nil || err.Error() != "Configuration rollback is unavailable" {
				t.Fatalf("error = %v", err)
			}
			for _, want := range []string{
				"Configuration rollback is unavailable",
				"No valid recovery source is available for the current configuration.",
				"The current configuration remains active and unchanged.",
				"aigw doctor",
			} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("output missing %q:\n%s", want, out.String())
				}
			}
			if count := strings.Count(out.String(), "aigw doctor"); count != 1 {
				t.Fatalf("safe next action count = %d, want 1:\n%s", count, out.String())
			}
			for _, internalTerm := range []string{"backup", "checkpoint", "journal"} {
				if strings.Contains(strings.ToLower(out.String()), internalTerm) {
					t.Fatalf("output exposes internal term %q:\n%s", internalTerm, out.String())
				}
			}
			after, readErr := os.ReadFile(app.Config.Path())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(after, before) {
				t.Fatal("rollback without a recovery source changed the current configuration")
			}
		})
	}
}

func TestRollbackRestoresLastConfigurationChange(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	before := configuration.NewConfig()
	addAccountProfile(&before, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	before.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Profiles = map[string]configuration.Profile{"two": {Label: "Two", Account: "one", Client: configuration.ClientClaude, Model: "claude-two"}}
	after.Routes[configuration.ClientClaude] = "two"
	if err := app.Config.Save(after); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "rollback", "--last-change"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil || got.Routes[configuration.ClientClaude] != "one" || !strings.Contains(out.String(), "Previous configuration") {
		t.Fatalf("config=%#v output=%q error=%v", got, out.String(), err)
	}
}

func TestRollbackUsesPreviousConfigurationWhenVerifiedRecoveryIsInvalid(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	previous := configuration.NewConfig()
	addAccountProfile(&previous, "stable", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-stable")
	previous.Routes[configuration.ClientClaude] = "stable"
	if err := app.Config.Save(previous); err != nil {
		t.Fatal(err)
	}
	current := previous
	current.Profiles = map[string]configuration.Profile{"current": {Label: "Current", Account: "one", Client: configuration.ClientClaude, Model: "claude-current"}}
	current.Routes[configuration.ClientClaude] = "current"
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.Config.Path()+".verified.json", []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "rollback"); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Routes[configuration.ClientClaude] != "stable" {
		t.Fatalf("rollback route = %q, want stable", restored.Routes[configuration.ClientClaude])
	}
	if !strings.Contains(out.String(), "Previous configuration") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRollbackReportsUnconfirmedConfigurationWhenRestoreFails(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	target := t.TempDir()
	verified := configuration.NewConfig()
	addAccountProfile(&verified, "stable", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-stable")
	verified.Routes[configuration.ClientCodex] = "stable"
	verified.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(verified, []string{configuration.ClientCodex}); err != nil {
		t.Fatal(err)
	}
	current := verified.Clone()
	current.Profiles["stable"] = configuration.Profile{Label: "Stable", Account: "one", Client: configuration.ClientCodex, Model: "gpt-current"}
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "rollback")
	if err == nil || err.Error() != "Configuration rollback did not complete" {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{
		"Configuration rollback did not complete",
		"AIGW could not restore the selected configuration and its client projections.",
		"A rolled-back configuration was not confirmed.",
		"aigw doctor",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
	for _, internalTerm := range []string{"snapshot", "postimage", target} {
		if strings.Contains(out.String(), internalTerm) {
			t.Fatalf("output exposes internal detail %q:\n%s", internalTerm, out.String())
		}
	}
}

func TestUpdateRollbackUsesLocalProgramRollbackOnly(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	updater := &fakeUpdater{rollbackResult: "restored the previous program version; you can run `aigw update --rollback` again to restore the current version."}
	app.Updater = updater
	if err := execute(t, app, "update", "--rollback"); err != nil {
		t.Fatal(err)
	}
	if updater.rollbackCalls != 1 || updater.updateCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
	if !strings.Contains(out.String(), "Program rollback") || !strings.Contains(out.String(), "restored the previous program version") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestUpdateWithoutRollbackKeepsNetworkUpdatePath(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	updater := &fakeUpdater{updateResult: "updated to v0.2.0."}
	app.Updater = updater
	if err := execute(t, app, "update"); err != nil {
		t.Fatal(err)
	}
	if updater.updateCalls != 1 || updater.rollbackCalls != 0 {
		t.Fatalf("update calls=%d rollback calls=%d", updater.updateCalls, updater.rollbackCalls)
	}
}

func TestUpdateRollbackReturnsLocalRollbackError(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cause := errors.New("read retained predecessor: permission denied")
	app.Updater = &fakeUpdater{rollbackErr: cause}
	err := execute(t, app, "update", "--rollback")
	if err == nil || err.Error() != "Program rollback did not complete" || !errors.Is(err, cause) {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{
		"Program rollback did not complete",
		"AIGW could not activate the retained previous program.",
		"No previous program version was confirmed active.",
		"aigw check",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), cause.Error()) {
		t.Fatalf("output exposes implementation error:\n%s", out.String())
	}
	if count := strings.Count(out.String(), "aigw check"); count != 1 {
		t.Fatalf("safe next action count = %d, want 1:\n%s", count, out.String())
	}
}

func TestUpdateHelpDescribesOfflineProgramRollback(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "update", "--help"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Roll back the portable AIGW program to the previous version offline") {
		t.Fatalf("help = %s", out.String())
	}
}

func TestRollbackRestoresVerifiedCheckpointBeforeLastChangeBackup(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	verified := configuration.NewConfig()
	verified.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	verified.Profiles["stable"] = configuration.Profile{Label: "Stable", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-stable"}
	verified.Routes[configuration.ClientCodex] = "stable"
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(verified, []string{configuration.ClientCodex}); err != nil {
		t.Fatal(err)
	}
	current := verified
	current.Profiles = map[string]configuration.Profile{"experimental": {Label: "Experimental", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-experimental"}}
	current.Routes[configuration.ClientCodex] = "experimental"
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "rollback"); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Routes[configuration.ClientCodex] != "stable" {
		t.Fatalf("rollback route = %q, want stable", restored.Routes[configuration.ClientCodex])
	}
}
