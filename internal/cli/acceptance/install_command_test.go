package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/surface"
)

func TestPortableInstallAndUninstallCommandsOwnOnlyProgramFiles(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Executable = filepath.Join(t.TempDir(), "download", "aigw")
	app.InstallTarget = filepath.Join(t.TempDir(), "bin", "aigw")
	if runtime.GOOS == "windows" {
		app.Executable += ".exe"
		app.InstallTarget += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(app.Executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.Executable, []byte("portable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "install"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(app.InstallTarget); err != nil || string(data) != "portable" {
		t.Fatalf("installed program = %q, %v", data, err)
	}
	if !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("install output = %q", out.String())
	}
	out.Reset()
	if err := execute(t, app, "uninstall", "--target", app.InstallTarget); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(app.InstallTarget); !os.IsNotExist(err) {
		t.Fatalf("installed program remains: %v", err)
	}
	if !strings.Contains(out.String(), "Configuration and credential-store secrets were preserved") {
		t.Fatalf("uninstall output = %q", out.String())
	}
}

func TestUninstallWithdrawsOwnedClientStateAndPreservesCapabilities(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	root := t.TempDir()
	app.Executable = filepath.Join(root, "bin", "aigw")
	if runtime.GOOS == "windows" {
		app.Executable += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(app.Executable), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{app.Executable, filepath.Join(filepath.Dir(app.Executable), ".aigw.previous")} {
		if runtime.GOOS == "windows" && strings.HasSuffix(path, ".previous") {
			path += ".exe"
		}
		if err := os.WriteFile(path, []byte("program"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	codexTarget := filepath.Join(root, "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	codexUserState := "approval_policy = \"on-request\"\n"
	if err := os.WriteFile(codexTarget, []byte(codexUserState), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(app.ClaudeSettingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.ClaudeSettingsPath, []byte("{\n  \"theme\": \"dark\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, "neighboring-user-state")
	if err := os.WriteFile(foreign, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}

	claudeExecutable := executableFixture(t, "claude")
	codexExecutable := executableFixture(t, "codex")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			configuration.ClientClaude: claudeExecutable,
			configuration.ClientCodex:  codexExecutable,
		},
		Surfaces: []discovery.Surface{{
			ID:          string(surface.CodexHomeDefault),
			Authority:   string(surface.AuthorityAIGW),
			ConfigPath:  codexTarget,
			AutoManaged: true,
		}},
	}}
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Model: "claude-model"}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "team", Client: configuration.ClientCodex, Model: "gpt-model"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: codexExecutable, Targets: []string{codexTarget}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(cfg, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "uninstall"); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		app.Executable,
		filepath.Join(filepath.Dir(app.Executable), ".aigw.previous"),
		codexTarget + ".aigw-state.json",
		codexTarget + ".aigw-model-catalog.json",
		app.ClaudeSettingsPath + ".aigw-state.json",
		app.Config.Path() + ".verified.json",
	} {
		if runtime.GOOS == "windows" && strings.HasSuffix(path, ".previous") {
			path += ".exe"
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned uninstall residue remains at %s: %v", path, err)
		}
	}
	if data, err := os.ReadFile(codexTarget); err != nil || string(data) != codexUserState {
		t.Fatalf("Codex user state = %q, %v", data, err)
	}
	claudeData, err := os.ReadFile(app.ClaudeSettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var claudeSettings map[string]any
	if err := json.Unmarshal(claudeData, &claudeSettings); err != nil {
		t.Fatal(err)
	}
	if claudeSettings["theme"] != "dark" || claudeSettings["apiKeyHelper"] != nil || claudeSettings["env"] != nil || claudeSettings["model"] != nil {
		t.Fatalf("Claude user state = %#v", claudeSettings)
	}
	retained, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained.Adapters) != 0 || retained.Routes[configuration.ClientClaude] != "claude" || retained.Routes[configuration.ClientCodex] != "codex" || len(retained.Accounts) != 1 || len(retained.Profiles) != 2 {
		t.Fatalf("retained capability configuration = %#v", retained)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "token" {
		t.Fatalf("retained credential = %q, %v", token, err)
	}
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "preserve" {
		t.Fatalf("neighboring user state = %q, %v", data, err)
	}
	previous, err := app.Config.LoadBackup()
	if err != nil || len(previous.Adapters) != 2 {
		t.Fatalf("previous configuration = %#v, %v", previous, err)
	}
}
