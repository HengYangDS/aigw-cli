package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VerifyChecksum verifies one manifest entry and returns the authenticated archive digest.
// It reads only the archive's own line and skips every other line.
func VerifyChecksum(archivePath, checksumPath, archiveName string) (string, error) {
	if filepath.Base(archiveName) != archiveName || archiveName == "" {
		return "", fmt.Errorf("invalid release asset name %q", archiveName)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for line := range strings.Lines(string(data)) {
		digest, name, ok := checksumLine(line)
		if !ok || name != archiveName {
			continue
		}
		if expected != "" {
			return "", fmt.Errorf("duplicate checksum entry for %s", archiveName)
		}
		expected = digest
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

// checksumLine reads one manifest line: an artifact's SHA-256 in hexadecimal and
// its name, which may start with "./". It returns the lowercase digest.
func checksumLine(line string) (digest, name string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 || !isSHA256(fields[0]) {
		return "", "", false
	}
	return strings.ToLower(fields[0]), strings.TrimPrefix(fields[1], "./"), true
}

// parseChecksums reads a release's complete manifest and refuses any line that
// is not a checksum line, and a repeated name.
func parseChecksums(manifest []byte) (map[string]string, error) {
	digests := map[string]string{}
	for line := range strings.Lines(string(manifest)) {
		digest, name, ok := checksumLine(line)
		if !ok {
			return nil, errors.New("invalid checksum manifest format")
		}
		if _, duplicate := digests[name]; duplicate {
			return nil, fmt.Errorf("duplicate checksum entry for %s", name)
		}
		digests[name] = digest
	}
	return digests, nil
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
