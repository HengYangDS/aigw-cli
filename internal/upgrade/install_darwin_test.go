//go:build darwin

package upgrade

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestInstallationObservationSeparatesCommandAndPayload(t *testing.T) {
	root := t.TempDir()
	payload, command := filepath.Join(root, "payload"), filepath.Join(root, "aigw")
	if err := os.WriteFile(payload, []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(payload, command); err != nil {
		t.Fatal(err)
	}
	result, err := InspectInstallation(command, "test")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(payload)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommandPath != command || result.Payload.Path != resolved {
		t.Fatalf("command and payload were conflated: %+v", result)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), RollbackPath(command)); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectInstallation(command, "test"); err == nil || !strings.Contains(err.Error(), "retained portable program") {
		t.Fatalf("broken retained link accepted as absent: %v", err)
	}
}

func makeImmutable(t *testing.T, path string) {
	t.Helper()
	if err := unix.Chflags(path, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Chflags(path, 0) })
}

func TestUpdatePreservesDownloadAndWorkspaceCleanupFailures(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	failure := errors.New("interrupted asset download")
	u := Updater{
		GOOS: "darwin", GOARCH: "arm64",
		HTTPClient: &http.Client{Transport: githubRoundTripFunc(func(*http.Request) (*http.Response, error) {
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatalf("expected one owned update workspace: %v, %v", entries, err)
			}
			directory := filepath.Join(root, entries[0].Name())
			makeImmutable(t, directory)
			return nil, failure
		})},
	}
	releases := []resolvedRelease{{Source: ReleaseSource{Provider: ReleaseProviderGitHub, Origin: "https://github.test", Repository: "team/product"}, Tag: "v1.0.0"}}
	_, err := u.updateFromResolvedPeers(t.Context(), releases, "0.1.0")
	if !errors.Is(err, failure) || !errors.Is(err, unix.EPERM) {
		t.Fatalf("update discarded download or cleanup failure: %v", err)
	}
}

func TestReplacePortableBinaryPropagatesWriteFailure(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, executable)
	u := Updater{Executable: executable, Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
	if err := u.replacePortableBinary(t.Context(), []byte("new-binary")); err == nil || !strings.Contains(err.Error(), "replace AIGW executable") {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"aigw"}) {
		t.Fatalf("failed replacement retained staging: %v", names)
	}
}

func TestRollbackPropagatesRestoreWriteFailure(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, executable)
	u := Updater{Executable: executable, Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
	if _, err := u.Rollback(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "restore previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackPropagatesBackupReplacementFailure(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	backup := RollbackPath(executable)
	if err := os.WriteFile(backup, []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, backup)
	u := Updater{Executable: executable, Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
	_, err := u.Rollback(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "restore previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(executable)
	if readErr != nil || string(got) != "current" {
		t.Fatalf("current executable was not restored: got=%q err=%v", got, readErr)
	}
}
