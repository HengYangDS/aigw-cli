// Package readiness validates release identity and admission evidence.
package readiness

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// ValidateVersion admits valid prereleases while GA native signing remains
// unavailable. Build metadata does not change a release's stability.
func ValidateVersion(version string) error {
	parsed, err := semver.StrictNewVersion(version)
	if err != nil {
		return fmt.Errorf("invalid release version %q: %w", version, err)
	}
	if parsed.Prerelease() != "" {
		return nil
	}
	return errors.New("GA release requires protected macOS notarization, Windows Authenticode, and artifact signature verification")
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

// ReadDeliveryVersion selects the declared release or an exact-source local identity.
func ReadDeliveryVersion(root string, local bool) (string, error) {
	version, err := ReadProductVersion(root)
	if err != nil || !local {
		return version, err
	}
	output, err := exec.Command("git", "--no-replace-objects", "-C", root, "show", "-s", "--format=%H %ct", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("read local delivery source: %w", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) != 2 {
		return "", errors.New("local delivery requires a commit and source epoch")
	}
	return LocalVersion(version, fields[0], fields[1])
}

// LocalVersion orders a source build after its baseline and before the next release.
func LocalVersion(base, commit, epoch string) (string, error) {
	version, err := semver.StrictNewVersion(base)
	if err != nil {
		return "", err
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{40}(?:[0-9a-f]{24})?$`, commit); !matched {
		return "", errors.New("local delivery requires an exact Git commit")
	}
	instant, err := ParseEpoch(epoch)
	if err != nil {
		return "", err
	}
	prerelease := "local." + strconv.FormatInt(instant.Unix(), 10)
	if version.Prerelease() == "" {
		next := version.IncPatch()
		version = &next
	} else {
		prerelease = version.Prerelease() + "." + prerelease
	}
	local, err := version.SetPrerelease(prerelease)
	if err != nil {
		return "", err
	}
	local, err = local.SetMetadata("local." + commit)
	return local.String(), err
}
