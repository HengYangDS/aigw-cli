package selfupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateCandidateRequiresBothPaths(t *testing.T) {
	u := Updater{}
	cases := []CandidateArchive{
		{ArchivePath: "", ChecksumsPath: "b"},
		{ArchivePath: "a", ChecksumsPath: ""},
		{ArchivePath: " ", ChecksumsPath: " "},
	}
	for _, candidate := range cases {
		if _, err := u.UpdateCandidate(context.Background(), "0.1.0", candidate); err == nil || !strings.Contains(err.Error(), "requires both archive and checksums paths") {
			t.Fatalf("candidate=%#v error = %v", candidate, err)
		}
	}
}

func TestUpdateCandidateRejectsInvalidArchiveName(t *testing.T) {
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	_, err := u.UpdateCandidate(context.Background(), "0.1.0", CandidateArchive{ArchivePath: "aigw_bad_darwin_arm64.tar.gz", ChecksumsPath: "checksums.txt"})
	if err == nil {
		t.Fatal("UpdateCandidate accepted a malformed archive name")
	}
}

func TestUpdateCandidateRejectsInvalidCurrentVersion(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "aigw_1.2.3_darwin_arm64.tar.gz")
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	_, err := u.UpdateCandidate(context.Background(), "not-a-version", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: filepath.Join(directory, "checksums.txt")})
	if err == nil {
		t.Fatal("UpdateCandidate accepted a malformed current version")
	}
}

func TestUpdateCandidateReportsAlreadyMatchingVersion(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "aigw_1.2.3_darwin_arm64.tar.gz")
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	message, err := u.UpdateCandidate(context.Background(), "v1.2.3", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: filepath.Join(directory, "checksums.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "already matches") {
		t.Fatalf("message = %q", message)
	}
}

func TestUpdateCandidateRefusesOlderVersion(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "aigw_1.2.3_darwin_arm64.tar.gz")
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	_, err := u.UpdateCandidate(context.Background(), "v1.3.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: filepath.Join(directory, "checksums.txt")})
	if err == nil || !strings.Contains(err.Error(), "older") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateCandidateRejectsChecksumFailureBeforeExtraction(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("not-a-real-archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	_, err := u.UpdateCandidate(context.Background(), "v1.0.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateCandidateRejectsExtractionFailureAfterChecksum(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	archive := []byte("not-a-real-tar-gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%x  %s\n", sum, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64"}
	_, err := u.UpdateCandidate(context.Background(), "v1.0.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
	if err == nil {
		t.Fatal("UpdateCandidate accepted an unreadable archive")
	}
}

func TestUpdateCandidateReplacesBinaryOnSuccess(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	archive := tarGzForTest(t, "aigw_1.2.3_darwin_arm64/aigw", []byte("new-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%x  %s\n", sum, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: executable}
	message, err := u.UpdateCandidate(context.Background(), "v1.0.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "v1.2.3") {
		t.Fatalf("message = %q", message)
	}
	got, err := os.ReadFile(executable)
	if err != nil || string(got) != "new-binary" {
		t.Fatalf("binary=%q err=%v", got, err)
	}
}

func TestUpdateCandidateRejectsReplaceBinaryFailure(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	archive := tarGzForTest(t, "aigw_1.2.3_darwin_arm64/aigw", []byte("new-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%x  %s\n", sum, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	// No executable exists at this path, so replacePortableBinary must fail
	// while preserving the previous binary.
	u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(directory, "missing", "aigw")}
	if _, err := u.UpdateCandidate(context.Background(), "v1.0.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath}); err == nil {
		t.Fatal("UpdateCandidate accepted a missing executable")
	}
}
