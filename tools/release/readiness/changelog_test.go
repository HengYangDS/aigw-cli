package readiness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangelogRequiresKeepAChangelogStructure(t *testing.T) {
	for name, content := range map[string]string{
		"missing title":               "## [Unreleased]\n",
		"missing standards":           "# Changelog\n\n## [Unreleased]\n",
		"missing unreleased":          validChangelog("## [1.0.0] - 2026-08-07\n\n### Fixed\n\n- Fix.\n"),
		"duplicate unreleased":        validChangelog("## [Unreleased]\n\n## [Unreleased]\n"),
		"malformed heading":           validChangelog("## [Unreleased]\n\n## [1.0.0] 2026-08-07\n"),
		"invalid version":             validChangelog("## [Unreleased]\n\n## [01.0.0] - 2026-08-07\n"),
		"invalid date":                validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-02-30\n"),
		"duplicate release":           validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n### Fixed\n\n- Fix.\n\n## [1.0.0] - 2026-08-06\n\n### Fixed\n\n- Fix.\n"),
		"ascending releases":          validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-08-06\n\n### Fixed\n\n- Fix.\n\n## [1.1.0] - 2026-08-07\n\n### Fixed\n\n- Fix.\n"),
		"nonstandard change category": validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n### Quality\n\n- Improve quality.\n"),
		"duplicate change category":   validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n### Fixed\n\n- Fix one.\n\n### Fixed\n\n- Fix two.\n"),
		"release without changes":     validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n### Fixed\n"),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "CHANGELOG.md")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := parseChangelog(path); err == nil {
				t.Fatal("invalid changelog structure was accepted")
			}
		})
	}

	for name, content := range map[string]string{
		"initial unreleased": validChangelog("## [Unreleased]\n"),
		"released history": validChangelog(
			"## [Unreleased]\n\n" +
				"## [1.2.3+build.1] - 2026-08-07\n\n### Fixed\n\n- Fix one.\n\n" +
				"## [1.2.3-rc.1] - 2026-08-06\n\n### Added\n\n- Add one.\n",
		),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "CHANGELOG.md")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := parseChangelog(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChangelogTagBindsTheFirstPublishedVersionToHead(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	path := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n## [1.2.3] - 2026-08-07\n\n### Fixed\n\n- Fix one.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "CHANGELOG.md", "VERSION")
	git(t, root, "-c", "user.name=Release Test", "-c", "user.email=release@example.test", "commit", "-q", "-m", "release")
	git(t, root, "tag", "v1.2.3")
	if err := ValidateChangelog(root, path, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChangelog(root, path, "v1.2.4"); err == nil || !strings.Contains(err.Error(), "first published section") {
		t.Fatalf("different release tag = %v", err)
	}
	for _, tag := range []string{"bad", "1.2.3", "v1.2.3-01", "v1.2.3+other"} {
		if err := ValidateChangelog(root, path, tag); err == nil {
			t.Fatalf("invalid or different tag accepted: %s", tag)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "later"), []byte("later\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "later")
	git(t, root, "-c", "user.name=Release Test", "-c", "user.email=release@example.test", "commit", "-q", "-m", "later")
	if err := ValidateChangelog(root, path, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "does not identify HEAD") {
		t.Fatalf("stale release tag = %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChangelog(root, path, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unavailable release tag = %v", err)
	}
}

func TestChangelogAllowsOnlyTheCurrentPendingRelease(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	path := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n### Added\n\n- Initial release.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "CHANGELOG.md", "VERSION")
	git(t, root, "-c", "user.name=Release Test", "-c", "user.email=release@example.test", "commit", "-q", "-m", "release")
	git(t, root, "tag", "v1.0.0")

	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.1.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n### Changed\n\n- Pending change.\n\n## [1.0.0] - 2026-08-07\n\n### Added\n\n- Initial release.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChangelog(root, path, ""); err != nil {
		t.Fatalf("unreleased active train = %v", err)
	}

	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n## [1.1.0] - 2026-08-08\n\n### Changed\n\n- Pending change.\n\n## [1.0.0] - 2026-08-07\n\n### Added\n\n- Initial release.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChangelog(root, path, ""); err != nil {
		t.Fatalf("prepared release = %v", err)
	}

	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n## [1.1.0] - 2026-08-08\n\n### Changed\n\n- Pending change.\n\n## [1.0.1] - 2026-08-07\n\n### Fixed\n\n- Never published.\n\n## [1.0.0] - 2026-08-06\n\n### Added\n\n- Initial release.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChangelog(root, path, ""); err == nil || !strings.Contains(err.Error(), "has no Git tag") {
		t.Fatalf("historical untagged release = %v", err)
	}
}

func TestChangelogRequiresEveryReleaseTagAndCurrentVersion(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	path := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n## [1.1.0] - 2026-08-08\n\n### Changed\n\n- Latest.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.1.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "CHANGELOG.md", "VERSION")
	git(t, root, "-c", "user.name=Release Test", "-c", "user.email=release@example.test", "commit", "-q", "-m", "release")
	git(t, root, "tag", "v1.1.0")
	git(t, root, "tag", "v1.0.0")
	if err := ValidateChangelog(root, path, ""); err == nil || !strings.Contains(err.Error(), "missing release section") {
		t.Fatalf("tag without changelog release = %v", err)
	}

	git(t, root, "tag", "-d", "v1.0.0")
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChangelog(root, path, ""); err == nil || !strings.Contains(err.Error(), "must not precede") {
		t.Fatalf("regressed active version = %v", err)
	}
}

func TestLookupReleaseEpochPreservesExactBuildIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n\n## [1.2.3+build.1] - 2026-08-07\n\n### Fixed\n\n- Fix.\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	epoch, found, err := LookupReleaseEpoch(path, "1.2.3+build.1")
	if err != nil || !found || epoch != "1786060800" {
		t.Fatalf("release epoch = %q, found=%t, error=%v", epoch, found, err)
	}
	if _, found, err := LookupReleaseEpoch(path, "1.2.3+build.2"); err != nil || found {
		t.Fatalf("different build identity found=%t, error=%v", found, err)
	}
	if _, _, err := LookupReleaseEpoch(filepath.Join(t.TempDir(), "missing.md"), "1.2.3"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing chronology = %v", err)
	}
}

func TestChangelogUsesStrictSemanticVersionPrecedence(t *testing.T) {
	for _, test := range []struct {
		name     string
		versions []string
		valid    bool
	}{
		{name: "build identity", versions: []string{"1.2.3+build.001", "1.2.3-rc.1+build.002"}, valid: true},
		{name: "numeric prerelease", versions: []string{"1.0.0-9223372036854775808", "1.0.0-999999999999999999"}, valid: true},
		{name: "wide major", versions: []string{"9223372036854775808.0.0", "9223372036854775807.9.9"}, valid: true},
		{name: "short core", versions: []string{"1.0"}},
		{name: "empty prerelease component", versions: []string{"1.0.0-rc..1"}},
		{name: "leading zero prerelease", versions: []string{"1.0.0-rc.01"}},
		{name: "unrepresentable core", versions: []string{"18446744073709551616.0.0"}},
		{name: "equal precedence", versions: []string{"1.2.3+first", "1.2.3+second"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var sections strings.Builder
			sections.WriteString("## [Unreleased]\n")
			for _, version := range test.versions {
				sections.WriteString("\n## [" + version + "] - 2026-08-07\n\n### Fixed\n\n- Fix.\n")
			}
			path := filepath.Join(t.TempDir(), "CHANGELOG.md")
			if err := os.WriteFile(path, []byte(validChangelog(sections.String())), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := parseChangelog(path)
			if (err == nil) != test.valid {
				t.Fatalf("versions=%v error=%v valid=%t", test.versions, err, test.valid)
			}
		})
	}
}

func TestSelectedReleaseTagUsesExplicitForgePrecedence(t *testing.T) {
	t.Setenv("AIGW_CHANGELOG_RELEASE_TAG", "")
	t.Setenv("GITHUB_REF_TYPE", "")
	t.Setenv("GITHUB_REF_NAME", "")
	t.Setenv("CI_COMMIT_TAG", "")
	if tag := SelectedReleaseTag(); tag != "" {
		t.Fatalf("unselected tag = %q", tag)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.0.0")
	if tag := SelectedReleaseTag(); tag != "v1.0.0" {
		t.Fatalf("GitLab tag = %q", tag)
	}
	t.Setenv("GITHUB_REF_TYPE", "branch")
	t.Setenv("GITHUB_REF_NAME", "v2.0.0")
	if tag := SelectedReleaseTag(); tag != "v1.0.0" {
		t.Fatalf("branch name selected as tag: %q", tag)
	}
	t.Setenv("GITHUB_REF_TYPE", "tag")
	if tag := SelectedReleaseTag(); tag != "v2.0.0" {
		t.Fatalf("GitHub tag = %q", tag)
	}
	t.Setenv("AIGW_CHANGELOG_RELEASE_TAG", "v3.0.0")
	if tag := SelectedReleaseTag(); tag != "v3.0.0" {
		t.Fatalf("explicit tag = %q", tag)
	}
}

func TestChangelogReportsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(validChangelog("## [Unreleased]\n"+strings.Repeat("a", 70*1024))), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseChangelog(path); err == nil || !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("oversized changelog = %v", err)
	}
}

func validChangelog(sections string) string {
	return "# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n" + sections
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	settings := []string{"-C", root, "-c", "core.hooksPath=" + filepath.Join(root, ".disabled-hooks"), "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}
	command := exec.Command("git", append(settings, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
