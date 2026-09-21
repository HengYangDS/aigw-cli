package manifest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

	"github.com/pelletier/go-toml/v2"
)

func TestMigrationPreviewAndApplyPreserveCredentialsAndClientFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	target := filepath.Join(t.TempDir(), "client-settings")
	legacy := legacyMigrationFixture(t, target)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	originalTarget := []byte("user-owned client state\n")
	if err := os.WriteFile(target, originalTarget, 0o600); err != nil {
		t.Fatal(err)
	}
	credentialStore := &observedSecretStore{Store: secrets.NewMemoryStore()}
	if err := credentialStore.Set("gateway", "must-remain-secret"); err != nil {
		t.Fatal(err)
	}
	credentialStore.reset()
	out := &bytes.Buffer{}
	runtime := invocation.Context{Config: configuration.NewStore(path), Secrets: credentialStore, Out: out, RenderOut: out, Width: 120}

	preview := newMigrateCommand(runtime)
	preview.SetArgs([]string{"--dry-run", "--json"})
	if err := executeManifestCommand(preview); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, `"from_version": 3`) || !strings.Contains(text, `"to_version": 5`) || strings.Contains(text, "must-remain-secret") {
		t.Fatalf("migration preview = %s", text)
	}
	if actual, err := os.ReadFile(path); err != nil || !bytes.Equal(actual, legacy) {
		t.Fatalf("preview changed configuration: %q, %v", actual, err)
	}

	out.Reset()
	apply := newMigrateCommand(runtime)
	if err := executeManifestCommand(apply); err != nil {
		t.Fatal(err)
	}
	if token, err := credentialStore.Get("gateway"); err != nil || token != "must-remain-secret" {
		t.Fatalf("migration changed credential: %q, %v", token, err)
	}
	if credentialStore.reads != 1 || credentialStore.writes != 0 {
		t.Fatalf("migration accessed credentials before the explicit assertion: reads=%d writes=%d", credentialStore.reads-1, credentialStore.writes)
	}
	if actual, err := os.ReadFile(target); err != nil || !bytes.Equal(actual, originalTarget) {
		t.Fatalf("migration changed client state: %q, %v", actual, err)
	}
	if _, err := runtime.Config.Load(); err != nil {
		t.Fatalf("migrated config is not current: %v", err)
	}
}

func TestMigrationRollbackUsesRetainedPredecessorWithoutTouchingCredentialsOrClients(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	target := filepath.Join(t.TempDir(), "client-settings")
	legacy := legacyMigrationFixture(t, target)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	originalTarget := []byte("user-owned client state\n")
	if err := os.WriteFile(target, originalTarget, 0o600); err != nil {
		t.Fatal(err)
	}
	credentialStore := &observedSecretStore{Store: secrets.NewMemoryStore()}
	if err := credentialStore.Set("gateway", "must-remain-secret"); err != nil {
		t.Fatal(err)
	}
	credentialStore.reset()
	out := &bytes.Buffer{}
	runtime := invocation.Context{Config: configuration.NewStore(path), Secrets: credentialStore, Out: out, RenderOut: out, Width: 120}

	apply := newMigrateCommand(runtime)
	if err := executeManifestCommand(apply); err != nil {
		t.Fatal(err)
	}
	rollback := newMigrateCommand(runtime)
	rollback.SetArgs([]string{"--rollback"})
	if err := executeManifestCommand(rollback); err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(path); err != nil || !bytes.Equal(actual, legacy) {
		t.Fatalf("rollback configuration = %q, %v", actual, err)
	}
	if actual, err := os.ReadFile(target); err != nil || !bytes.Equal(actual, originalTarget) {
		t.Fatalf("rollback changed client state: %q, %v", actual, err)
	}
	if credentialStore.reads+credentialStore.writes != 0 {
		t.Fatalf("migration accessed credentials: reads=%d writes=%d", credentialStore.reads, credentialStore.writes)
	}
}

type observedSecretStore struct {
	secrets.Store
	reads  int
	writes int
}

func (store *observedSecretStore) Get(account string) (string, error) {
	store.reads++
	return store.Store.Get(account)
}

func (store *observedSecretStore) Exists(account string) (bool, error) {
	store.reads++
	return store.Store.Exists(account)
}

func (store *observedSecretStore) Set(account, value string) error {
	store.writes++
	return store.Store.Set(account, value)
}

func (store *observedSecretStore) Delete(account string) error {
	store.writes++
	return store.Store.Delete(account)
}

func (store *observedSecretStore) reset() {
	store.reads = 0
	store.writes = 0
}

func legacyMigrationFixture(t *testing.T, target string) []byte {
	t.Helper()
	data, err := toml.Marshal(map[string]any{
		"version": 3,
		"accounts": map[string]any{"gateway": map[string]any{
			"label": "Gateway", "endpoints": map[string]string{"openai_responses": "https://gateway.test/v1"},
		}},
		"profiles": map[string]any{"codex": map[string]string{
			"label": "Codex", "account": "gateway", "client": "codex", "model": "gpt-test",
		}},
		"routes": map[string]string{"codex": "codex"},
		"adapters": map[string]any{"codex": map[string]any{
			"enabled": true, "executable": filepath.Join(filepath.Dir(target), "codex"), "targets": []string{target},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
