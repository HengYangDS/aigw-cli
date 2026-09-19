// Package readiness validates release identity and admission evidence.
package readiness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// ParseEpoch returns the canonical UTC release instant.
func ParseEpoch(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || epoch < 0 {
		return time.Time{}, errors.New("epoch must be a non-negative integer")
	}
	return time.Unix(epoch, 0).UTC(), nil
}

// ValidateToolchain verifies that the running Go toolchain matches go.mod.
func ValidateToolchain(modulePath, actual string) error {
	data, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	expected := ""
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			expected = "go" + fields[1]
			break
		}
	}
	if expected == "" {
		return errors.New("release toolchain: go.mod has no Go version")
	}
	if actual != expected {
		return fmt.Errorf("release toolchain: expected %s, found %s", expected, actual)
	}
	return nil
}

// ValidateVersion checks strict SemVer syntax without claiming product or artifact readiness.
func ValidateVersion(version string) error {
	if _, err := semver.StrictNewVersion(version); err != nil {
		return fmt.Errorf("invalid release version %q: %w", version, err)
	}
	return nil
}

// ReadProductVersion reads the canonical VERSION carrier and validates strict SemVer.
func ReadProductVersion(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return "", fmt.Errorf("read VERSION: %w", err)
	}
	version := strings.TrimSpace(string(data))
	if _, err := semver.StrictNewVersion(version); err != nil {
		return "", fmt.Errorf("VERSION contains invalid release version %q: %w", version, err)
	}
	return version, nil
}
