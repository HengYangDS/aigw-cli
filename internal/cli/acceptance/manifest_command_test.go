package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCommandIOFailures(t *testing.T) {
	t.Run("path output", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := execute(t, app, "config", "path"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("export load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "config", "export"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("export output", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := execute(t, app, "config", "export"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("import read", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "config", "import", filepath.Join(t.TempDir(), "missing.toml")); err == nil {
			t.Fatal("expected manifest read failure")
		}
	})

	t.Run("import parse", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		path := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "config", "import", path); err == nil {
			t.Fatal("expected manifest parse failure")
		}
	})
}

func TestConfigImportAndExportAreSecretFree(t *testing.T) {
	app, out, secrets, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 4
[recommended_routes]
claude = "team"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team Gateway"
purpose = "Default agent"
account = "team"
client = "claude"
model = "claude-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "config", "import", manifestPath); err != nil {
		t.Fatal(err)
	}
	if secrets.Has("team") {
		t.Fatal("manifest import invented a secret")
	}
	out.Reset()
	if err := execute(t, app, "config", "export"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out.String()), "token") || !strings.Contains(out.String(), "Team Gateway") || !strings.Contains(out.String(), "Default agent") {
		t.Fatalf("unsafe export:\n%s", out.String())
	}
}

func TestConfigUpgradeIsNotAnAvailableCommand(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	err := execute(t, app, "config", "upgrade")
	if err == nil || !strings.Contains(err.Error(), "Unknown config subcommand") {
		t.Fatalf("config upgrade error = %v", err)
	}
}

func TestConfigDoesNotExposeRemovedLegacyMigration(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	err := execute(t, app, "config", "migrate", filepath.Join(t.TempDir(), "configuration.json"))
	if err == nil || !strings.Contains(err.Error(), "Unknown config subcommand") {
		t.Fatalf("legacy migration command error = %v, want Unknown config subcommand", err)
	}
}

func TestCodexSyncReconcilesEachConfiguredHomeWithoutLoggingIn(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	dir := t.TempDir()
	targets := []string{filepath.Join(dir, "one", "configuration.toml"), filepath.Join(dir, "two", "configuration.toml")}
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}, configuration.ClientCodex, "team-model")
	cfg.Routes[configuration.ClientCodex] = "team"
	cfg.Adapters["codex"] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex-real", Targets: targets}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("team", "secret")
	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync must not start credential binding plans: %#v", runner.plans)
	}
	for _, target := range targets {
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "AIGW managed provider") {
			t.Fatalf("sync did not reconcile %s:\n%s", target, data)
		}
	}
}

func TestUpdateCandidateRequiresChecksumManifest(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Updater = &fakeUpdater{}
	err := execute(t, app, "update", "--candidate", "/tmp/aigw_0.2.0_darwin_arm64.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "must all be set") {
		t.Fatalf("error = %v", err)
	}
}

func TestRepairPreservesConfiguredClaudeExecutable(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/different/claude"}}}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Adapters[configuration.ClientClaude].Executable; got != claudeExecutable {
		t.Fatalf("repair replaced configured Claude executable: %q", got)
	}
	if !strings.Contains(out.String(), "Unchanged") {
		t.Fatalf("repair incorrectly claimed authentication refresh:\n%s", out.String())
	}
}

func TestTerminalErrorLocalizesUnsupportedConfigVersion(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := os.WriteFile(app.Config.Path(), []byte("version = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "status")
	if err == nil {
		t.Fatal("status unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"unsupported configuration version: found 0, expected 3", "Recommended action", "aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized configuration error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unsupported config version") {
		t.Fatalf("terminal leaked raw configuration error:\n%s", text)
	}
}

func TestUnconfiguredCommandsPointToSetupWithoutLoops(t *testing.T) {
	for _, command := range [][]string{{"status"}, {"check"}, {"repair"}, {"models"}, {"catalog"}} {
		app, out, _, _ := testApp(t, "")
		err := execute(t, app, command...)
		if command[0] == "status" && err != nil {
			t.Fatalf("%v error = %v", command, err)
		}
		if command[0] != "status" && err == nil {
			t.Fatalf("%v succeeded without configuration", command)
		}
		text := out.String() + "\n"
		if err != nil {
			text += err.Error()
		}
		if !strings.Contains(text, "aigw setup") {
			t.Fatalf("%v should point to setup:\n%s", command, text)
		}
		if strings.Contains(text, "run `aigw`") || strings.Contains(text, "aigw repair") || strings.Contains(text, "aigw check") {
			t.Fatalf("%v retained a loop or ambiguous first-use action:\n%s", command, text)
		}
	}
}

func TestStatusAndCheckHideUnreadableConfigDetails(t *testing.T) {
	for _, command := range [][]string{{"status"}, {"check"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			app, out, _, _ := testApp(t, "")
			if err := os.WriteFile(app.Config.Path(), []byte("version = [\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			err := execute(t, app, command...)
			if err == nil {
				t.Fatalf("%s unexpectedly succeeded", strings.Join(command, " "))
			}
			text := out.String()
			for _, want := range []string{"Cannot read or validate local configuration", "aigw doctor"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s output lacks %q:\n%s", strings.Join(command, " "), want, text)
				}
			}
			for _, forbidden := range []string{"parse config:", "validate config:", "version = [", app.Config.Path()} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s leaked %q:\n%s", strings.Join(command, " "), forbidden, text)
				}
			}
		})
	}
}
