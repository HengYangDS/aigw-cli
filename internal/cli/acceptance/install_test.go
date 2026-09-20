package cli_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

func TestInstallationDescribesCurrentFilesWithoutConfiguration(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	root := t.TempDir()
	app.Executable = filepath.Join(root, executableName("aigw"))
	app.Version = "1.2.3"
	program := []byte("current portable program")
	writeFile(t, app.Executable, program, 0o755)
	writeFile(t, app.Config.Path(), []byte("malformed configuration"), 0o600)
	if err := cli.Execute(app, []string{"installation", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		Version       string `json:"version"`
		CommandPath   string `json:"command_path"`
		Payload       struct {
			Path      string `json:"path"`
			SHA256    string `json:"sha256"`
			SizeBytes int64  `json:"size_bytes"`
		} `json:"payload"`
		Rollback json.RawMessage `json:"rollback"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(app.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Version != app.Version || result.CommandPath != app.Executable ||
		result.Payload.Path != resolved || result.Payload.SHA256 != fmt.Sprintf("%x", sha256.Sum256(program)) ||
		result.Payload.SizeBytes != int64(len(program)) || string(result.Rollback) != "null" {
		t.Fatalf("installation description = %+v", result)
	}
	if data, err := os.ReadFile(app.Config.Path()); err != nil || string(data) != "malformed configuration" {
		t.Fatalf("installation observation changed configuration: %q, %v", data, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("installation observation added state: %v, %v", entries, err)
	}
}

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
	for _, manager := range []string{"portable", "homebrew"} {
		t.Run(manager, func(t *testing.T) { verifyUninstallOwnership(t, manager) })
	}
}

func verifyUninstallOwnership(t *testing.T, manager string) {
	t.Helper()
	app, _, secretStore, _, _ := testApp(t, "")
	root := t.TempDir()
	app.Executable = filepath.Join(root, "bin", executableName("aigw"))
	if manager == "homebrew" {
		packageRoot := filepath.Join(root, "Caskroom", "aigw")
		app.Executable = filepath.Join(packageRoot, "0.1.0", "bin", executableName("aigw"))
		writeFile(t, filepath.Join(packageRoot, ".metadata", "INSTALL_RECEIPT.json"), []byte(`{"homebrew_version":"7.0.2","source":{"tap":"owner/tap"}}`), 0o600)
	}
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

	configureUninstallClients(t, app, codexTarget)

	credentialCalls := &recordingCredentialStore[string]{backend: secretStore}
	app.Secrets = credentialCalls
	withdrawInstallationClients(t, app, manager, previousExecutable)

	if len(credentialCalls.getCalls)+len(credentialCalls.setCalls)+len(credentialCalls.deleteCalls) != 0 {
		t.Fatal("client withdrawal accessed credential values")
	}

	removedPaths := []string{
		codexTarget + ".aigw-state.json",
		codexTarget + ".aigw-model-catalog.json",
		app.ClaudeSettingsPath + ".aigw-state.json",
		app.Config.Path() + ".verified.json",
	}
	if manager == "portable" {
		removedPaths = append(removedPaths, app.Executable, previousExecutable)
	}
	for _, path := range removedPaths {
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
	expectedBindings := 0
	if manager == "homebrew" {
		expectedBindings = 2
	}
	if len(retained.Adapters) != expectedBindings || len(retained.EnabledClientIDs()) != 0 || retained.Routes[configuration.ClientClaude] != "claude" || retained.Routes[configuration.ClientCodex] != "codex" || len(retained.Accounts) != 1 || len(retained.Profiles) != 2 {
		t.Fatalf("retained capability configuration = %#v", retained)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "token" {
		t.Fatalf("retained credential = %q, %v", token, err)
	}
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "preserve" {
		t.Fatalf("neighboring user state = %q, %v", data, err)
	}
	previous, err := app.Config.LoadBackup()
	expectedAdapters := 2
	if err != nil || len(previous.Adapters) != expectedAdapters {
		t.Fatalf("previous configuration = %#v, %v", previous, err)
	}
}

func configureUninstallClients(t *testing.T, app *cli.App, codexTarget string) {
	t.Helper()
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
	if err := app.Secrets.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
}

func withdrawInstallationClients(t *testing.T, app *cli.App, manager, previousExecutable string) {
	t.Helper()
	if manager == "portable" {
		if err := cli.Execute(app, []string{"uninstall"}); err != nil {
			t.Fatal(err)
		}
		return
	}

	if err := cli.Execute(app, []string{"uninstall"}); err == nil || !strings.Contains(err.Error(), "Homebrew") {
		t.Fatalf("managed uninstall = %v", err)
	}
	for _, client := range configuration.AdmittedClientIDs() {
		if err := cli.Execute(app, []string{"adapter", "disable", client}); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"adapter", "disable", client}); err != nil {
			t.Fatalf("repeat disable: %v", err)
		}
	}
	for _, path := range []string{app.Executable, previousExecutable} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "program" {
			t.Fatalf("package-owned file changed: %q, %v", data, err)
		}
	}
}
