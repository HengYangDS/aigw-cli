package artifact

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyChecksumSkipsMalformedAndMismatchedLines(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	content := []byte("archive-bytes")
	if err := os.WriteFile(archivePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte("archive-bytes")))
	checksums := strings.Join([]string{
		"not-enough-fields",
		"nothex-nothex-nothex  " + archiveName,
		strings.Repeat("a", 64) + "  other-name",
		"./" + sum + "  should-not-match", // wrong field order, ignored
		sum + "  ./" + archiveName,
	}, "\n")
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0o600); err != nil {
		t.Fatal(err)
	}
	if digest, err := VerifyChecksum(archivePath, checksumsPath, archiveName); err != nil || digest != sum {
		t.Fatalf("digest = %q, error = %v; want %q", digest, err, sum)
	}
}

func TestVerifyChecksumRejectsDuplicateEntry(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("archive-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte("archive-bytes")))
	checksums := sum + "  " + archiveName + "\n" + sum + "  " + archiveName + "\n"
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChecksum(archivePath, checksumsPath, archiveName); err == nil || !strings.Contains(err.Error(), "duplicate checksum entry") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsHashMismatch(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("archive-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChecksum(archivePath, checksumsPath, archiveName); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsUnreadableArchiveContent(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.Mkdir(archivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChecksum(archivePath, checksumsPath, archiveName); err == nil || !strings.Contains(err.Error(), "hash update archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestIsSHA256RejectsMalformedValues(t *testing.T) {
	if isSHA256(strings.Repeat("a", 63)) {
		t.Fatal("isSHA256 accepted a short value")
	}
	if isSHA256(strings.Repeat("g", 64)) {
		t.Fatal("isSHA256 accepted a non-hex value")
	}
	if !isSHA256(strings.Repeat("a", 64)) {
		t.Fatal("isSHA256 rejected a valid value")
	}
	if !isSHA256(strings.Repeat("F", 64)) {
		t.Fatal("isSHA256 rejected uppercase hex")
	}
}

func TestVerifyChecksumRejectsInvalidArchiveName(t *testing.T) {
	directory := t.TempDir()
	if _, err := VerifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), "nested/name"); err == nil {
		t.Fatal("verifyChecksum accepted a nested archive name")
	}
	if _, err := VerifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), ""); err == nil {
		t.Fatal("verifyChecksum accepted an empty archive name")
	}
}

func TestVerifyChecksumRejectsMissingChecksumsFile(t *testing.T) {
	directory := t.TempDir()
	if _, err := VerifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "missing.txt"), "a"); err == nil || !strings.Contains(err.Error(), "read checksums") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsMissingEntry(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte("deadbeef  other-name\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), "a"); err == nil || !strings.Contains(err.Error(), "checksum entry missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsUnreadableArchive(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(strings.Repeat("a", 64)+"  a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), "a"); err == nil || !strings.Contains(err.Error(), "open update archive") {
		t.Fatalf("error = %v", err)
	}
}
