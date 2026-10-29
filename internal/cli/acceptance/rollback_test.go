package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
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
			app, out, _, _, _ := testApp(t, "")
			saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
			if test.prepare != nil {
				test.prepare(t, app.Config.Path())
			}
			before, readErr := os.ReadFile(app.Config.Path())
			if readErr != nil {
				t.Fatal(readErr)
			}
			err := cli.Execute(app, append([]string{"rollback"}, test.args...))
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
	app, out, _, _, _ := testApp(t, "")
	before := domain.NewConfig(clients.Admission())
	addAccountRoute(&before, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	before.SetSelectedRoute(claude.ID, "one", "")
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Routes = map[string]domain.Route{"two": qualifiedRoute("Two", "one", "claude-two", domain.ProtocolAnthropic)}
	after.SetSelectedRoute(claude.ID, "two", "")
	if err := app.Config.Save(after); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"rollback", "--last-change"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil || got.SelectedRoute(claude.ID) != "one" || !strings.Contains(out.String(), "Previous configuration") {
		t.Fatalf("config=%#v output=%q error=%v", got, out.String(), err)
	}
}

func TestRollbackUsesPreviousConfigurationWhenVerifiedRecoveryIsInvalid(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	previous := domain.NewConfig(clients.Admission())
	addAccountRoute(&previous, "stable", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-stable")
	previous.SetSelectedRoute(claude.ID, "stable", "")
	if err := app.Config.Save(previous); err != nil {
		t.Fatal(err)
	}
	current := previous
	current.Routes = map[string]domain.Route{"current": qualifiedRoute("Current", "one", "claude-current", domain.ProtocolAnthropic)}
	current.SetSelectedRoute(claude.ID, "current", "")
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.Config.Path()+".verified.json", []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"rollback"}); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.SelectedRoute(claude.ID) != "stable" {
		t.Fatalf("rollback selection = %q, want stable", restored.SelectedRoute(claude.ID))
	}
	if !strings.Contains(out.String(), "Previous configuration") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRollbackReportsUnconfirmedConfigurationWhenRestoreFails(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	target := t.TempDir()
	verified := domain.NewConfig(clients.Admission())
	addAccountRoute(&verified, "stable", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt-stable")
	verified.SetSelectedRoute(codex.ID, "stable", "")
	verified.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), verified, []string{codex.ID}); err != nil {
		t.Fatal(err)
	}
	current := verified.Clone()
	current.Routes["stable"] = qualifiedRoute("Stable", "one", "gpt-current", domain.ProtocolOpenAIResponses)
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"rollback"})
	if err == nil || err.Error() != "Configuration rollback did not complete" {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{
		"Configuration rollback did not complete",
		"One rollback step failed; the cause identifies the affected boundary.",
		"Configuration or client projections may have changed; inspect current state before retrying.",
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

func TestRollbackRestoresVerifiedCheckpointBeforeLastChangeBackup(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	verified := domain.NewConfig(clients.Admission())
	verified.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	verified.Routes["stable"] = qualifiedRoute("Stable", "dmx", "gpt-stable", domain.ProtocolOpenAIResponses)
	verified.SetSelectedRoute(codex.ID, "stable", "")
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), verified, []string{codex.ID}); err != nil {
		t.Fatal(err)
	}
	current := verified.Clone()
	account := current.Accounts["dmx"]
	account.Order = 10
	current.Accounts["dmx"] = account
	current.Routes = map[string]domain.Route{"experimental": qualifiedRoute("Experimental", "dmx", "gpt-experimental", domain.ProtocolOpenAIResponses)}
	current.SetSelectedRoute(codex.ID, "experimental", "")
	// External edits leave the historical checkpoint available for recovery.
	canonical := current.Clone()
	account.Order = 0
	canonical.Accounts["dmx"] = account
	isolated := store.NewStore(filepath.Join(t.TempDir(), "config.toml"), clients.Admission())
	if err := isolated.Save(canonical); err != nil {
		t.Fatal(err)
	}
	writeFile(t, app.Config.Path(), readFile(t, isolated.Path()), 0o600)
	writeFile(t, strings.TrimSuffix(app.Config.Path(), filepath.Ext(app.Config.Path()))+".catalog.toml", []byte("[accounts]\ndmx = 10\n"), 0o600)
	checkpoint, err := app.Config.LoadVerifiedCheckpoint()
	if err != nil || checkpoint.Config.Accounts["dmx"].Order != 10 {
		t.Fatalf("checkpoint did not restore current presentation: error=%v", err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"rollback"}); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.SelectedRoute(codex.ID) != "stable" {
		t.Fatalf("rollback selection = %q, want stable", restored.SelectedRoute(codex.ID))
	}
	if restored.Accounts["dmx"].Order != 10 {
		t.Fatal("verified rollback discarded current Account presentation")
	}
}
