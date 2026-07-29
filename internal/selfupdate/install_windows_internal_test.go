//go:build windows

package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPortableArchiveWindowsPropagatesHelperWriteFailure(t *testing.T) {
	directory := t.TempDir()
	archivePath, _ := writeWindowsPortableArchiveForTest(t, directory)
	executable := filepath.Join(directory, "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(executable+".update.cmd", 0o700); err != nil {
		t.Fatal(err)
	}

	u := Updater{GOOS: "windows", GOARCH: "amd64", Executable: executable}
	message, scheduled, err := u.installPortableArchive(archivePath, "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "write Windows update helper") {
		t.Fatalf("error = %v", err)
	}
	if message != "" || scheduled {
		t.Fatalf("message = %q, scheduled = %v", message, scheduled)
	}
	staged, readErr := os.ReadFile(executable + ".update")
	if readErr != nil || string(staged) != "windows-binary" {
		t.Fatalf("staged=%q err=%v", staged, readErr)
	}
}

func TestUpdateCandidateWindowsPropagatesHelperWriteFailure(t *testing.T) {
	directory := t.TempDir()
	archivePath, checksumsPath := writeWindowsPortableArchiveForTest(t, directory)
	executable := filepath.Join(directory, "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(executable+".update.cmd", 0o700); err != nil {
		t.Fatal(err)
	}

	u := Updater{GOOS: "windows", GOARCH: "amd64", Executable: executable}
	message, err := u.UpdateCandidate(context.Background(), "v1.0.0", CandidateArchive{
		ArchivePath:   archivePath,
		ChecksumsPath: checksumsPath,
	})
	if err == nil || !strings.Contains(err.Error(), "write Windows update helper") {
		t.Fatalf("error = %v", err)
	}
	if message != "" {
		t.Fatalf("message = %q", message)
	}
}

func TestRollbackWindowsPropagatesStagingCollision(t *testing.T) {
	executable := writeWindowsRollbackPairForTest(t)
	if err := os.Mkdir(windowsRollbackStagePath(executable), 0o700); err != nil {
		t.Fatal(err)
	}

	u := Updater{Channel: ChannelPortable, GOOS: "windows", Executable: executable}
	message, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stage Windows AIGW rollback") {
		t.Fatalf("error = %v", err)
	}
	if message != "" {
		t.Fatalf("message = %q", message)
	}
}

func TestRollbackWindowsCleansStageAfterHelperWriteFailure(t *testing.T) {
	executable := writeWindowsRollbackPairForTest(t)
	if err := os.Mkdir(executable+".rollback.cmd", 0o700); err != nil {
		t.Fatal(err)
	}

	u := Updater{Channel: ChannelPortable, GOOS: "windows", Executable: executable}
	message, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "write Windows AIGW rollback helper") {
		t.Fatalf("error = %v", err)
	}
	if message != "" {
		t.Fatalf("message = %q", message)
	}
	if _, statErr := os.Stat(windowsRollbackStagePath(executable)); !os.IsNotExist(statErr) {
		t.Fatalf("staged rollback binary was not cleaned up: %v", statErr)
	}
}

func TestDetectChannelWindowsPathHeuristic(t *testing.T) {
	previous := InstallChannel
	t.Cleanup(func() { InstallChannel = previous })
	InstallChannel = "bogus"
	t.Setenv("AIGW_INSTALL_CHANNEL", "")

	if got := detectChannel(`C:\Program Files\AIGW\aigw.exe`); got != ChannelMSI {
		t.Fatalf("channel = %q, want msi", got)
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
	archive := zipArchive(t, []byte("aigw_1.2.3_windows_amd64/aigw.exe"), []byte("windows-binary"))
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

func writeWindowsRollbackPairForTest(t *testing.T) string {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	return executable
}
