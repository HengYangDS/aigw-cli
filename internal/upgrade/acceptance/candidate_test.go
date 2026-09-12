package acceptance_test

import (
	"aigw-cli/internal/upgrade"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateCandidateInstallsExplicitArchiveWithoutReleaseSource(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	directory := t.TempDir()
	archivePath := filepath.Join(directory, archiveName)
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	message, err := u.UpdateCandidate(context.Background(), "0.1.0", upgrade.CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "candidate-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
	}
	if !strings.Contains(message, "verified local candidate") || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("message = %q", message)
	}
	if len(runner.calls) != 1 || len(runner.calls[0]) != 2 || !filepath.IsAbs(runner.calls[0][0]) || runner.calls[0][1] != "--version" {
		t.Fatalf("local candidate must only execute its staged program: %v", runner.calls)
	}
}

func TestUpdateCandidateRejectsChecksumMismatchWithoutNetworkFallback(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("candidate-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	directory := t.TempDir()
	archivePath := filepath.Join(directory, archiveName)
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.UpdateCandidate(context.Background(), "0.1.0", upgrade.CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	if got, err := os.ReadFile(binary); err != nil || string(got) != "old-binary" {
		t.Fatalf("binary=%q error=%v", got, err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("local checksum failure invoked a release client: %v", runner.calls)
	}
}
