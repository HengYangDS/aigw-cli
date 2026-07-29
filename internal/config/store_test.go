package config_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestLoadRejectsProfileOwnedEndpointResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 2

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
openai_responses = "https://gateway.test/v1"

[profiles.gpt]
label = "GPT"
account = "gateway"
client = "codex"

[profiles.gpt.models]
codex = "gpt-test"

[profiles.gpt.endpoints]
openai_responses = "https://duplicate.test/v1"

[profiles.gpt.account_probe]
kind = "dmxapi"
base_url = "https://duplicate.test"

[routes]
default = "gpt"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("Profile-owned endpoint residue was accepted")
	}
}

func TestLoadRejectsNonCanonicalSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 1

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
anthropic = "https://gateway.test"

[profiles.agent]
label = "Agent"
account = "gateway"

[routes]
default = "agent"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.NewStore(path).Load(); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("version 1 load error = %v", err)
	}
}

func TestLockSerializesMutations(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
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
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	store := config.NewStore(path)
	want := domain.Config{
		Version:  domain.ConfigVersion,
		Accounts: map[string]domain.Account{"dmx": {Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]domain.Profile{"dmx": {Label: "DMXAPI", Account: "dmx"}},
		Routes:   domain.Routes{Default: "dmx", Overrides: map[string]string{}},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("mode = %o, want 600", got)
		}
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["dmx"].Endpoints.Anthropic != "https://example.test" || got.Routes.Default != "dmx" {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestSaveSeparatesTOMLTableBlocksVisually(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	cfg := domain.Config{
		Version: domain.ConfigVersion,
		Accounts: map[string]domain.Account{
			"dmx": {Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}},
		},
		Profiles: map[string]domain.Profile{
			"claude": {
				Label:   "Claude",
				Account: "dmx",
				Client:  domain.ClientClaude,
				Models:  domain.Models{domain.ClientClaude: "claude-test"},
			},
		},
		Routes: domain.Routes{Default: "claude", Overrides: map[string]string{}},
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

func TestLoadPreservesAdmittedClaudeAndCodexModelKeysAsClientMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := `version = 2
[accounts.gateway]
label = "Gateway"
[accounts.gateway.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"
[profiles.both]
label = "Both"
account = "gateway"
[profiles.both.models]
claude = "claude-test"
codex = "gpt-test"
[routes]
default = "both"
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Profiles["both"].ModelFor(domain.ClientClaude); got != "claude-test" {
		t.Fatalf("Claude model = %q", got)
	}
	if got := cfg.Profiles["both"].ModelFor(domain.ClientCodex); got != "gpt-test" {
		t.Fatalf("Codex model = %q", got)
	}
}

func TestSaveRefusesInvalidConfigWithoutReplacingExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(path)
	if err := store.Save(domain.Config{Version: domain.ConfigVersion}); err == nil {
		t.Fatal("expected validation error")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "sentinel" {
		t.Fatalf("existing config was replaced: %q", got)
	}
}

func TestRestoreSnapshotRestoresAnAbsentConfigurationAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.Config{
		Version:  domain.ConfigVersion,
		Accounts: map[string]domain.Account{"gateway": {Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}},
		Profiles: map[string]domain.Profile{"gateway": {Label: "Gateway", Account: "gateway"}},
		Routes:   domain.Routes{Default: "gateway", Overrides: map[string]string{}},
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
	store := config.NewStore(path)
	first := domain.Config{
		Version:  domain.ConfigVersion,
		Accounts: map[string]domain.Account{"one": {Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}},
		Profiles: map[string]domain.Profile{"one": {Label: "One", Account: "one"}},
		Routes:   domain.Routes{Default: "one", Overrides: map[string]string{}},
	}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Accounts = map[string]domain.Account{"two": {Label: "Two", Endpoints: domain.Endpoints{Anthropic: "https://two.test"}}}
	second.Profiles = map[string]domain.Profile{"two": {Label: "Two", Account: "two"}}
	second.Routes.Default = "two"
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
	store := config.NewStore(path)
	cfg := domain.Config{
		Version:  domain.ConfigVersion,
		Accounts: map[string]domain.Account{"dmx": {Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]domain.Profile{"claude": {Label: "Claude", Account: "dmx", Models: domain.Models{domain.ClientClaude: "claude-test"}}},
		Routes:   domain.Routes{Default: "claude", Overrides: map[string]string{}},
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
	if checkpoint.Config.Routes.Default != "claude" || len(checkpoint.Clients) != 2 || checkpoint.VerifiedAt.IsZero() {
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
	if _, err := config.NewStore(path).LoadVerifiedCheckpoint(); err == nil {
		t.Fatal("Profile-owned checkpoint endpoint residue was accepted")
	}
}

func TestConvergeVerifiedBackupCopiesExactCurrentBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	oldConfig := convergenceConfig("old")
	currentConfig := convergenceConfig("current")
	if err := store.Save(oldConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(currentConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(currentConfig, domain.AdmittedClientIDs()); err != nil {
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
	if result.Backup.Mode != 0o600 {
		t.Fatalf("converged backup mode = %o, want 600", result.Backup.Mode)
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
			store := config.NewStore(path)
			if err := store.Save(convergenceConfig("old")); err != nil {
				t.Fatal(err)
			}
			current := convergenceConfig("current")
			if err := store.Save(current); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(current, domain.AdmittedClientIDs()); err != nil {
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
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	_, err := store.CaptureVerifiedBackupState()
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}

func convergenceConfig(id string) domain.Config {
	cfg := domain.NewConfig()
	cfg.Accounts[id] = domain.Account{Label: strings.ToUpper(id), Endpoints: domain.Endpoints{OpenAIResponses: "https://" + id + ".test/v1"}}
	cfg.Profiles[id] = domain.Profile{Label: strings.ToUpper(id), Account: id, Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: id + "-model"}}
	cfg.Routes.Default = id
	return cfg
}

func TestPathReturnsConfiguredPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if got := config.NewStore(path).Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
}

func TestCaptureSnapshotSurfacesConfigReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := t.TempDir()
	if _, err := config.NewStore(path).CaptureSnapshot(); err == nil {
		t.Fatal("CaptureSnapshot succeeded despite a directory at the config path")
	}
}

func TestCaptureSnapshotSurfacesBackupReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := config.NewStore(path).CaptureSnapshot(); err == nil {
		t.Fatal("CaptureSnapshot succeeded despite a directory at the backup path")
	}
}

func TestCaptureVerifiedBackupStateRequiresExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := config.NewStore(path).CaptureVerifiedBackupState()
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing config error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateSurfacesConfigReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := t.TempDir()
	if _, err := config.NewStore(path).CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the config path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesBackupReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the backup path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesVerifiedReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the verified checkpoint path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesConfigDecodeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("not = [valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".verified.json", []byte(`{"config":{"version":2,"accounts":{},"profiles":{},"routes":{"default":""}},"clients":["codex"],"verified_at":"2026-07-11T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.NewStore(path).CaptureVerifiedBackupState()
	if err == nil || !strings.Contains(err.Error(), "decode current config snapshot") {
		t.Fatalf("malformed config decode error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateSurfacesCheckpointDecodeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".verified.json", []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil || !strings.Contains(err.Error(), "parse verified checkpoint") {
		t.Fatalf("malformed checkpoint decode error = %v", err)
	}
}

func TestConvergeVerifiedBackupSurfacesConfigReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := t.TempDir()
	_, err := config.NewStore(path).ConvergeVerifiedBackup(config.VerifiedBackupSnapshot{})
	if err == nil {
		t.Fatal("ConvergeVerifiedBackup succeeded despite a directory at the config path")
	}
}

func TestConvergeVerifiedBackupSurfacesVerifiedReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	configSnapshot, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	expected := config.VerifiedBackupSnapshot{Config: configSnapshot.Config}
	if _, err := store.ConvergeVerifiedBackup(expected); err == nil {
		t.Fatal("ConvergeVerifiedBackup succeeded despite a directory at the verified checkpoint path")
	}
}

func TestRestoreSnapshotSurfacesConfigPostimageMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreSnapshot(before, after); err == nil || !strings.Contains(err.Error(), "restore config snapshot") {
		t.Fatalf("config postimage mismatch error = %v", err)
	}
}

func TestRestoreSnapshotSurfacesBackupPostimageMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".bak", []byte("external backup change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreSnapshot(before, after); err == nil || !strings.Contains(err.Error(), "restore config backup snapshot") {
		t.Fatalf("backup postimage mismatch error = %v", err)
	}
}

func TestLockSurfacesUnwritableConfigDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	store := config.NewStore(filepath.Join(locked, "child", "config.toml"))
	if _, err := store.Lock(context.Background()); err == nil {
		t.Fatal("Lock succeeded despite an unwritable config directory")
	}
}

func TestLoadOfMissingConfigReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	got, err := config.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != domain.ConfigVersion || len(got.Profiles) != 0 {
		t.Fatalf("default config = %#v", got)
	}
}

func TestLoadSurfacesUnderlyingReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := t.TempDir()
	if _, err := config.NewStore(path).Load(); err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("Load(directory) error = %v", err)
	}
}

func TestSaveSurfacesUnwritableConfigDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	store := config.NewStore(filepath.Join(locked, "child", "config.toml"))
	if err := store.Save(convergenceConfig("current")); err == nil {
		t.Fatal("Save succeeded despite an unwritable config directory")
	}
}

func TestSaveSurfacesBackupWriteFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := store.Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "back up current config") {
		t.Fatalf("backup write failure error = %v", err)
	}
}

func TestSaveSurfacesUnderlyingCurrentReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := t.TempDir()
	if err := config.NewStore(path).Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "read current config for backup") {
		t.Fatalf("Save(directory) error = %v", err)
	}
}

func TestSaveVerifiedCheckpointRejectsInvalidConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := config.NewStore(path).SaveVerifiedCheckpoint(domain.Config{}, []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint accepted an invalid configuration")
	}
}

func TestSaveVerifiedCheckpointSurfacesUnwritableConfigDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	store := config.NewStore(filepath.Join(locked, "child", "config.toml"))
	if err := store.SaveVerifiedCheckpoint(convergenceConfig("current"), []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an unwritable config directory")
	}
}

func TestLoadVerifiedCheckpointSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := config.NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "read verified checkpoint") {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsIncompleteCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path+".verified.json", []byte(`{"config":{"version":2,"accounts":{},"profiles":{},"routes":{"default":""}},"clients":[],"verified_at":"0001-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsNonCanonicalConfigVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	checkpoint := `{
  "config": {
    "version": 1,
    "accounts": {"team": {"label": "Team", "endpoints": {"anthropic": "https://team.test"}}},
    "profiles": {"team": {"label": "Team", "account": "team"}},
    "routes": {"default": "team"}
  },
  "clients": ["codex"],
  "verified_at": "2026-07-11T00:00:00Z"
}`
	if err := os.WriteFile(path+".verified.json", []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "validate verified checkpoint") {
		t.Fatalf("non-canonical config version error = %v", err)
	}
}

func TestLoadBackupRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	backup, err := store.LoadBackup()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backup.Accounts["old"]; !ok {
		t.Fatalf("backup config = %#v, want the previous version", backup)
	}
}

func TestLoadBackupSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := config.NewStore(path).LoadBackup(); err == nil || !strings.Contains(err.Error(), "read previous config backup") {
		t.Fatalf("missing backup error = %v", err)
	}
}

func TestSaveSurfacesTemporaryFileCreationFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	path := filepath.Join(dir, "config.toml")
	if err := config.NewStore(path).Save(convergenceConfig("current")); err == nil {
		t.Fatal("Save succeeded despite an unwritable existing config directory")
	}
}

func TestSaveVerifiedCheckpointSurfacesRenameFailureOverExistingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := config.NewStore(path).SaveVerifiedCheckpoint(convergenceConfig("current"), []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an existing directory at the checkpoint path")
	}
}
