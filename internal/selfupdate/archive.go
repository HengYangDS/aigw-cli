package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func normalizeVersion(value string) string { return strings.TrimPrefix(strings.TrimSpace(value), "v") }

func compareVersions(left, right string) (int, error) {
	leftParts, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	for index := 0; index < 3; index++ {
		if leftParts.core[index] < rightParts.core[index] {
			return -1, nil
		}
		if leftParts.core[index] > rightParts.core[index] {
			return 1, nil
		}
	}
	if leftParts.pre == rightParts.pre {
		return 0, nil
	}
	if leftParts.pre == "" {
		return 1, nil
	}
	if rightParts.pre == "" {
		return -1, nil
	}
	return comparePrerelease(leftParts.pre, rightParts.pre)
}

type parsedVersion struct {
	core [3]uint64
	pre  string
}

func parseVersion(value string) (parsedVersion, error) {
	value = normalizeVersion(value)
	if value == "" {
		return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
	}
	core, pre, _ := strings.Cut(value, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
	}
	parsed := parsedVersion{pre: pre}
	for index, part := range parts {
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
		}
		parsed.core[index] = number
	}
	if pre != "" && !validPrerelease(pre) {
		return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
	}
	return parsed, nil
}

func comparePrerelease(left, right string) (int, error) {
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	limit := len(leftParts)
	if len(rightParts) < limit {
		limit = len(rightParts)
	}
	for index := 0; index < limit; index++ {
		leftPart, rightPart := leftParts[index], rightParts[index]
		leftNumber, leftNumeric := prereleaseNumber(leftPart)
		rightNumber, rightNumeric := prereleaseNumber(rightPart)
		switch {
		case leftNumeric && rightNumeric:
			if leftNumber < rightNumber {
				return -1, nil
			}
			if leftNumber > rightNumber {
				return 1, nil
			}
		case leftNumeric:
			return -1, nil
		case rightNumeric:
			return 1, nil
		case leftPart < rightPart:
			return -1, nil
		case leftPart > rightPart:
			return 1, nil
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1, nil
	}
	if len(leftParts) > len(rightParts) {
		return 1, nil
	}
	return 0, nil
}

func validPrerelease(value string) bool {
	for _, part := range strings.Split(value, ".") {
		if part == "" {
			return false
		}
		for _, character := range part {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
		if _, numeric := prereleaseNumber(part); numeric {
			continue
		}
		if allDigits(part) {
			return false
		}
	}
	return true
}

func prereleaseNumber(value string) (uint64, bool) {
	number, err := strconv.ParseUint(value, 10, 64)
	return number, err == nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func verifyChecksum(archivePath, checksumPath, archiveName string) error {
	if filepath.Base(archiveName) != archiveName || archiveName == "" {
		return fmt.Errorf("invalid release asset name %q", archiveName)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !isSHA256(fields[0]) {
			continue
		}
		name := strings.TrimPrefix(fields[1], "./")
		if name != archiveName {
			continue
		}
		if expected != "" {
			return fmt.Errorf("duplicate checksum entry for %s", archiveName)
		}
		expected = strings.ToLower(fields[0])
	}
	if expected == "" {
		return fmt.Errorf("checksum entry missing for %s", archiveName)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open update archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return fmt.Errorf("hash update archive: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func extractBinary(path, expectedPath string) ([]byte, error) {
	if strings.HasSuffix(path, ".zip") {
		return extractZipBinary(path, expectedPath)
	}
	archive, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open update archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open gzip archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	reader := tar.NewReader(gz)
	var binary []byte
	matches := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar archive: %w", err)
		}
		if filepath.Clean(header.Name) != expectedPath || header.Typeflag != tar.TypeReg {
			continue
		}
		matches++
		if matches > 1 {
			return nil, fmt.Errorf("update archive contains multiple expected AIGW binaries")
		}
		binary, err = io.ReadAll(io.LimitReader(reader, 128<<20))
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("expected AIGW binary is missing from update archive")
	}
	return binary, nil
}

func extractZipBinary(path, expectedPath string) ([]byte, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	var binary []byte
	matches := 0
	for _, file := range archive.File {
		if filepath.Clean(file.Name) != expectedPath || file.FileInfo().IsDir() {
			continue
		}
		matches++
		if matches > 1 {
			return nil, fmt.Errorf("update archive contains multiple expected AIGW binaries")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open AIGW binary in zip: %w", err)
		}
		binary, err = io.ReadAll(io.LimitReader(reader, 128<<20))
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close AIGW binary in zip: %w", closeErr)
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("expected AIGW binary is missing from update archive")
	}
	return binary, nil
}
