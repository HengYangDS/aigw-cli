package configuration

import (
	"aigw-cli/internal/transaction"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestCaptureSnapshotSurfacesVerifiedCheckpointReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).CaptureSnapshot(); err == nil {
		t.Fatal("CaptureSnapshot succeeded despite a directory at the verified checkpoint path")
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
	if got.Version != ConfigVersion || len(got.Routes) != 0 {
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
	if err := store.SaveVerifiedCheckpoint(t.Context(), beforeConfig, []string{ClientClaude}); err != nil {
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
	t.Cleanup(func() { removeConfigurationFileIfUnchanged = originalRemove })
	failure := errors.New("checkpoint removal failed")
	foreign := []byte("newer writer")
	// Mutating the just-written config makes the guarded compensation refuse
	// to overwrite a newer writer, which is the failure this branch reports.
	removeConfigurationFileIfUnchanged = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if err := os.WriteFile(path, foreign, 0o600); err != nil {
			t.Fatal(err)
		}
		return transaction.FileSnapshot{}, failure
	}
	_, err = store.Commit(before, convergenceConfig("after"))
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "restore config") {
		t.Fatalf("Commit() error = %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !bytes.Equal(after.Config.Data, foreign) || !after.Backup.Equal(before.Backup) || !after.Verified.Equal(before.Verified) {
		t.Fatalf("compensation changed foreign config or failed to restore independent recovery state: %v", err)
	}
}

func TestRestoreSnapshotPreservesConflictsAndRestoresEveryOwnedFile(t *testing.T) {
	for _, test := range []struct {
		name    string
		changed []int
	}{
		{"config", []int{0}},
		{"backup", []int{1}},
		{"checkpoint", []int{2}},
		{"both-recovery-files", []int{1, 2}},
		{"forwarding", []int{3}},
		{"forwarding-backup", []int{4}},
		{"forwarding-and-checkpoint", []int{2, 3, 4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			store := NewStore(path)
			cfg := convergenceConfig("before")
			if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/before/v1"); err != nil {
				t.Fatal(err)
			}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude}); err != nil {
				t.Fatal(err)
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			next := convergenceConfig("after")
			if err := next.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/after/v1"); err != nil {
				t.Fatal(err)
			}
			after, err := store.Commit(before, next)
			if err != nil {
				t.Fatal(err)
			}
			files := []struct {
				name string
				path string
				want transaction.FileSnapshot
			}{
				{"config", path, before.Config},
				{"backup", path + ".bak", before.Backup},
				{"checkpoint", path + ".verified.json", before.Verified},
				{"forwarding", store.forwardingPath(), before.Forwarding},
				{"forwarding-backup", store.forwardingPath() + ".bak", before.ForwardingBackup},
			}
			for _, index := range test.changed {
				file := &files[index]
				if err := os.WriteFile(file.path, []byte("newer "+file.name), 0o600); err != nil {
					t.Fatal(err)
				}
				file.want, err = transaction.CaptureFileSnapshot(file.path)
				if err != nil {
					t.Fatal(err)
				}
			}
			err = store.RestoreSnapshot(before, after)
			if err == nil || !strings.Contains(err.Error(), "postimage changed") {
				t.Fatalf("restore must report the conflict: %v", err)
			}
			for _, index := range test.changed {
				if !strings.Contains(err.Error(), files[index].path) {
					t.Errorf("restore lost conflict for %s: %v", files[index].path, err)
				}
			}
			for _, file := range files {
				requireRestoredFileSnapshot(t, file.path, file.want)
			}
		})
	}
}

func requireRestoredFileSnapshot(t *testing.T, path string, want transaction.FileSnapshot) {
	t.Helper()
	got, err := transaction.CaptureFileSnapshot(path)
	if err != nil || !got.Equal(want) {
		t.Errorf("%s after restore: exists=%t digest=%s mode=%o error=%v; want exists=%t digest=%s mode=%o", path, got.Exists, got.SHA256, got.Mode, err, want.Exists, want.SHA256, want.Mode)
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
	if err := store.SaveVerifiedCheckpoint(t.Context(), beforeConfig, []string{ClientCodex}); err != nil {
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

func TestForwardingCommitCompensatesEveryOwnedPhase(t *testing.T) {
	for _, phase := range []string{"backup", "config", "forwarding-backup", "forwarding", "checkpoint"} {
		t.Run(phase, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
			cfg := convergenceConfig("current")
			if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/first/v1"); err != nil {
				t.Fatal(err)
			}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/second/v1"); err != nil {
				t.Fatal(err)
			}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
				t.Fatal(err)
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			next := cfg.Clone()
			if err := next.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/next/v1"); err != nil {
				t.Fatal(err)
			}
			account := next.Accounts["current"]
			account.Label = "Renamed presentation"
			next.Accounts["current"] = account
			paths := map[string]string{
				"backup": store.Path() + ".bak", "config": store.Path(),
				"forwarding-backup": store.forwardingPath() + ".bak",
				"forwarding":        store.forwardingPath(), "checkpoint": store.Path() + ".verified.json",
			}
			failure := errors.New("injected " + phase + " failure")
			originalWrite, originalRemove := writeConfigurationFileIfUnchanged, removeConfigurationFileIfUnchanged
			t.Cleanup(func() {
				writeConfigurationFileIfUnchanged, removeConfigurationFileIfUnchanged = originalWrite, originalRemove
			})
			writeConfigurationFileIfUnchanged = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
				if path == paths[phase] {
					return transaction.FileSnapshot{}, failure
				}
				return originalWrite(path, expected, data, mode)
			}
			removeConfigurationFileIfUnchanged = func(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
				if path == paths[phase] {
					return transaction.FileSnapshot{}, failure
				}
				return originalRemove(path, expected)
			}
			if _, err := store.Commit(before, next); !errors.Is(err, failure) {
				t.Fatalf("commit did not reach its %s failure: %v", phase, err)
			}
			after, err := store.CaptureSnapshot()
			if err != nil || !before.equal(after) {
				t.Fatalf("failed %s write left owned changes: %v", phase, err)
			}
		})
	}
}
