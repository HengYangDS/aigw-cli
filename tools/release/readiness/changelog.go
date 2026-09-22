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

const unreleasedHeading = "## [Unreleased]"

var releaseHeading = regexp.MustCompile(`^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$`)

var changelogCategories = map[string]struct{}{
	"Added":      {},
	"Changed":    {},
	"Deprecated": {},
	"Removed":    {},
	"Fixed":      {},
	"Security":   {},
}

type changelogEntry struct {
	version *semver.Version
	date    time.Time
	items   int
}

type changelogParser struct {
	entries            []changelogEntry
	sectionCategories  map[string]struct{}
	firstHeading       string
	currentEntry       int
	unreleasedCount    int
	hasTitle           bool
	hasKeepAChangelog  bool
	hasSemanticVersion bool
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

// ValidateChangelog verifies the Keep a Changelog structure, local release
// provenance, and any selected release tag against the current product version.
func ValidateChangelog(root, path, selectedTag string) error {
	entries, err := parseChangelog(resolveChangelogPath(root, path))
	if err != nil {
		return err
	}
	selectedVersion, err := validateSelectedReleaseTag(root, entries, selectedTag)
	if err != nil {
		return err
	}
	return validateChangelogProvenance(root, entries, selectedVersion)
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

	parser := changelogParser{currentEntry: -1}
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		if err := parser.consume(scanner.Text(), lineNumber); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return parser.result()
}

func (p *changelogParser) consume(line string, lineNumber int) error {
	if lineNumber == 1 {
		p.hasTitle = line == "# Changelog"
	}
	p.hasKeepAChangelog = p.hasKeepAChangelog ||
		strings.Contains(line, "https://keepachangelog.com/en/1.1.0/")
	p.hasSemanticVersion = p.hasSemanticVersion || strings.Contains(line, "https://semver.org/")

	if strings.HasPrefix(line, "## ") {
		return p.consumeReleaseHeading(line, lineNumber)
	}
	if category, ok := strings.CutPrefix(line, "### "); ok {
		return p.consumeCategory(category, lineNumber)
	}
	if !strings.HasPrefix(line, "- ") {
		return nil
	}
	if len(p.sectionCategories) == 0 && p.firstHeading != "" {
		return fmt.Errorf("CHANGELOG.md: change item at line %d is outside a canonical category", lineNumber)
	}
	if p.currentEntry >= 0 {
		p.entries[p.currentEntry].items++
	}
	return nil
}

func (p *changelogParser) consumeReleaseHeading(line string, lineNumber int) error {
	if p.firstHeading == "" {
		p.firstHeading = line
	}
	p.sectionCategories = map[string]struct{}{}
	p.currentEntry = -1
	if line == unreleasedHeading {
		p.unreleasedCount++
		if p.unreleasedCount > 1 {
			return fmt.Errorf("CHANGELOG.md: duplicate Unreleased section at line %d", lineNumber)
		}
		return nil
	}
	if !strings.HasPrefix(line, "## [") {
		return fmt.Errorf("CHANGELOG.md: unsupported release heading at line %d: %s", lineNumber, line)
	}
	match := releaseHeading.FindStringSubmatch(line)
	if match == nil {
		return fmt.Errorf("CHANGELOG.md: malformed published heading at line %d: %s", lineNumber, line)
	}
	version, err := semver.StrictNewVersion(match[1])
	if err != nil {
		return fmt.Errorf("CHANGELOG.md: invalid semantic version %q: %w", match[1], err)
	}
	date, err := time.Parse("2006-01-02", match[2])
	if err != nil {
		return fmt.Errorf("invalid release date: %s", match[2])
	}
	if len(p.entries) > 0 && !p.entries[len(p.entries)-1].version.GreaterThan(version) {
		return fmt.Errorf("CHANGELOG.md: published releases must appear once in strict descending semantic-version order")
	}
	p.entries = append(p.entries, changelogEntry{version: version, date: date})
	p.currentEntry = len(p.entries) - 1
	return nil
}

func (p *changelogParser) consumeCategory(category string, lineNumber int) error {
	if p.firstHeading == "" {
		return fmt.Errorf("CHANGELOG.md: change category at line %d precedes Unreleased", lineNumber)
	}
	if _, ok := changelogCategories[category]; !ok {
		return fmt.Errorf("CHANGELOG.md: unsupported change category at line %d: %s", lineNumber, category)
	}
	if _, duplicate := p.sectionCategories[category]; duplicate {
		return fmt.Errorf("CHANGELOG.md: duplicate change category at line %d: %s", lineNumber, category)
	}
	p.sectionCategories[category] = struct{}{}
	return nil
}

func (p *changelogParser) result() ([]changelogEntry, error) {
	if !p.hasTitle {
		return nil, fmt.Errorf("CHANGELOG.md: first line must be # Changelog")
	}
	if !p.hasKeepAChangelog || !p.hasSemanticVersion {
		return nil, fmt.Errorf("CHANGELOG.md: introduction must identify Keep a Changelog 1.1.0 and Semantic Versioning")
	}
	if p.firstHeading != unreleasedHeading {
		return nil, fmt.Errorf("CHANGELOG.md: the first release section must be %s", unreleasedHeading)
	}
	for _, entry := range p.entries {
		if entry.items == 0 {
			return nil, fmt.Errorf("CHANGELOG.md: release %s has no categorized change items", entry.version.Original())
		}
	}
	return p.entries, nil
}

func validateSelectedReleaseTag(root string, entries []changelogEntry, selectedTag string) (string, error) {
	if selectedTag == "" {
		return "", nil
	}
	version, prefixed := strings.CutPrefix(selectedTag, "v")
	if _, err := semver.StrictNewVersion(version); !prefixed || err != nil {
		return "", fmt.Errorf("CHANGELOG.md: selected release tag is malformed: %s", selectedTag)
	}
	if len(entries) == 0 || entries[0].version.Original() != version {
		return "", fmt.Errorf("CHANGELOG.md: first published section must identify selected release tag: %s", selectedTag)
	}
	tagCommit, err := gitOutput(root, "rev-parse", "refs/tags/"+selectedTag+"^{}")
	if err != nil {
		return "", fmt.Errorf("CHANGELOG.md: selected release tag is unavailable: %s", selectedTag)
	}
	head, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil || tagCommit != head {
		return "", fmt.Errorf("CHANGELOG.md: selected release tag does not identify HEAD: %s", selectedTag)
	}
	return version, nil
}

func validateChangelogProvenance(root string, entries []changelogEntry, selectedVersion string) error {
	currentVersion, err := ReadProductVersion(root)
	if err != nil {
		return err
	}
	if selectedVersion != "" && selectedVersion != currentVersion {
		return fmt.Errorf("CHANGELOG.md: selected release version %s does not match VERSION %s", selectedVersion, currentVersion)
	}
	tags, err := localReleaseVersions(root)
	if err != nil {
		return err
	}
	entryVersions := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entryVersions[entry.version.Original()] = struct{}{}
	}
	for version := range tags {
		if _, ok := entryVersions[version]; !ok {
			return fmt.Errorf("CHANGELOG.md: missing release section for Git tag v%s", version)
		}
	}
	pending := ""
	for index, entry := range entries {
		version := entry.version.Original()
		if _, published := tags[version]; published {
			continue
		}
		if pending != "" || index != 0 || version != currentVersion {
			return fmt.Errorf("CHANGELOG.md: release %s has no Git tag", version)
		}
		pending = version
	}
	current, _ := semver.StrictNewVersion(currentVersion)
	for version := range tags {
		published, _ := semver.StrictNewVersion(version)
		if _, currentPublished := tags[currentVersion]; !currentPublished && !current.GreaterThan(published) {
			return fmt.Errorf("VERSION %s must not precede latest release %s", currentVersion, version)
		}
	}
	return nil
}

func localReleaseVersions(root string) (map[string]struct{}, error) {
	output, err := gitOutput(root, "tag", "--list", "v*")
	if err != nil {
		return nil, fmt.Errorf("read local release tags: %w", err)
	}
	versions := map[string]struct{}{}
	for tag := range strings.FieldsSeq(output) {
		version, prefixed := strings.CutPrefix(tag, "v")
		if _, err := semver.StrictNewVersion(version); !prefixed || err != nil {
			return nil, fmt.Errorf("release tag is not strict SemVer: %s", tag)
		}
		versions[version] = struct{}{}
	}
	return versions, nil
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
