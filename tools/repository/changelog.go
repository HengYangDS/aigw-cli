package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

var releaseHeading = regexp.MustCompile(`^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$`)

type releaseEntry struct {
	version *semver.Version
	date    time.Time
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
	for _, entry := range entries {
		if entry.version.Original() == args[0] {
			_, err := fmt.Fprintln(os.Stdout, entry.date.Unix())
			return err
		}
	}
	return fmt.Errorf("release heading not found: %s", args[0])
}

func parseChangelog(path string) ([]releaseEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("CHANGELOG.md: missing file")
	}
	defer func() { _ = file.Close() }()
	firstHeading := ""
	entries := []releaseEntry{}
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
		entries = append(entries, releaseEntry{version: version, date: date})
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
