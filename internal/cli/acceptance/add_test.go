package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

func TestAddProjectsOnlyItsSelectedClient(t *testing.T) {
	for _, clientID := range configuration.AdmittedClientIDs() {
		t.Run(clientID, func(t *testing.T) {
			app, _, credentials, _, _ := testApp(t, "new-token\n")
			target := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			cfg.Accounts["old"] = configuration.Account{Label: "Old", Endpoints: configuration.Endpoints{Anthropic: "https://old.test", OpenAIResponses: "https://old.test/v1"}}
			paths := map[string]string{configuration.ClientCodex: target, configuration.ClientClaude: app.ClaudeSettingsPath}
			for _, id := range configuration.AdmittedClientIDs() {
				cfg.Profiles[id] = configuration.Profile{Label: id, Account: "old", Client: id, Model: "old-model"}
				cfg.Routes[id] = id
				cfg.Adapters[id] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, id)}
			}
			adapter := cfg.Adapters[configuration.ClientCodex]
			adapter.Targets = []string{target}
			cfg.Adapters[configuration.ClientCodex] = adapter
			if err := credentials.Set("old", "old-token"); err != nil {
				t.Fatal(err)
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, []string{"sync"}); err != nil {
				t.Fatal(err)
			}
			other := configuration.ClientClaude
			if clientID == other {
				other = configuration.ClientCodex
			}
			foreign := []byte("external unfinished edit\n")
			if err := os.WriteFile(paths[other], foreign, 0o600); err != nil {
				t.Fatal(err)
			}
			state := readFile(t, paths[other]+".aigw-state.json")
			if err := cli.Execute(app, []string{"add", "new", "--for", clientID, "--model", "new-model", "--anthropic-url", "https://new.test", "--openai-url", "https://new.test/v1", "--token-stdin"}); err != nil {
				t.Fatal(err)
			}
			current, err := app.Config.Load()
			if err != nil || current.Routes[clientID] != "new" || current.Routes[other] != other {
				t.Fatalf("creation routes = %v, %v", current.Routes, err)
			}
			if !bytes.Contains(readFile(t, paths[clientID]), []byte("new-model")) {
				t.Fatal("service creation left the selected client on its old model")
			}
			if !bytes.Equal(readFile(t, paths[other]), foreign) || !bytes.Equal(readFile(t, paths[other]+".aigw-state.json"), state) {
				t.Fatal("service creation touched the unrelated client")
			}
		})
	}
}

func TestAddProjectionConflictRestoresConfigurationAndToken(t *testing.T) {
	app, _, credentials, _, _ := testApp(t, "new-token\n")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: executableFixture(t, "claude")}}}
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://old.test"}, configuration.ClientClaude, "old-model")
	if err := credentials.Set("one", "old-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	foreign := []byte(`{"apiKeyHelper":"user-owned-helper"}`)
	if err := os.WriteFile(app.ClaudeSettingsPath, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"add", "new", "--for", "claude", "--model", "new-model", "--anthropic-url", "https://new.test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("projection conflict = %v", err)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
		t.Fatalf("failed creation changed configuration: %v", err)
	}
	if secretExists(t, credentials, "new") || !bytes.Equal(readFile(t, app.ClaudeSettingsPath), foreign) {
		t.Fatal("failed creation retained its Token or overwrote client edits")
	}
}

func TestAddPreservesExistingAccountWithDifferentProfileID(t *testing.T) {
	app, _, credentials, _, _ := testApp(t, "new-token\n")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "existing-profile", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "old-model")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Set("team", "existing-token"); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, app.Config.Path())
	err := cli.Execute(app, []string{"add", "team", "--for", "claude", "--model", "new-model", "--anthropic-url", "https://new.test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Account collision = %v", err)
	}
	token, err := credentials.Get("team")
	if err != nil || token != "existing-token" || !bytes.Equal(readFile(t, app.Config.Path()), before) {
		t.Fatalf("creation overwrote existing Account or Token: %v", err)
	}
}

func TestAddRejectsInvalidProfileName(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	err := cli.Execute(app, []string{"add", "not valid!", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "Invalid service ID") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddSurfacesConfigLoadFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	// A config path that is itself an existing directory makes os.ReadFile
	// fail with something other than os.ErrNotExist.
	dir := t.TempDir()
	app.Config = configuration.NewStore(dir)
	err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"})
	if err == nil || strings.Contains(err.Error(), "Invalid service ID") {
		t.Fatalf("error = %v, want a config load failure", err)
	}
}

func TestAddRejectsDuplicateProfile(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	if err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"}); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddWithoutLabelDefaultsToProfileName(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	if err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles["dmx"].Label != "dmx" || cfg.Accounts["dmx"].Label != "dmx" {
		t.Fatalf("profile/account label = %#v, want the profile name as a default", cfg.Profiles["dmx"])
	}
}

func TestAddRejectsConfigThatFailsValidation(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	// Omitting both endpoint flags cannot satisfy the selected client's
	// endpoint contract.
	err := cli.Execute(app, []string{"add", "dmx", "--for", "claude", "--model", "claude-test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "has no Anthropic endpoint") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddSurfacesSecretStoreSetFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
	want := errors.New("keychain unavailable")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), setErr: want}
	err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	cfg, loadErr := app.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, exists := cfg.Profiles["dmx"]; exists {
		t.Fatal("a failed secret write must not leave a persisted profile")
	}
}

func TestAddRejectsUnavailableLockBeforeCredentialAccess(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "token\n")
	recorder := &recordingCredentialStore[string]{backend: secretStore}
	app.Secrets = recorder
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Config = configuration.NewStore(filepath.Join(blocker, "nested", "configuration.toml"))
	err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "create config directory for lock") {
		t.Fatalf("lock admission error = %v", err)
	}
	if len(recorder.setCalls)+len(recorder.deleteCalls)+len(recorder.getCalls)+len(recorder.existsCalls) != 0 {
		t.Fatal("lock failure touched credentials")
	}
}

func TestAddCompensatesCredentialAfterConfigurationFailure(t *testing.T) {
	compensationFailure := errors.New("credential compensation unavailable")
	for _, failure := range []error{nil, compensationFailure} {
		name := "compensated"
		if failure != nil {
			name = "compensation failure"
		}
		t.Run(name, func(t *testing.T) {
			app, _, backend, _, _ := testApp(t, "token\n")
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://current.test"}, configuration.ClientClaude, "current")
			before := readFile(t, app.Config.Path())
			if err := os.Mkdir(app.Config.Path()+".bak", 0o700); err != nil {
				t.Fatal(err)
			}
			recorder := &recordingCredentialStore[string]{backend: backend, deleteErr: failure}
			app.Secrets = recorder
			err := cli.Execute(app, []string{"add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"})
			if err == nil || !strings.Contains(err.Error(), ".bak") {
				t.Fatalf("configuration snapshot failure = %v", err)
			}
			if len(recorder.setCalls) != 1 || recorder.setCalls[0] != "dmx" || len(recorder.deleteCalls) != 1 || recorder.deleteCalls[0] != "dmx" {
				t.Fatalf("credential transition not exercised: set=%v delete=%v", recorder.setCalls, recorder.deleteCalls)
			}
			if failure != nil && !errors.Is(err, failure) {
				t.Fatalf("compensation failure was lost: %v", err)
			}
			if present := secretExists(t, backend, "dmx"); present != (failure != nil) {
				t.Fatalf("credential presence = %v, compensation failure = %v", present, failure)
			}
			if after := readFile(t, app.Config.Path()); string(after) != string(before) {
				t.Fatal("failed creation changed configuration")
			}
		})
	}
}

func TestAddWithTokenStdinCreatesProfileWithoutPrintingSecret(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "top-secret\n")
	err := cli.Execute(app, []string{"add", "dmx", "--label", "DMXAPI", "--openai-url", "https://example.test/v1", "--anthropic-url", "https://example.test", "--for", "codex", "--model", "gpt-test", "--token-stdin"})
	if err != nil {
		t.Fatal(err)
	}
	if !secretExists(t, secretStore, "dmx") {
		t.Fatal("secret not stored")
	}
	if strings.Contains(out.String(), "top-secret") {
		t.Fatalf("secret leaked in output: %s", out.String())
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.Routes[configuration.ClientCodex] != "dmx" || cfg.Profiles["dmx"].Label != "DMXAPI" {
		t.Fatalf("config = %#v, %v", cfg, err)
	}
}

func TestAddRefusesNonInteractiveImplicitTokenInput(t *testing.T) {
	app, _, _, _, _ := testApp(t, "top-secret\n")
	err := cli.Execute(app, []string{"add", "dmx", "--label", "DMX", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test"})
	if err == nil || !strings.Contains(err.Error(), "--token-stdin") {
		t.Fatalf("error = %v", err)
	}
}
