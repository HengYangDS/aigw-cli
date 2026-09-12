package upgrade

import (
	"aigw-cli/internal/process"
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

func TestUpdateCandidateSameVersionRequiresVerifiedProgramIdentity(t *testing.T) {
	for _, test := range []struct {
		name        string
		program     string
		missing     string
		badChecksum bool
		canceled    bool
		wantError   string
	}{
		{name: "exact program", program: "current"},
		{name: "different program", program: "different", wantError: "different program bytes"},
		{name: "missing archive", program: "current", missing: "archive", wantError: "open"},
		{name: "missing current", program: "current", missing: "current", wantError: "read current AIGW executable"},
		{name: "invalid checksum", program: "current", badChecksum: true, wantError: "checksum"},
		{name: "canceled", program: "current", canceled: true, wantError: "context canceled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
			archivePath := filepath.Join(directory, archiveName)
			checksumsPath := filepath.Join(directory, "checksums.txt")
			archive := tarGzForTest(t, "aigw_1.2.3_darwin_arm64/aigw", []byte(test.program))
			digest := fmt.Sprintf("%x", sha256.Sum256(archive))
			if test.badChecksum {
				digest = strings.Repeat("0", 64)
			}
			executable := filepath.Join(directory, "aigw")
			previous := rollbackPath(executable)
			for path, content := range map[string][]byte{
				executable: []byte("current"), previous: []byte("retained"),
				archivePath: archive, checksumsPath: []byte(digest + "  " + archiveName + "\n"),
			} {
				if err := os.WriteFile(path, content, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if test.missing != "" {
				path := map[string]string{"archive": archivePath, "current": executable}[test.missing]
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.canceled {
				cancel()
			}
			u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: executable, Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
			message, err := u.UpdateCandidate(ctx, "v1.2.3", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
			if test.wantError == "" {
				if err != nil || !strings.Contains(message, "already matches") {
					t.Fatalf("exact program = %q, %v", message, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("unverified candidate = %q, %v; want %q", message, err, test.wantError)
			}
			if test.missing != "current" {
				if current, readErr := os.ReadFile(executable); readErr != nil || string(current) != "current" {
					t.Fatalf("current program changed: %q, %v", current, readErr)
				}
			}
			if retained, readErr := os.ReadFile(previous); readErr != nil || string(retained) != "retained" {
				t.Fatalf("rollback program changed: %q, %v", retained, readErr)
			}
		})
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

func TestUpdateCandidateUsesWindowsExecutableName(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_windows_amd64.zip"
	archivePath := filepath.Join(directory, archiveName)
	archive := zipArchive(t, "aigw_1.2.3_windows_amd64/aigw.exe", []byte("windows-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(fileSHA256ForTest(t, archivePath)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "aigw.exe")
	if err := os.WriteFile(executable, []byte("old-binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "windows", GOARCH: "amd64", Executable: executable, Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
	message, err := u.UpdateCandidate(context.Background(), "v1.0.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksumsPath})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "v1.2.3") {
		t.Fatalf("message = %q", message)
	}
	if got, err := os.ReadFile(executable); err != nil || string(got) != "windows-binary" {
		t.Fatalf("binary=%q err=%v", got, err)
	}
}

func TestUpdateCandidateKeepsCurrentAndRollbackUntilProgramIsVerified(t *testing.T) {
	for _, test := range []struct {
		name     string
		runner   process.CaptureRunner
		canceled bool
	}{
		{name: "invalid executable"},
		{name: "wrong version", runner: &recordingRunner{output: []byte("aigw version 1.2.2\n")}},
		{name: "startup failure", runner: &recordingRunner{err: os.ErrPermission}},
		{name: "canceled update", canceled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			executable := filepath.Join(directory, "aigw")
			previous := rollbackPath(executable)
			for path, content := range map[string]string{executable: "current", previous: "retained"} {
				if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
			archive := tarGzForTest(t, "aigw_1.2.3_darwin_arm64/aigw", []byte("not an executable"))
			archivePath := filepath.Join(t.TempDir(), archiveName)
			if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
				t.Fatal(err)
			}
			checksums := filepath.Join(filepath.Dir(archivePath), "checksums.txt")
			if err := os.WriteFile(checksums, []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)), 0o600); err != nil {
				t.Fatal(err)
			}
			updater := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: executable, Runner: test.runner}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.canceled {
				cancel()
			}
			if _, err := updater.UpdateCandidate(ctx, "1.2.0", CandidateArchive{ArchivePath: archivePath, ChecksumsPath: checksums}); err == nil {
				t.Fatal("update accepted a candidate without verified startup and version")
			}
			for path, expected := range map[string]string{executable: "current", previous: "retained"} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != expected {
					t.Fatalf("preserved program %s = %q, %v", path, got, err)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 2 {
				t.Fatalf("candidate validation left staging residue: %v, %v", entries, err)
			}
		})
	}
}
