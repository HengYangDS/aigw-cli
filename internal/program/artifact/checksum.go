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
func VerifyChecksum(archivePath, checksumPath, archiveName string) (string, error) {
	if filepath.Base(archiveName) != archiveName || archiveName == "" {
		return "", fmt.Errorf("invalid release asset name %q", archiveName)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	digests, err := parseChecksums(data)
	if err != nil {
		return "", err
	}
	expected, ok := digests[archiveName]
	if !ok {
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

// parseChecksums reads a checksum manifest. Each line holds an artifact's SHA-256
// in hexadecimal and its name, which may start with "./". It returns each name's
// lowercase digest and refuses any other line and a repeated name.
func parseChecksums(manifest []byte) (map[string]string, error) {
	digests := map[string]string{}
	for line := range strings.Lines(string(manifest)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || !isSHA256(fields[0]) {
			return nil, errors.New("invalid checksum manifest format")
		}
		name := strings.TrimPrefix(fields[1], "./")
		if _, duplicate := digests[name]; duplicate {
			return nil, fmt.Errorf("duplicate checksum entry for %s", name)
		}
		digests[name] = strings.ToLower(fields[0])
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
