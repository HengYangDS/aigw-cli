//go:build windows

package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsPortableUpdateDoesNotCreateCommandScripts(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	updater := Updater{Executable: executable, GOOS: "windows", GOARCH: "amd64"}
	archivePath, _ := writeWindowsPortableArchiveForTest(t, root)
	if _, scheduled, err := updater.installPortableArchive(archivePath, "v1.2.3"); err != nil || scheduled {
		t.Fatalf("installPortableArchive() = scheduled %v, err %v", scheduled, err)
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
