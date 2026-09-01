package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

// failingSecretsStore lets tests force a specific secret-store operation to
// fail without weakening any assertion about the resulting rollback.
type failingSecretsStore struct {
	getErr    error
	setErr    error
	deleteErr error
	has       bool
	deleted   []string
}

func (s *failingSecretsStore) Get(string) (string, error) { return "", s.getErr }
func (s *failingSecretsStore) Set(string, string) error   { return s.setErr }
func (s *failingSecretsStore) Delete(profile string) error {
	s.deleted = append(s.deleted, profile)
	return s.deleteErr
}
func (s *failingSecretsStore) Exists(string) (bool, error) { return s.has, nil }

func TestAddRejectsInvalidProfileName(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	err := execute(t, app, "add", "not valid!", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "Invalid service ID") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddSurfacesConfigLoadFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	// A config path that is itself an existing directory makes os.ReadFile
	// fail with something other than os.ErrNotExist.
	dir := t.TempDir()
	app.Config = configuration.NewStore(dir)
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin")
	if err == nil || strings.Contains(err.Error(), "Invalid service ID") {
		t.Fatalf("error = %v, want a config load failure", err)
	}
}

func TestAddRejectsDuplicateProfile(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	if err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddWithoutLabelDefaultsToProfileName(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	if err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin"); err != nil {
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
	app, _, _, _ := testApp(t, "token\n")
	// Omitting both endpoint flags cannot satisfy the selected client's
	// endpoint contract.
	err := execute(t, app, "add", "dmx", "--for", "claude", "--model", "claude-test", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "has no Anthropic endpoint") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddSurfacesSecretStoreSetFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	want := errors.New("keychain unavailable")
	app.Secrets = &failingSecretsStore{setErr: want}
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin")
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

func TestAddRollsBackSecretWhenConfigSaveFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "token\n")
	// A regular file where the config directory should be makes
	// os.MkdirAll fail inside Store.Save.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Config = configuration.NewStore(filepath.Join(blocker, "nested", "configuration.toml"))
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test", "--token-stdin")
	if err == nil {
		t.Fatal("expected a config save failure")
	}
	if secretExists(t, secretStore, "dmx") {
		t.Fatal("a failed config save must roll back the newly stored secret")
	}
}

func TestAddWithTokenStdinCreatesProfileWithoutPrintingSecret(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "top-secret\n")
	err := execute(t, app, "add", "dmx", "--label", "DMXAPI", "--openai-url", "https://example.test/v1", "--anthropic-url", "https://example.test", "--for", "codex", "--model", "gpt-test", "--token-stdin")
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
	app, _, _, _ := testApp(t, "top-secret\n")
	err := execute(t, app, "add", "dmx", "--label", "DMX", "--anthropic-url", "https://example.test", "--for", "claude", "--model", "claude-test")
	if err == nil || !strings.Contains(err.Error(), "--token-stdin") {
		t.Fatalf("error = %v", err)
	}
}
