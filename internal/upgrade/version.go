package upgrade

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

func normalizeVersion(value string) string { return strings.TrimPrefix(strings.TrimSpace(value), "v") }

func parseVersion(value string) (*semver.Version, error) {
	version, err := semver.StrictNewVersion(normalizeVersion(value))
	if err != nil {
		return nil, fmt.Errorf("invalid release version %q: %w", value, err)
	}
	return version, nil
}

func compareVersions(left, right string) (int, error) {
	leftVersion, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightVersion, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	return leftVersion.Compare(rightVersion), nil
}
