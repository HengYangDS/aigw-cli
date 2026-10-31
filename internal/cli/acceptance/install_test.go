package cli_test

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/program"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/surface"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
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

func TestUninstallReportsCommittedWithdrawalWithIncompleteProgramRemoval(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	codexTarget := filepath.Join(t.TempDir(), "config.toml")
	configureUninstallClients(t, app, codexTarget)
	blockRemoval(t, app.Executable)
	out.Reset()
	if err := cli.Execute(app, []string{"uninstall"}); err == nil {
		t.Fatal("incomplete program removal was accepted")
	}
	cfg, err := app.Config.Load()
	if err != nil || len(cfg.EnabledClientIDs()) != 0 {
		t.Fatalf("committed client withdrawal was lost: enabled=%v error=%v", cfg.EnabledClientIDs(), err)
	}
	if data, err := os.ReadFile(app.Executable); err != nil || string(data) != "program" {
		t.Fatalf("the program that could not be removed changed: %q error=%v", data, err)
	}
	if !strings.Contains(out.String(), "Client withdrawal completed") || !strings.Contains(out.String(), "program removal is incomplete") || strings.Contains(out.String(), app.Executable) {
		t.Fatalf("uninstall obscured the committed state or exposed its path: %s", out.String())
	}
	if !strings.Contains(out.String(), "another verified AIGW executable") {
		t.Fatalf("partial uninstall requires its removed command: %s", out.String())
	}
}

func TestUnconfiguredUninstallReportsRemovalFailureWithoutClaimingWithdrawal(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	writeFile(t, app.Executable, []byte("program"), 0o700)
	blockRemoval(t, app.Executable)
	if err := cli.Execute(app, []string{"uninstall"}); err == nil {
		t.Fatal("incomplete program removal was accepted")
	}
	if !strings.Contains(out.String(), "No AIGW configuration existed") || strings.Contains(out.String(), "withdrawal completed") || strings.Contains(out.String(), app.Executable) {
		t.Fatalf("unconfigured uninstall claims a nonexistent withdrawal: %s", out.String())
	}
}

// TestUninstallRefusesARollbackDirectoryBeforeWithdrawal shows that a
// directory at the rollback copy's path, which AIGW did not write, stops the
// preview and the uninstall alike before any client is withdrawn.
func TestUninstallRefusesARollbackDirectoryBeforeWithdrawal(t *testing.T) {
	for _, args := range [][]string{{"uninstall", "--dry-run"}, {"uninstall"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			writeFile(t, app.Executable, []byte("program"), 0o700)
			configureUninstallClients(t, app, filepath.Join(t.TempDir(), "config.toml"))
			foreign := filepath.Join(program.RollbackPath(app.Executable), "foreign-child")
			writeFile(t, foreign, []byte("preserve"), 0o600)
			out.Reset()
			if err := cli.Execute(app, args); err == nil || !strings.Contains(err.Error(), "rollback copy path") {
				t.Fatalf("rollback directory = %v\n%s", err, out.String())
			}
			cfg, err := app.Config.Load()
			if err != nil || len(cfg.EnabledClientIDs()) != 2 {
				t.Fatalf("clients were withdrawn before the refusal: enabled=%v error=%v", cfg.EnabledClientIDs(), err)
			}
			for path, want := range map[string]string{app.Executable: "program", foreign: "preserve"} {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Fatalf("%s = %q, %v; want %q", path, data, err, want)
				}
			}
		})
	}
}

func TestUninstallPreservesDriftedSharedReader(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	root := t.TempDir()
	app.Executable = filepath.Join(root, "bin", executableName("aigw"))
	app.DataDir = filepath.Join(root, "data")
	writeFile(t, app.Executable, []byte("portable program"), 0o755)
	reader, err := secret.VersionedEntrypointPath(app.DataDir, app.Executable, executableName("aigw"))
	if err != nil {
		t.Fatal(err)
	}
	app.CredentialPath = reader
	codexTarget := filepath.Join(root, "codex", "config.toml")
	writeFile(t, codexTarget, []byte("approval_policy = \"on-request\"\n"), 0o600)
	configureUninstallClients(t, app, codexTarget)
	writeFile(t, reader+".sha256", []byte("changed receipt\n"), 0o600)

	if err := cli.Execute(app, []string{"uninstall"}); err != nil {
		t.Fatal(err)
	}
	retained, loadErr := app.Config.Load()
	if loadErr != nil || len(retained.EnabledClientIDs()) != 0 {
		t.Fatalf("client withdrawal was not committed: %#v, %v", retained.Clients, loadErr)
	}
	if _, err := os.Stat(app.Executable); !os.IsNotExist(err) {
		t.Fatalf("uninstall retained its executable: %v", err)
	}
	if _, err := os.Stat(reader); err != nil {
		t.Fatalf("uninstall removed a possibly shared reader: %v", err)
	}
	if got := string(readFile(t, reader+".sha256")); got != "changed receipt\n" {
		t.Fatalf("uninstall changed a retained reader receipt: %q", got)
	}
	if token, getErr := secretStore.Get("team"); getErr != nil || token != "token" {
		t.Fatalf("uninstall changed Token: %q, %v", token, getErr)
	}
}

func TestPortableUninstallPreservesExternalCredentialCommand(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	root := t.TempDir()
	app.Executable = filepath.Join(root, "bin", executableName("aigw"))
	app.DataDir = filepath.Join(root, "data")
	writeFile(t, app.Executable, []byte("portable program"), 0o755)
	reader, err := secret.VersionedEntrypointPath(app.DataDir, app.Executable, executableName("aigw"))
	if err != nil {
		t.Fatal(err)
	}
	app.CredentialPath = reader
	codexTarget := filepath.Join(root, "codex", "config.toml")
	writeFile(t, codexTarget, []byte("approval_policy = \"on-request\"\n"), 0o600)
	configureUninstallClients(t, app, codexTarget)
	external := filepath.Join(root, "external-credential")
	writeFile(t, external, []byte("operator-owned helper"), 0o700)
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{claude.ID, codex.ID} {
		binding := cfg.Clients[client]
		binding.CredentialCommand = external
		cfg.Clients[client] = binding
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(reader); err != nil {
		t.Fatalf("external switch removed a potentially cached AIGW reader: %v", err)
	}
	if err := cli.Execute(app, []string{"uninstall"}); err != nil {
		t.Fatal(err)
	}
	if got := string(readFile(t, external)); got != "operator-owned helper" {
		t.Fatalf("uninstall changed external credential command: %q", got)
	}
	if _, err := os.Stat(app.Executable); !os.IsNotExist(err) {
		t.Fatalf("uninstall retained its executable: %v", err)
	}
	for _, path := range []string{reader, reader + ".sha256"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("uninstall removed a possibly cached reader %s: %v", path, err)
		}
	}
	if token, err := secretStore.Get("team"); err != nil || token != "token" {
		t.Fatalf("uninstall changed Token: %q, %v", token, err)
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
	writeFile(t, claudeSettingsPath(t, app.Host), []byte("{\n  \"theme\": \"dark\"\n}\n"), 0o600)
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
		claudeSettingsPath(t, app.Host) + ".aigw-state.json",
		app.Config.Path() + ".verified.json",
	}
	if manager == "portable" {
		removedPaths = append(removedPaths, app.Executable, previousExecutable)
	}
	requirePathsAbsent(t, removedPaths)
	if data, err := os.ReadFile(codexTarget); err != nil || string(data) != codexUserState {
		t.Fatalf("Codex user state = %q, %v", data, err)
	}
	claudeData := readFile(t, claudeSettingsPath(t, app.Host))
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
	if len(retained.Clients) != expectedBindings || len(retained.EnabledClientIDs()) != 0 || len(retained.Accounts) != 1 || len(retained.Routes) != 2 {
		t.Fatalf("retained capability configuration = %#v", retained)
	}
	if manager == "homebrew" && (retained.SelectedRoute(claude.ID) != "claude" || retained.SelectedRoute(codex.ID) != "codex") {
		t.Fatalf("package-managed uninstall changed retained client selections: %#v", retained.Clients)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "token" {
		t.Fatalf("retained credential = %q, %v", token, err)
	}
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "preserve" {
		t.Fatalf("neighboring user state = %q, %v", data, err)
	}
	previous, err := app.Config.LoadBackup()
	expectedAdapters := 2
	if err != nil || len(previous.Clients) != expectedAdapters {
		t.Fatalf("previous configuration = %#v, %v", previous, err)
	}
}

func requirePathsAbsent(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned uninstall residue remains at %s: %v", path, err)
		}
	}
}

func configureUninstallClients(t *testing.T, app *cli.App, codexTarget string) {
	t.Helper()
	claudeExecutable := executableFixture(t, "claude")
	codexExecutable := executableFixture(t, "codex")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			claude.ID: claudeExecutable,
			codex.ID:  codexExecutable,
		},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
			Authority:   string(surface.AuthorityAIGW),
			ConfigPath:  codexTarget,
			AutoManaged: true,
		}},
	}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
	cfg.Routes["claude"] = qualifiedRoute("Claude", "team", "claude-model", domain.ProtocolAnthropic)
	cfg.Routes["codex"] = qualifiedRoute("Codex", "team", "gpt-model", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	cfg.SetClientActivation(claude.ID, true, claudeExecutable, nil)
	cfg.SetClientActivation(codex.ID, true, codexExecutable, []string{codexTarget})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := app.Secrets.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, clients.Admission().IDs()); err != nil {
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
	for _, client := range clients.Admission().IDs() {
		if err := cli.Execute(app, []string{"client", "disable", client}); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"client", "disable", client}); err != nil {
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
