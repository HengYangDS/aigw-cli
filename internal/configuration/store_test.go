package configuration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestForwardingStorePreservesStrictPublishedConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/backup/v1"
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Equal(original, canonical) {
		t.Fatalf("forwarding rewrote predecessor-readable configuration: %v", err)
	}
	loaded, err := store.Load()
	if err != nil || loaded.Clients[ClientCodex].ForwardingEndpoint != binding.ForwardingEndpoint {
		t.Fatalf("forwarding did not survive Store round trip: %#v, %v", loaded.Clients, err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.Commit(before, loaded)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("unchanged forwarding commit rewrote its snapshot: %v", err)
	}
}

func TestForwardingSnapshotRestoresPresenceAndPriorBinding(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/backup/v1"
	cfg.Clients[ClientCodex] = binding
	after, err := store.Commit(before, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	restored, err := store.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, restored) {
		t.Fatalf("forwarding rollback did not restore exact prior presence: %v", err)
	}
	loaded, err := store.Load()
	if err != nil || loaded.Clients[ClientCodex].ForwardingEndpoint != "" {
		t.Fatalf("forwarding remained after rollback: %#v, %v", loaded.Clients, err)
	}
}

func TestForwardingStoreRejectsMixedAccountGenerations(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := validConfig()
	binding := cfg.Clients[ClientCodex]
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/backup/v1"
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Routes["other"] = testRoute("Other", "dmx", "other-model", ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ClientCodex, "other", "")
	canonical, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), canonical, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("mixed canonical and forwarding generations were admitted")
	}
}

func TestForwardingMigrationPreviewPreservesAssembledBinding(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := validConfig()
	binding := cfg.Clients[ClientCodex]
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/v1"
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	plan, err := store.PrepareMigration(false)
	if err != nil || plan.Required || plan.Clients[ClientCodex].ForwardingEndpoint != binding.ForwardingEndpoint {
		t.Fatalf("current-schema migration preview lost a client destination: %#v, %v", plan.Clients, err)
	}
}

func TestForwardingUnchangedCanonicalRejectsStalePreimage(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	newer := append([]byte("# operator edit\n"), before.Config.Data...)
	if err := os.WriteFile(store.Path(), newer, 0o600); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/v1"
	cfg.Clients[ClientCodex] = binding
	if _, err := store.Commit(before, cfg); err == nil {
		t.Fatal("forwarding committed despite a newer canonical preimage")
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !bytes.Equal(after.Config.Data, newer) || !after.Backup.Equal(before.Backup) || after.Forwarding.Exists {
		t.Fatalf("rejected stale commit changed independent state: %v", err)
	}
}

func TestLoadRejectsLegacyProfileOwnedEndpointResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 3

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
openai_responses = "https://gateway.test/v1"

[profiles.gpt]
label = "GPT"
account = "gateway"
client = "codex"
model = "gpt-test"

[profiles.gpt.endpoints]
openai_responses = "https://duplicate.test/v1"

[routes]
codex = "gpt"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("Legacy Profile-owned endpoint residue was accepted")
	}
}

func TestLoadRequiresCanonicalSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	previousVersion := ConfigVersion - 1
	raw := fmt.Appendf(nil, "version = %d\n", previousVersion)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path).Load()
	var loadErr *LoadError
	var versionErr *UnsupportedConfigVersionError
	if !errors.As(err, &loadErr) || loadErr.Phase != LoadPhaseValidate || !errors.As(err, &versionErr) {
		t.Fatalf("previous-version load error = %v", err)
	}
	if versionErr.Version != previousVersion || versionErr.ExpectedVersion != ConfigVersion {
		t.Fatalf("previous-version load error context = %#v, %v", versionErr, err)
	}
}

func TestLoadRejectsLegacySelectionFieldsAtTheCurrentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := fmt.Sprintf(`version = %d

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
anthropic = "https://gateway.test"

[profiles.claude]
label = "Claude"
account = "gateway"
client = "claude"
model = "claude-test"

[routes]
claude = "claude"

[adapters.claude]
enabled = true
`, ConfigVersion)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path).Load()
	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Phase != LoadPhaseParse {
		t.Fatalf("legacy selection load error = %v", err)
	}
}

func TestLoadRejectsUnsupportedVersionBeforeReadingItsFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 3
unexpected = true
[accounts.gateway]
label = "Gateway"
[accounts.gateway.endpoints]
anthropic = "https://gateway.test"
[profiles.claude]
label = "Claude"
account = "gateway"
client = "claude"
model = "claude-test"
[routes]
claude = "claude"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path).Load()
	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Phase != LoadPhaseValidate {
		t.Fatalf("unknown field load error = %v", err)
	}
}

func TestLockSerializesMutations(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	unlock, err := store.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			t.Error(unlockErr)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err = store.Lock(ctx)
	if err == nil || !strings.Contains(err.Error(), "context deadline") {
		t.Fatalf("second lock error = %v", err)
	}
}

func TestSaveLoadRoundTripAndSecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", " toml")
	store := NewStore(path)
	want := Config{
		Version:         ConfigVersion,
		Accounts:        map[string]Account{"dmx": {Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://example.test"}}},
		Routes:          map[string]Route{"dmx": testRoute("DMXAPI", "dmx", "claude-test", ProtocolAnthropic)},
		Recommendations: map[string]ClientRecommendation{ClientClaude: {Primary: ClientSelection{Route: "dmx"}}},
		Clients:         map[string]ClientBinding{ClientClaude: {Route: "dmx"}},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), securePersistedFileMode(); got != want {
		t.Fatalf("mode = %o, want %o", got, want)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["dmx"].Endpoints.Anthropic != "https://example.test" || got.SelectedRoute(ClientClaude) != "dmx" || got.RecommendedRoute(ClientClaude) != "dmx" {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestSaveSeparatesTOMLTableBlocksVisually(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	cfg := Config{
		Version: ConfigVersion,
		Accounts: map[string]Account{
			"dmx": {Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://example.test"}},
		},
		Routes: map[string]Route{
			"claude": testRoute("Claude", "dmx", "claude-test", ProtocolAnthropic),
		},
		Clients: map[string]ClientBinding{ClientClaude: {Route: "claude"}},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	previousTable := -1
	for index, line := range lines {
		if !strings.HasPrefix(line, "[") || strings.HasPrefix(line, "[[") {
			continue
		}
		if previousTable < 0 {
			previousTable = index
			continue
		}
		if strings.TrimSpace(lines[index-1]) != "" {
			t.Fatalf("table %q at line %d must have exactly one blank separator:\n%s", line, index+1, data)
		}
		if index > 1 && strings.TrimSpace(lines[index-2]) == "" {
			t.Fatalf("table %q at line %d has a double separator:\n%s", line, index+1, data)
		}
		previousTable = index
	}
}

func TestSaveRefusesInvalidConfigWithoutReplacingExistingFile(t *testing.T) {
	incompatible := validConfig()
	account := incompatible.Accounts["dmx"]
	account.Endpoints.Anthropic = ""
	incompatible.Accounts["dmx"] = account
	for name, cfg := range map[string]Config{
		"empty":                       {Version: ConfigVersion},
		"incompatible legacy Profile": incompatible,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewStore(path)
			if err := store.Save(cfg); err == nil {
				t.Fatal("expected validation error")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "sentinel" {
				t.Fatalf("existing config was replaced: %q, %v", got, err)
			}
		})
	}
}

func TestRestoreSnapshotRestoresAnAbsentConfigurationAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Version:  ConfigVersion,
		Accounts: map[string]Account{"gateway": {Label: "Gateway", Endpoints: Endpoints{Anthropic: "https://gateway.test"}}},
		Routes:   map[string]Route{"gateway": testRoute("Gateway", "gateway", "claude-test", ProtocolAnthropic)},
		Clients:  map[string]ClientBinding{ClientClaude: {Route: "gateway"}},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config remains after restore: %v", err)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup remains after restore: %v", err)
	}
}

func TestCommitInvalidatesAndRestoreSnapshotRecoversVerifiedCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	beforeConfig := convergenceConfig("before")
	if err := store.Save(beforeConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), beforeConfig, []string{ClientClaude}); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.Commit(before, convergenceConfig("after"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".verified.json"); !os.IsNotExist(err) {
		t.Fatalf("verified checkpoint remains after configuration change: %v", err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Config.Accounts["before"].Label != "BEFORE" {
		t.Fatalf("restored checkpoint = %#v", checkpoint)
	}
}

func TestSaveKeepsOneSecretFreePreviousVersionBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	first := Config{
		Version:  ConfigVersion,
		Accounts: map[string]Account{"one": {Label: "One", Endpoints: Endpoints{Anthropic: "https://one.test"}}},
		Routes:   map[string]Route{"one": testRoute("One", "one", "claude-one", ProtocolAnthropic)},
		Clients:  map[string]ClientBinding{ClientClaude: {Route: "one"}},
	}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Accounts = map[string]Account{"two": {Label: "Two", Endpoints: Endpoints{Anthropic: "https://two.test"}}}
	second.Routes = map[string]Route{"two": testRoute("Two", "two", "claude-two", ProtocolAnthropic)}
	second.Clients = map[string]ClientBinding{ClientClaude: {Route: "two"}}
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), `[routes.one]`) || strings.Contains(strings.ToLower(string(backup)), "token") {
		t.Fatalf("backup = %s", backup)
	}
}

// securePersistedFileMode is the exact permission bits Store persists secrets
// with, as observed through os.Stat. On Unix this is the literal 0o600 mode
// passed to WriteFileAtomicExactMode/os.Chmod. Windows has no POSIX
// owner/group/other bits: os.Chmod only toggles the FILE_ATTRIBUTE_READONLY
// attribute, and a writable file is always reported back as 0o666 (see
// https://pkg.go.dev/os#Chmod). Asserting 0o600 unconditionally on Windows is
// not a real platform mode; it is the wrong expectation for that platform.
func securePersistedFileMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o666
	}
	return 0o600
}
