package configuration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathReturnsConfiguredPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if got := NewStore(path).Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
}

func TestCaptureSnapshotSurfacesConfigReadErrors(t *testing.T) {
	path := t.TempDir()
	if _, err := NewStore(path).CaptureSnapshot(); err == nil {
		t.Fatal("CaptureSnapshot succeeded despite a directory at the config path")
	}
}

func TestCaptureSnapshotSurfacesBackupReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).CaptureSnapshot(); err == nil {
		t.Fatal("CaptureSnapshot succeeded despite a directory at the backup path")
	}
}

func TestCaptureVerifiedBackupStateRequiresExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := NewStore(path).CaptureVerifiedBackupState()
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing config error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateSurfacesConfigReadErrors(t *testing.T) {
	path := t.TempDir()
	if _, err := NewStore(path).CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the config path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesBackupReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
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
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
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
	_, err := NewStore(path).CaptureVerifiedBackupState()
	if err == nil || !strings.Contains(err.Error(), "decode current config snapshot") {
		t.Fatalf("malformed config decode error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateSurfacesCheckpointDecodeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
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
	path := t.TempDir()
	_, err := NewStore(path).ConvergeVerifiedBackup(VerifiedBackupSnapshot{})
	if err == nil {
		t.Fatal("ConvergeVerifiedBackup succeeded despite a directory at the config path")
	}
}

func TestConvergeVerifiedBackupSurfacesVerifiedReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
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
	expected := VerifiedBackupSnapshot{Config: configSnapshot.Config}
	if _, err := store.ConvergeVerifiedBackup(expected); err == nil {
		t.Fatal("ConvergeVerifiedBackup succeeded despite a directory at the verified checkpoint path")
	}
}

func TestRestoreSnapshotSurfacesConfigPostimageMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
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
	store := NewStore(path)
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
	// A regular file standing where the config directory must be created
	// refuses MkdirAll identically on every platform: unlike a chmod-
	// restricted directory, this is not a permission model Windows is
	// free to ignore (see https://github.com/golang/go/issues/35042).
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(blocked, "child", " toml"))
	if _, err := store.Lock(context.Background()); err == nil {
		t.Fatal("Lock succeeded despite an unwritable config directory")
	}
}

func TestLoadOfMissingConfigReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	got, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != ConfigVersion || len(got.Profiles) != 0 {
		t.Fatalf("default config = %#v", got)
	}
}

func TestLoadSurfacesUnderlyingReadErrors(t *testing.T) {
	path := t.TempDir()
	_, err := NewStore(path).Load()
	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Phase != LoadPhaseRead || loadErr.Err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("Load(directory) error = %v", err)
	}
}

func TestLoadSurfacesTypedParseErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("version = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path).Load()
	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Phase != LoadPhaseParse || loadErr.Err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("malformed config load error = %v", err)
	}
}

func TestSaveSurfacesUnwritableConfigDirectory(t *testing.T) {
	// See TestLockSurfacesUnwritableConfigDirectory: a file blocking the
	// directory component is reachable on every platform, unlike chmod.
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(blocked, "child", " toml"))
	if err := store.Save(convergenceConfig("current")); err == nil {
		t.Fatal("Save succeeded despite an unwritable config directory")
	}
}

func TestSaveSurfacesBackupWriteFailures(t *testing.T) {
	// An existing directory at the backup path refuses the atomic rename
	// that finalizes the backup write on every platform: renaming a file
	// onto an existing directory fails on POSIX (EISDIR) and on Windows
	// (access denied), unlike a chmod-restricted parent directory.
	dir := t.TempDir()
	path := filepath.Join(dir, " toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "back up current config") {
		t.Fatalf("backup write failure error = %v", err)
	}
}

func TestSaveSurfacesUnderlyingCurrentReadErrors(t *testing.T) {
	path := t.TempDir()
	if err := NewStore(path).Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "read current config for backup") {
		t.Fatalf("Save(directory) error = %v", err)
	}
}

func TestSaveVerifiedCheckpointRejectsInvalidConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := NewStore(path).SaveVerifiedCheckpoint(Config{}, []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint accepted an invalid configuration")
	}
}

func TestSaveVerifiedCheckpointSurfacesUnwritableConfigDirectory(t *testing.T) {
	// See TestLockSurfacesUnwritableConfigDirectory: a file blocking the
	// directory component is reachable on every platform, unlike chmod.
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(blocked, "child", " toml"))
	if err := store.SaveVerifiedCheckpoint(convergenceConfig("current"), []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an unwritable config directory")
	}
}

func TestLoadVerifiedCheckpointSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "read verified checkpoint") {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsIncompleteCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path+".verified.json", []byte(`{"config":{"version":2,"accounts":{},"profiles":{},"routes":{"default":""}},"clients":[],"verified_at":"0001-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "incomplete") {
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
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "validate verified checkpoint") {
		t.Fatalf("non-canonical config version error = %v", err)
	}
}

func TestLoadBackupRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
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
	if _, err := NewStore(path).LoadBackup(); err == nil || !strings.Contains(err.Error(), "read previous config backup") {
		t.Fatalf("missing backup error = %v", err)
	}
}

func TestSaveVerifiedCheckpointSurfacesRenameFailureOverExistingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(path).SaveVerifiedCheckpoint(convergenceConfig("current"), []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an existing directory at the checkpoint path")
	}
}
