package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VerifyChecksum verifies one manifest entry and returns the authenticated archive digest.
func VerifyChecksum(archivePath, checksumPath, archiveName string) (string, error) {
	if filepath.Base(archiveName) != archiveName || archiveName == "" {
		return "", fmt.Errorf("invalid release asset name %q", archiveName)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !isSHA256(fields[0]) {
			continue
		}
		name := strings.TrimPrefix(fields[1], "./")
		if name != archiveName {
			continue
		}
		if expected != "" {
			return "", fmt.Errorf("duplicate checksum entry for %s", archiveName)
		}
		expected = strings.ToLower(fields[0])
	}
	if expected == "" {
		return "", fmt.Errorf("checksum entry missing for %s", archiveName)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open update archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return "", fmt.Errorf("hash update archive: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return "", fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	return actual, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}
