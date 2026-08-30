package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"errors"
	"strings"
	"testing"
)

func TestRollbackHandlesAbsentAndLastChangeBackups(t *testing.T) {
	t.Run("no restore source", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
		err := execute(t, app, "rollback")
		if err == nil || !strings.Contains(err.Error(), "No fully verified checkpoint") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("last change", func(t *testing.T) {
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
	})
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
	app, _, _, _ := testApp(t, "")
	app.Updater = &fakeUpdater{rollbackErr: errors.New("no previous portable AIGW binary is available")}
	err := execute(t, app, "update", "--rollback")
	if err == nil || !strings.Contains(err.Error(), "no previous portable AIGW binary") {
		t.Fatalf("error = %v", err)
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
