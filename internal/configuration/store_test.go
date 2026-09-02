package configuration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadRejectsProfileOwnedEndpointResidue(t *testing.T) {
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
		t.Fatal("Profile-owned endpoint residue was accepted")
	}
}

func TestLoadRequiresCanonicalSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	previousVersion := ConfigVersion - 1
	raw := fmt.Appendf(nil, "version = %d\n", previousVersion)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
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

func TestLoadRejectsUnknownVersionThreeField(t *testing.T) {
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
	if !errors.As(err, &loadErr) || loadErr.Phase != LoadPhaseParse {
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
		Version:  ConfigVersion,
		Accounts: map[string]Account{"dmx": {Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]Profile{"dmx": {Label: "DMXAPI", Account: "dmx", Client: ClientClaude, Model: "claude-test"}},
		Routes:   Routes{ClientClaude: "dmx"},
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
	if got.Accounts["dmx"].Endpoints.Anthropic != "https://example.test" || got.Routes[ClientClaude] != "dmx" {
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
		Profiles: map[string]Profile{
			"claude": {
				Label:   "Claude",
				Account: "dmx",
				Client:  ClientClaude,
				Model:   "claude-test",
			},
		},
		Routes: Routes{ClientClaude: "claude"},
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
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if err := store.Save(Config{Version: ConfigVersion}); err == nil {
		t.Fatal("expected validation error")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "sentinel" {
		t.Fatalf("existing config was replaced: %q", got)
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
		Profiles: map[string]Profile{"gateway": {Label: "Gateway", Account: "gateway", Client: ClientClaude, Model: "claude-test"}},
		Routes:   Routes{ClientClaude: "gateway"},
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

func TestSaveKeepsOneSecretFreePreviousVersionBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	first := Config{
		Version:  ConfigVersion,
		Accounts: map[string]Account{"one": {Label: "One", Endpoints: Endpoints{Anthropic: "https://one.test"}}},
		Profiles: map[string]Profile{"one": {Label: "One", Account: "one", Client: ClientClaude, Model: "claude-one"}},
		Routes:   Routes{ClientClaude: "one"},
	}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Accounts = map[string]Account{"two": {Label: "Two", Endpoints: Endpoints{Anthropic: "https://two.test"}}}
	second.Profiles = map[string]Profile{"two": {Label: "Two", Account: "two", Client: ClientClaude, Model: "claude-two"}}
	second.Routes = Routes{ClientClaude: "two"}
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), `[profiles.one]`) || strings.Contains(strings.ToLower(string(backup)), "token") {
		t.Fatalf("backup = %s", backup)
	}
}

func TestVerifiedCheckpointRoundTripIsSecretFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	cfg := Config{
		Version:  ConfigVersion,
		Accounts: map[string]Account{"dmx": {Label: "DMX", Endpoints: Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]Profile{"claude": {Label: "Claude", Account: "dmx", Client: ClientClaude, Model: "claude-test"}},
		Routes:   Routes{ClientClaude: "claude"},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(cfg, []string{"claude", "codex"}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Config.Routes[ClientClaude] != "claude" || len(checkpoint.Clients) != 2 || checkpoint.VerifiedAt.IsZero() {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
	data, err := os.ReadFile(path + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "token") {
		t.Fatalf("checkpoint contains token-like content: %s", data)
	}
}

func TestLoadVerifiedCheckpointRejectsProfileOwnedEndpointResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	checkpoint := `{
  "config": {
    "version": 1,
    "accounts": {
      "gateway": {
        "label": "Gateway",
        "endpoints": {"openai_responses": "https://gateway.test/v1"}
      }
    },
    "profiles": {
      "gpt": {
        "label": "GPT",
        "account": "gateway",
        "client": "codex",
        "models": {"codex": "gpt-test"},
        "endpoints": {"openai_responses": "https://duplicate.test/v1"}
      }
    },
    "routes": {"default": "gpt"}
  },
  "clients": ["codex"],
  "verified_at": "2026-07-11T00:00:00Z"
}`
	if err := os.WriteFile(path+".verified.json", []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil {
		t.Fatal("Profile-owned checkpoint endpoint residue was accepted")
	}
}

func TestConvergeVerifiedBackupCopiesExactCurrentBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	oldConfig := convergenceConfig("old")
	currentConfig := convergenceConfig("current")
	if err := store.Save(oldConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(currentConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(currentConfig, AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
	currentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	customBytes := append([]byte("# byte-exact verified current\n"), currentBytes...)
	if err := os.WriteFile(path, customBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	verifiedBefore, err := os.ReadFile(path + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.ConvergeVerifiedBackup(state.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, customBytes) || !bytes.Equal(result.Backup.Data, customBytes) {
		t.Fatalf("backup was not converged byte-exactly\nwant %q\ngot  %q", customBytes, backup)
	}
	if want := securePersistedFileMode(); result.Backup.Mode != want {
		t.Fatalf("converged backup mode = %o, want %o", result.Backup.Mode, want)
	}
	verifiedAfter, err := os.ReadFile(path + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(verifiedAfter, verifiedBefore) {
		t.Fatal("backup convergence changed the verified checkpoint")
	}
}

func TestConvergeVerifiedBackupRejectsChangedPreimages(t *testing.T) {
	for _, changed := range []string{"config", "backup", "verified"} {
		t.Run(changed, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			store := NewStore(path)
			if err := store.Save(convergenceConfig("old")); err != nil {
				t.Fatal(err)
			}
			current := convergenceConfig("current")
			if err := store.Save(current); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(current, AdmittedClientIDs()); err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if err != nil {
				t.Fatal(err)
			}
			backupBefore, err := os.ReadFile(path + ".bak")
			if err != nil {
				t.Fatal(err)
			}
			changedPath := map[string]string{
				"config":   path,
				"backup":   path + ".bak",
				"verified": path + ".verified.json",
			}[changed]
			changedBytes := []byte("external " + changed + " change\n")
			if changed == "config" {
				original, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				changedBytes = append(original, []byte("# external change\n")...)
			}
			if changed == "verified" {
				original, readErr := os.ReadFile(path + ".verified.json")
				if readErr != nil {
					t.Fatal(readErr)
				}
				changedBytes = append(original, '\n')
			}
			if err := os.WriteFile(changedPath, changedBytes, 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := store.ConvergeVerifiedBackup(state.Snapshot); err == nil || !strings.Contains(err.Error(), "preimage changed") {
				t.Fatalf("convergence error = %v", err)
			}
			backupAfter, err := os.ReadFile(path + ".bak")
			if err != nil {
				t.Fatal(err)
			}
			wantBackup := backupBefore
			if changed == "backup" {
				wantBackup = changedBytes
			}
			if !bytes.Equal(backupAfter, wantBackup) {
				t.Fatalf("backup overwritten after %s preimage change: %q", changed, backupAfter)
			}
		})
	}
}

func TestCaptureVerifiedBackupStateRequiresCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	_, err := store.CaptureVerifiedBackupState()
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing checkpoint error = %v", err)
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

func convergenceConfig(id string) Config {
	cfg := NewConfig()
	cfg.Accounts[id] = Account{Label: strings.ToUpper(id), Endpoints: Endpoints{OpenAIResponses: "https://" + id + ".test/v1"}}
	cfg.Profiles[id] = Profile{Label: strings.ToUpper(id), Account: id, Client: ClientCodex, Model: id + "-model"}
	cfg.Routes[ClientCodex] = id
	return cfg
}
