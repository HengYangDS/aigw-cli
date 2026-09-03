package configuration

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/transaction"
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

func TestCaptureSnapshotSurfacesVerifiedCheckpointReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).CaptureSnapshot(); err == nil {
		t.Fatal("CaptureSnapshot succeeded despite a directory at the verified checkpoint path")
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
	if err := os.WriteFile(path+".verified.json", []byte(`{}`), 0o600); err != nil {
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
	if err := store.Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "capture current config") {
		t.Fatalf("backup write failure error = %v", err)
	}
}

func TestSaveSurfacesUnderlyingCurrentReadErrors(t *testing.T) {
	path := t.TempDir()
	if err := NewStore(path).Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "capture current config") {
		t.Fatalf("Save(directory) error = %v", err)
	}
}

func TestCommitRestoresPreparedSnapshotWhenPostimageCaptureFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	originalWrite := writeConfigurationFileIfUnchanged
	t.Cleanup(func() { writeConfigurationFileIfUnchanged = originalWrite })
	writes := 0
	writeConfigurationFileIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		writes++
		if writes == 2 {
			return transaction.FileSnapshot{}, errors.New("write config failed")
		}
		return originalWrite(target, expected, data, mode)
	}
	result, err := store.Commit(before, convergenceConfig("current"))
	if err == nil || !strings.Contains(err.Error(), "write config") {
		t.Fatalf("Commit() = %#v, %v; want config write failure", result, err)
	}
	after, captureErr := store.CaptureSnapshot()
	if captureErr != nil {
		t.Fatal(captureErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("snapshot after failed commit = %#v, want %#v", after, before)
	}
}

func TestCommitLeavesConfigurationUntouchedWhenBackupWriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("backup write failed")
	originalWrite := writeConfigurationFileIfUnchanged
	t.Cleanup(func() { writeConfigurationFileIfUnchanged = originalWrite })
	writeConfigurationFileIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if target == path+".bak" {
			return transaction.FileSnapshot{}, want
		}
		return originalWrite(target, expected, data, mode)
	}

	if _, err := store.Commit(before, convergenceConfig("current")); !errors.Is(err, want) || !strings.Contains(err.Error(), "back up current config") {
		t.Fatalf("Commit() error = %v, want backup write failure", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("snapshot after failed backup = %#v, want %#v", after, before)
	}
}

func TestCommitPreservesNewerBackupWhenConfigurationWriteAndCompensationConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("config write failed")
	newerBackup := []byte("newer backup\n")
	originalWrite := writeConfigurationFileIfUnchanged
	t.Cleanup(func() { writeConfigurationFileIfUnchanged = originalWrite })
	writeConfigurationFileIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if target == path {
			return transaction.FileSnapshot{}, want
		}
		postimage, writeErr := originalWrite(target, expected, data, mode)
		if writeErr != nil {
			return transaction.FileSnapshot{}, writeErr
		}
		if err := os.WriteFile(target, newerBackup, 0o600); err != nil {
			t.Fatal(err)
		}
		return postimage, nil
	}

	_, err = store.Commit(before, convergenceConfig("current"))
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restore config backup") || !strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("Commit() error = %v, want write and compensation conflict", err)
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(current, before.Config.Data) {
		t.Fatalf("configuration after failed commit = %q, %v", current, readErr)
	}
	backup, readErr := os.ReadFile(path + ".bak")
	if readErr != nil || !bytes.Equal(backup, newerBackup) {
		t.Fatalf("newer backup after rejected compensation = %q, %v", backup, readErr)
	}
}

func TestCommitRestoresConfigurationWhenVerifiedCheckpointInvalidationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	beforeConfig := convergenceConfig("before")
	if err := store.Save(beforeConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(beforeConfig, []string{ClientClaude}); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("checkpoint removal failed")
	originalRemove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() { removeConfigurationFileIfUnchanged = originalRemove })
	removeConfigurationFileIfUnchanged = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		return transaction.FileSnapshot{}, want
	}

	if _, err := store.Commit(before, convergenceConfig("after")); !errors.Is(err, want) || !strings.Contains(err.Error(), "invalidate verified checkpoint") {
		t.Fatalf("Commit() error = %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("snapshot after failed checkpoint invalidation = %#v, want %#v", after, before)
	}
}

func TestCommitReportsConfigurationRestoreFailureAfterCheckpointInvalidationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("before")); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	originalRemove := removeConfigurationFileIfUnchanged
	originalWrite := writeConfigurationFileIfUnchanged
	t.Cleanup(func() {
		removeConfigurationFileIfUnchanged = originalRemove
		writeConfigurationFileIfUnchanged = originalWrite
	})
	removeConfigurationFileIfUnchanged = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		return transaction.FileSnapshot{}, errors.New("checkpoint removal failed")
	}
	writes := 0
	writeConfigurationFileIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		writes++
		return originalWrite(target, expected, data, mode)
	}
	// Mutating the just-written config makes the guarded compensation refuse
	// to overwrite a newer writer, which is the failure this branch reports.
	removeConfigurationFileIfUnchanged = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if err := os.WriteFile(path, []byte("newer writer"), 0o600); err != nil {
			t.Fatal(err)
		}
		return transaction.FileSnapshot{}, errors.New("checkpoint removal failed")
	}
	_, err = store.Commit(before, convergenceConfig("after"))
	if err == nil || !strings.Contains(err.Error(), "restore config") || writes == 0 {
		t.Fatalf("Commit() error = %v", err)
	}
}

func TestCommitReportsBackupRestoreFailureAfterCheckpointInvalidationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("before")); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	originalRemove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() { removeConfigurationFileIfUnchanged = originalRemove })
	removeConfigurationFileIfUnchanged = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if err := os.WriteFile(path+".bak", []byte("newer backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		return transaction.FileSnapshot{}, errors.New("checkpoint removal failed")
	}
	_, err = store.Commit(before, convergenceConfig("after"))
	if err == nil || !strings.Contains(err.Error(), "restore config backup") {
		t.Fatalf("Commit() error = %v", err)
	}
}

func TestRestoreSnapshotSurfacesVerifiedCheckpointPostimageMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	beforeConfig := convergenceConfig("before")
	if err := store.Save(beforeConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(beforeConfig, []string{ClientCodex}); err != nil {
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
	if err := os.WriteFile(path+".verified.json", []byte("newer checkpoint"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreSnapshot(before, after); err == nil || !strings.Contains(err.Error(), "restore verified checkpoint snapshot") {
		t.Fatalf("verified checkpoint postimage mismatch error = %v", err)
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
	if err := os.WriteFile(path+".verified.json", []byte(`{}`), 0o600); err != nil {
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
