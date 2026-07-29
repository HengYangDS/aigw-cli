package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPortableArchiveUsesWindowsBinaryNameWithoutWindowsRuntime(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_windows_amd64.zip"
	archivePath := filepath.Join(directory, archiveName)
	archive := zipArchive(t, []byte("aigw_1.2.3_windows_amd64/aigw.exe"), []byte("windows-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fileSHA256ForTest(t, archivePath)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(sum+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "aigw.exe")
	if err := os.WriteFile(executable, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "windows", GOARCH: "amd64", Executable: executable}
	message, scheduled, err := u.installPortableArchive(archivePath, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if scheduled {
		t.Fatal("installPortableArchive scheduled a Windows replacement outside a Windows runtime")
	}
	if !strings.Contains(message, "v1.2.3") {
		t.Fatalf("message = %q", message)
	}
	got, err := os.ReadFile(executable)
	if err != nil || string(got) != "windows-binary" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func fileSHA256ForTest(t *testing.T, path string) string {
	t.Helper()
	sum, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestInstallPortableArchivePropagatesReplaceFailure(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	archive := tarGzForTest(t, "aigw_1.2.3_darwin_arm64/aigw", []byte("new-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fileSHA256ForTest(t, archivePath)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(sum+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(directory, "missing", "aigw")}
	if _, _, err := u.installPortableArchive(archivePath, "v1.2.3"); err == nil {
		t.Fatal("installPortableArchive accepted a missing executable directory")
	}
}

func TestPortableArchiveNameUsesZipExtensionOnWindows(t *testing.T) {
	if got := portableArchiveName("1.2.3", "windows", "amd64"); got != "aigw_1.2.3_windows_amd64.zip" {
		t.Fatalf("portableArchiveName = %q", got)
	}
	if got := portableArchiveName("1.2.3", "darwin", "arm64"); got != "aigw_1.2.3_darwin_arm64.tar.gz" {
		t.Fatalf("portableArchiveName = %q", got)
	}
}

func TestRollbackDefaultsEmptyChannelToPortable(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", Executable: executable}
	if _, err := u.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackRejectsEmptyExecutable(t *testing.T) {
	u := Updater{Channel: ChannelPortable, Executable: "  "}
	if _, err := u.Rollback(context.Background()); err == nil || !strings.Contains(err.Error(), "AIGW executable path is empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRejectsMissingCurrentExecutable(t *testing.T) {
	directory := t.TempDir()
	u := Updater{Channel: ChannelPortable, Executable: filepath.Join(directory, "missing")}
	if _, err := u.Rollback(context.Background()); err == nil || !strings.Contains(err.Error(), "read current AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRejectsUnreadableBackup(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory at the backup path makes os.ReadFile fail with an error
	// other than os.ErrNotExist.
	if err := os.Mkdir(rollbackPath(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	u := Updater{Channel: ChannelPortable, Executable: executable}
	if _, err := u.Rollback(context.Background()); err == nil || !strings.Contains(err.Error(), "read previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduleWindowsRollbackRejectsUnreadableBackup(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rollbackPath(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsRollback(); err == nil || !strings.Contains(err.Error(), "read previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}
