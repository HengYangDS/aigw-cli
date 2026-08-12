package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var releaseHeading = regexp.MustCompile(`^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$`)
var semanticVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type versionPart struct {
	numeric bool
	number  int
	text    string
}

type versionKey struct {
	major, minor, patch int
	release             bool
	prerelease          []versionPart
}

type releaseEntry struct {
	version string
	date    time.Time
	key     versionKey
}

func checkChangelog(root string, args []string) error {
	if len(args) > 2 {
		return fmt.Errorf("usage: repository changelog [path] [tag]")
	}
	path := filepath.Join(root, "CHANGELOG.md")
	if len(args) >= 1 && args[0] != "" {
		path = args[0]
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
	}
	entries, err := parseChangelog(path)
	if err != nil {
		return err
	}
	selectedTag := ""
	if len(args) == 2 {
		selectedTag = args[1]
	}
	if selectedTag == "" {
		selectedTag = os.Getenv("AIGW_CHANGELOG_RELEASE_TAG")
	}
	if selectedTag == "" && os.Getenv("GITHUB_REF_TYPE") == "tag" {
		selectedTag = os.Getenv("GITHUB_REF_NAME")
	}
	if selectedTag == "" {
		selectedTag = os.Getenv("CI_COMMIT_TAG")
	}
	if selectedTag == "" {
		return nil
	}
	if !releaseTag.MatchString(selectedTag) {
		return fmt.Errorf("CHANGELOG.md: selected release tag is malformed: %s", selectedTag)
	}
	if entries[0].version != strings.TrimPrefix(selectedTag, "v") {
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

func printReleaseEpoch(root string, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: repository release-epoch <version> [changelog]")
	}
	path := filepath.Join(root, "CHANGELOG.md")
	if len(args) == 2 {
		path = args[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
	}
	entries, err := parseChangelog(path)
	if err != nil {
		return err
	}
	matched := []releaseEntry{}
	for _, entry := range entries {
		if entry.version == args[0] {
			matched = append(matched, entry)
		}
	}
	if len(matched) == 0 {
		return fmt.Errorf("release heading not found: %s", args[0])
	}
	// parseChangelog rejects duplicate published versions, so a non-empty
	// match set contains exactly one entry.
	_, _ = fmt.Fprintln(os.Stdout, matched[0].date.Unix())
	return nil
}

func parseChangelog(path string) ([]releaseEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("CHANGELOG.md: missing file")
	}
	defer func() { _ = file.Close() }()
	firstHeading := ""
	entries := []releaseEntry{}
	seen := map[string]bool{}
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
		key, err := parseVersion(match[1])
		if err != nil {
			return nil, fmt.Errorf("CHANGELOG.md: %w", err)
		}
		date, err := time.Parse("2006-01-02", match[2])
		if err != nil {
			return nil, fmt.Errorf("invalid release date: %s", match[2])
		}
		if seen[match[1]] {
			return nil, fmt.Errorf("CHANGELOG.md: duplicate published version at line %d: %s", lineNumber, match[1])
		}
		seen[match[1]] = true
		entries = append(entries, releaseEntry{version: match[1], date: date, key: key})
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
	sorted := append([]releaseEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return compareVersion(sorted[i].key, sorted[j].key) > 0 })
	for index := range entries {
		if entries[index].version != sorted[index].version {
			return nil, fmt.Errorf("CHANGELOG.md: published releases must appear once in strict descending semantic-version order")
		}
	}
	return entries, nil
}

func parseVersion(raw string) (versionKey, error) {
	match := semanticVersion.FindStringSubmatch(raw)
	if match == nil {
		return versionKey{}, fmt.Errorf("invalid semantic version: %s", raw)
	}
	key := versionKey{major: atoi(match[1]), minor: atoi(match[2]), patch: atoi(match[3]), release: match[4] == ""}
	for _, part := range strings.Split(match[4], ".") {
		if part == "" {
			continue
		}
		if number, err := strconv.Atoi(part); err == nil {
			key.prerelease = append(key.prerelease, versionPart{numeric: true, number: number})
		} else {
			key.prerelease = append(key.prerelease, versionPart{text: part})
		}
	}
	return key, nil
}

func compareVersion(left, right versionKey) int {
	for _, pair := range [][2]int{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] != pair[1] {
			if pair[0] > pair[1] {
				return 1
			}
			return -1
		}
	}
	if left.release != right.release {
		if left.release {
			return 1
		}
		return -1
	}
	for index := 0; index < len(left.prerelease) && index < len(right.prerelease); index++ {
		a, b := left.prerelease[index], right.prerelease[index]
		if a.numeric && b.numeric && a.number != b.number {
			if a.number > b.number {
				return 1
			}
			return -1
		}
		if a.numeric != b.numeric {
			if a.numeric {
				return -1
			}
			return 1
		}
		if a.text != b.text {
			if a.text > b.text {
				return 1
			}
			return -1
		}
	}
	if len(left.prerelease) > len(right.prerelease) {
		return 1
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	return 0
}

func atoi(raw string) int { value, _ := strconv.Atoi(raw); return value }

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
