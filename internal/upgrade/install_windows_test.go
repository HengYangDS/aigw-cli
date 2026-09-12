//go:build windows

package upgrade

import (
	"aigw-cli/internal/process"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/robustio"
	"golang.org/x/sys/windows"
)

func TestCandidateVerificationWaitsForReleasedWindowsFile(t *testing.T) {
	root := t.TempDir()
	runner := &lockedCandidateRunner{testing: t}
	updater := Updater{Executable: filepath.Join(root, "aigw.exe"), Runner: runner}
	err := updater.verifyCandidateProgram(t.Context(), []byte("candidate"), "1.2.3")
	if runner.released != nil {
		<-runner.released
	}
	if err != nil {
		t.Fatalf("temporary executable lock interrupted candidate verification: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("verification retained owned candidate files: %v, %v", entries, err)
	}
}

type lockedCandidateRunner struct {
	testing  *testing.T
	released chan struct{}
}

func (r *lockedCandidateRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	release := holdWindowsFile(r.testing, plan.Executable)
	r.released = make(chan struct{})
	go func() {
		defer close(r.released)
		time.Sleep(100 * time.Millisecond)
		release()
	}()
	return []byte("aigw version 1.2.3\n"), nil
}

func holdWindowsFile(t *testing.T, path string) func() {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	release := sync.OnceFunc(func() {
		if err := windows.CloseHandle(handle); err != nil {
			t.Errorf("release file handle: %v", err)
		}
	})
	t.Cleanup(release)
	return release
}

func TestReplacementHandlesWindowsExecutableLocks(t *testing.T) {
	for _, test := range []struct {
		name, locked        string
		transient, rollback bool
		current, previous   string
	}{
		{"update/current/transient", "aigw.exe", true, false, "next", "current"},
		{"update/previous/transient", ".aigw.previous.exe", true, false, "next", "current"},
		{"rollback/current/transient", "aigw.exe", true, true, "previous", "current"},
		{"rollback/previous/transient", ".aigw.previous.exe", true, true, "previous", "current"},
		{"update/current/persistent", "aigw.exe", false, false, "current", "previous"},
		{"update/previous/persistent", ".aigw.previous.exe", false, false, "current", "previous"},
		{"rollback/current/persistent", "aigw.exe", false, true, "current", "previous"},
		{"rollback/previous/persistent", ".aigw.previous.exe", false, true, "current", "previous"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			executable := filepath.Join(root, "aigw.exe")
			for path, value := range map[string]string{executable: "current", rollbackPath(executable): "previous"} {
				if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			locked := filepath.Join(root, test.locked)
			release := holdWindowsFile(t, locked)
			var lockedRename *os.LinkError
			if err := os.Rename(executable, rollbackPath(executable)); !errors.As(err, &lockedRename) {
				t.Fatalf("fixture did not establish a locked replacement: %v", err)
			}
			if test.transient {
				timer := time.AfterFunc(100*time.Millisecond, release)
				t.Cleanup(func() { timer.Stop() })
			}
			updater := Updater{Executable: executable}
			var err error
			if test.rollback {
				_, err = updater.Rollback(t.Context())
			} else {
				err = updater.replacePortableBinary(t.Context(), []byte("next"))
			}
			release()
			if (err == nil) != test.transient || !test.transient && !errors.Is(err, lockedRename.Err) {
				t.Fatalf("transient=%t error=%v", test.transient, err)
			}
			for path, want := range map[string]string{executable: test.current, rollbackPath(executable): test.previous} {
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Fatalf("replacement file %s = %q, %v; want %q", path, got, err, want)
				}
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 2 {
				t.Fatalf("replacement retained staging: %v, %v", entries, err)
			}
		})
	}
}

func TestCandidateActivationHandlesWindowsFileLocks(t *testing.T) {
	for _, test := range []struct {
		name      string
		transient bool
		files     map[string]string
	}{
		{"released", true, map[string]string{"aigw.exe": "next", ".aigw.previous.exe": "current"}},
		{"persistent", false, map[string]string{"aigw.exe": "current", "candidate.exe": "next"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			current := filepath.Join(root, "aigw.exe")
			candidate := filepath.Join(root, "candidate.exe")
			for path, value := range map[string]string{current: "current", candidate: "next", rollbackPath(current): "older"} {
				if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			release := holdWindowsFile(t, candidate)
			if err := os.Rename(candidate, candidate+".probe"); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
				t.Fatalf("candidate source lock was not established: %v", err)
			}
			if test.transient {
				timer := time.AfterFunc(100*time.Millisecond, release)
				t.Cleanup(func() { timer.Stop() })
			}
			err := commitProgramReplacement(candidate, current, rollbackPath(current), robustio.Rename)
			release()
			if (err == nil) != test.transient || !test.transient && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
				t.Fatalf("candidate activation: transient=%t error=%v", test.transient, err)
			}
			for name, want := range test.files {
				if got, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(got) != want {
					t.Fatalf("activation file %s = %q, %v; want %q", name, got, err, want)
				}
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != len(test.files) {
				t.Fatalf("activation left unexplained files: %v, %v", entries, err)
			}
		})
	}
}

func TestWindowsPortableUpdateDoesNotCreateCommandScripts(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	updater := Updater{Executable: executable, GOOS: "windows", GOARCH: "amd64", Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
	archivePath, _ := writeWindowsPortableArchiveForTest(t, root)
	if err := updater.installPortableArchive(t.Context(), archivePath, filepath.Join(root, "checksums.txt"), "1.2.3"); err != nil {
		t.Fatalf("installPortableArchive() = %v", err)
	}
	if got, err := os.ReadFile(executable); err != nil || string(got) != "windows-binary" {
		t.Fatalf("updated executable = %q, %v", got, err)
	}
	if got, err := os.ReadFile(rollbackPath(executable)); err != nil || string(got) != "current" {
		t.Fatalf("rollback executable = %q, %v", got, err)
	}
	if _, err := updater.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".update.cmd", ".rollback.cmd"} {
		if _, err := os.Stat(executable + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("generated command script %s exists: %v", suffix, err)
		}
	}
}

func TestRollbackPathWindowsExecutableBaseName(t *testing.T) {
	if got := rollbackPath("aigw.exe"); got != ".aigw.previous.exe" {
		t.Fatalf("rollbackPath = %q", got)
	}
}

func writeWindowsPortableArchiveForTest(t *testing.T, directory string) (string, string) {
	t.Helper()
	archiveName := "aigw_1.2.3_windows_amd64.zip"
	archivePath := filepath.Join(directory, archiveName)
	archive := zipArchive(t, "aigw_1.2.3_windows_amd64/aigw.exe", []byte("windows-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	checksum := fileSHA256ForTest(t, archivePath)
	if err := os.WriteFile(checksumsPath, []byte(checksum+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return archivePath, checksumsPath
}
