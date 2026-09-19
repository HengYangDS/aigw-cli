package readiness

import (
	"bufio"
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

var releaseHeading = regexp.MustCompile(`^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$`)

type changelogEntry struct {
	version *semver.Version
	date    time.Time
}

// SelectedReleaseTag resolves the current Forge release tag without treating a
// branch name as a tag.
func SelectedReleaseTag() string {
	if tag := os.Getenv("AIGW_CHANGELOG_RELEASE_TAG"); tag != "" {
		return tag
	}
	if os.Getenv("GITHUB_REF_TYPE") == "tag" {
		return os.Getenv("GITHUB_REF_NAME")
	}
	return os.Getenv("CI_COMMIT_TAG")
}

// ValidateChangelog verifies release chronology and, when selectedTag is set,
// binds the first published release to the exact tag and current Git object.
func ValidateChangelog(root, path, selectedTag string) error {
	path = resolveChangelogPath(root, path)
	entries, err := parseChangelog(path)
	if err != nil {
		return err
	}
	if selectedTag == "" {
		return nil
	}
	version, prefixed := strings.CutPrefix(selectedTag, "v")
	if _, err := semver.StrictNewVersion(version); !prefixed || err != nil {
		return fmt.Errorf("CHANGELOG.md: selected release tag is malformed: %s", selectedTag)
	}
	if entries[0].version.Original() != version {
		return fmt.Errorf("CHANGELOG.md: first published section must identify selected release tag: %s", selectedTag)
	}
	tagCommit, err := gitOutput(root, "rev-parse", "refs/tags/"+selectedTag+"^{}")
	if err != nil {
		return fmt.Errorf("CHANGELOG.md: selected release tag is unavailable: %s", selectedTag)
	}
	head, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil || tagCommit != head {
		return fmt.Errorf("CHANGELOG.md: selected release tag does not identify HEAD: %s", selectedTag)
	}
	return nil
}

// LookupReleaseEpoch returns the exact release heading's UTC Unix timestamp.
func LookupReleaseEpoch(path, version string) (string, bool, error) {
	entries, err := parseChangelog(path)
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.version.Original() == version {
			return strconv.FormatInt(entry.date.Unix(), 10), true, nil
		}
	}
	return "", false, nil
}

func resolveChangelogPath(root, path string) string {
	if path == "" {
		path = "CHANGELOG.md"
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func parseChangelog(path string) ([]changelogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CHANGELOG.md: %w", err)
	}
	defer func() { _ = file.Close() }()
	firstHeading := ""
	entries := []changelogEntry{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if firstHeading == "" && strings.HasPrefix(line, "## ") {
			firstHeading = line
		}
		if !strings.HasPrefix(line, "## [") || line == "## [Unreleased]" {
			continue
		}
		match := releaseHeading.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("CHANGELOG.md: malformed published heading at line %d: %s", lineNumber, line)
		}
		version, err := semver.StrictNewVersion(match[1])
		if err != nil {
			return nil, fmt.Errorf("CHANGELOG.md: invalid semantic version %q: %w", match[1], err)
		}
		date, err := time.Parse("2006-01-02", match[2])
		if err != nil {
			return nil, fmt.Errorf("invalid release date: %s", match[2])
		}
		if len(entries) > 0 && !entries[len(entries)-1].version.GreaterThan(version) {
			return nil, fmt.Errorf("CHANGELOG.md: published releases must appear once in strict descending semantic-version order")
		}
		entries = append(entries, changelogEntry{version: version, date: date})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if firstHeading != "## [Unreleased]" {
		return nil, fmt.Errorf("CHANGELOG.md: the first release section must be ## [Unreleased]")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("CHANGELOG.md: missing published release heading")
	}
	return entries, nil
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
