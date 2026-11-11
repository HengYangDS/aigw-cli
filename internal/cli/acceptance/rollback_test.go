package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
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
				for _, recoveryPath := range []string{verifiedPath(path), path + ".bak"} {
					if err := os.WriteFile(recoveryPath, []byte("invalid\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
			if test.prepare != nil {
				test.prepare(t, app.Config.Path())
			}
			before, readErr := os.ReadFile(app.Config.Path())
			if readErr != nil {
				t.Fatal(readErr)
			}
			err := cli.Execute(app, append([]string{"config", "rollback"}, test.args...))
			if err == nil || err.Error() != "Configuration rollback is unavailable" {
				t.Fatalf("error = %v", err)
			}
			for _, want := range []string{
				"Configuration rollback is unavailable",
				"No valid recovery source is available for the current configuration.",
				"The current configuration remains active and unchanged.",
				"aigw check --depth local",
			} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("output missing %q:\n%s", want, out.String())
				}
			}
			if count := strings.Count(out.String(), "aigw check --depth local"); count != 1 {
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
	app, out, _, _, _ := testApp(t, "")
	before := domain.NewConfig(clients.Admission())
	addProviderModel(&before, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	before.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-one"), "")
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Models = map[domain.ModelRef]domain.Model{domain.NewModelRef("one", "claude-two"): qualifiedModel("Two", "one", "claude-two", domain.ProtocolAnthropic)}
	after.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-two"), "")
	if err := app.Config.Save(after); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "rollback", "--last-change"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil || got.SelectedModel(claude.ID) != domain.NewModelRef("one", "claude-one") || !strings.Contains(out.String(), "Previous configuration") {
		t.Fatalf("config=%#v output=%q error=%v", got, out.String(), err)
	}
}

func TestRollbackUsesPreviousConfigurationWhenVerifiedRecoveryIsInvalid(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	previous := domain.NewConfig(clients.Admission())
	addProviderModel(&previous, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-stable")
	previous.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-stable"), "")
	if err := app.Config.Save(previous); err != nil {
		t.Fatal(err)
	}
	current := previous
	current.Models = map[domain.ModelRef]domain.Model{domain.NewModelRef("one", "claude-current"): qualifiedModel("Current", "one", "claude-current", domain.ProtocolAnthropic)}
	current.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-current"), "")
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verifiedPath(app.Config.Path()), []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"config", "rollback"}); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.SelectedModel(claude.ID) != domain.NewModelRef("one", "claude-stable") {
		t.Fatalf("rollback selection = %q, want stable", restored.SelectedModel(claude.ID))
	}
	if !strings.Contains(out.String(), "Previous configuration") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRollbackReportsUnconfirmedConfigurationWhenRestoreFails(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	target := t.TempDir()
	verified := domain.NewConfig(clients.Admission())
	stable := addProviderModel(&verified, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt-stable")
	verified.SetSelectedModel(codex.ID, stable, "")
	verified.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), verified, sessions(codex.ID)); err != nil {
		t.Fatal(err)
	}
	current := verified.Clone()
	current.Models[stable] = qualifiedModel("Stable", "one", "gpt-stable", domain.ProtocolResponses)
	current.Models[stable].Protocols[domain.ProtocolResponses] = []domain.Capability{domain.CapabilityText}
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"config", "rollback"})
	if err == nil || err.Error() != "Configuration rollback did not complete" {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{
		"Configuration rollback did not complete",
		"One rollback step failed; the cause identifies the affected boundary.",
		"The configuration or client files may have changed; inspect them before you run it again.",
		"aigw check --depth local",
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

func TestRollbackRestoresVerifiedCheckpointBeforeLastChangeBackup(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	verified := domain.NewConfig(clients.Admission())
	verified.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: domain.Endpoints{Responses: "https://example.test/v1"}}
	verified.Models[domain.NewModelRef("dmx", "gpt-stable")] = qualifiedModel("Stable", "dmx", "gpt-stable", domain.ProtocolResponses)
	verified.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-stable"), "")
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), verified, sessions(codex.ID)); err != nil {
		t.Fatal(err)
	}
	current := verified.Clone()
	current.Models = map[domain.ModelRef]domain.Model{domain.NewModelRef("dmx", "gpt-experimental"): qualifiedModel("Experimental", "dmx", "gpt-experimental", domain.ProtocolResponses)}
	current.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-experimental"), "")
	// An external edit leaves the verified snapshot available for recovery.
	isolated := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := isolated.Save(current); err != nil {
		t.Fatal(err)
	}
	writeFile(t, app.Config.Path(), readFile(t, isolated.Path()), 0o600)
	if _, err := app.Config.LoadVerifiedCheckpoint(); err != nil {
		t.Fatalf("the verified snapshot is not readable: %v", err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"config", "rollback"}); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.SelectedModel(codex.ID) != domain.NewModelRef("dmx", "gpt-stable") {
		t.Fatalf("rollback selection = %q, want stable", restored.SelectedModel(codex.ID))
	}
}
