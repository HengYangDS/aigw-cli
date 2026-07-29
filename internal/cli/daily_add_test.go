package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
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
func (s *failingSecretsStore) Has(string) bool { return s.has }

func TestAddRejectsInvalidProfileName(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	err := execute(t, app, "add", "not valid!", "--anthropic-url", "https://example.test", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "Invalid service ID") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddSurfacesConfigLoadFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	// A config path that is itself an existing directory makes os.ReadFile
	// fail with something other than os.ErrNotExist.
	dir := t.TempDir()
	app.Config = config.NewStore(dir)
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--token-stdin")
	if err == nil || strings.Contains(err.Error(), "Invalid service ID") {
		t.Fatalf("error = %v, want a config load failure", err)
	}
}

func TestAddRejectsDuplicateProfile(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	if err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddWithoutLabelDefaultsToProfileName(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	if err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--token-stdin"); err != nil {
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
	// Omitting both endpoint flags produces an account with no endpoint at
	// all, which domain.Config.Validate rejects.
	err := execute(t, app, "add", "dmx", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "must define at least one endpoint") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddSurfacesSecretStoreSetFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	want := errors.New("keychain unavailable")
	app.Secrets = &failingSecretsStore{setErr: want}
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--token-stdin")
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
	app.Config = config.NewStore(filepath.Join(blocker, "nested", "config.toml"))
	err := execute(t, app, "add", "dmx", "--anthropic-url", "https://example.test", "--token-stdin")
	if err == nil {
		t.Fatal("expected a config save failure")
	}
	if secretStore.Has("dmx") {
		t.Fatal("a failed config save must roll back the newly stored secret")
	}
}
