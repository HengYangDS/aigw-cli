package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/surface"
)

func TestPortableInstallAndUninstallCommandsOwnOnlyProgramFiles(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
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
	if err := cli.Execute(app, []string{"install"}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(app.InstallTarget); err != nil || string(data) != "portable" {
		t.Fatalf("installed program = %q, %v", data, err)
	}
	if !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("install output = %q", out.String())
	}
	out.Reset()
	if err := cli.Execute(app, []string{"uninstall", "--target", app.InstallTarget}); err != nil {
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
	app, _, secretStore, _, _ := testApp(t, "")
	root := t.TempDir()
	app.Executable = filepath.Join(root, "bin", executableName("aigw"))
	previousExecutable := filepath.Join(filepath.Dir(app.Executable), executableName(".aigw.previous"))
	for _, path := range []string{app.Executable, previousExecutable} {
		writeFile(t, path, []byte("program"), 0o755)
	}

	codexTarget := filepath.Join(root, "codex", "config.toml")
	codexUserState := "approval_policy = \"on-request\"\n"
	writeFile(t, codexTarget, []byte(codexUserState), 0o600)
	writeFile(t, app.ClaudeSettingsPath, []byte("{\n  \"theme\": \"dark\"\n}\n"), 0o600)
	foreign := filepath.Join(root, "neighboring-user-state")
	writeFile(t, foreign, []byte("preserve"), 0o600)

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
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"uninstall"}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		app.Executable,
		previousExecutable,
		codexTarget + ".aigw-state.json",
		codexTarget + ".aigw-model-catalog.json",
		app.ClaudeSettingsPath + ".aigw-state.json",
		app.Config.Path() + ".verified.json",
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned uninstall residue remains at %s: %v", path, err)
		}
	}
	if data, err := os.ReadFile(codexTarget); err != nil || string(data) != codexUserState {
		t.Fatalf("Codex user state = %q, %v", data, err)
	}
	claudeData := readFile(t, app.ClaudeSettingsPath)
	var claudeSettings map[string]any
	if err := json.Unmarshal(claudeData, &claudeSettings); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(claudeSettings, map[string]any{"theme": "dark"}) {
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
