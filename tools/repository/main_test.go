package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	for _, variable := range []string{
		"AIGW_CHANGELOG_RELEASE_TAG",
		"GITHUB_REF_TYPE",
		"GITHUB_REF_NAME",
		"CI_COMMIT_TAG",
	} {
		_ = os.Unsetenv(variable)
	}
	os.Exit(m.Run())
}

func TestParseChangelogOrdersSemanticVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	content := "## [Unreleased]\n\n## [1.1.0] - 2026-08-06\n\n## [1.0.0] - 2026-08-05\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := parseChangelog(path)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

func TestParseChangelogRejectsInvalidDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte("## [Unreleased]\n\n## [1.0.0] - 2026-02-30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := parseChangelog(path)
	if err == nil || !strings.Contains(err.Error(), "invalid release date") {
		t.Fatalf("err=%v", err)
	}
}

func TestChangelogUsesStrictReleaseVersions(t *testing.T) {
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
			var content strings.Builder
			content.WriteString("## [Unreleased]\n")
			for _, version := range test.versions {
				fmt.Fprintf(&content, "\n## [%s] - 2026-08-07\n", version)
			}
			path := filepath.Join(t.TempDir(), "CHANGELOG.md")
			if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			entries, err := parseChangelog(path)
			if (err == nil) != test.valid || (test.valid && len(entries) != len(test.versions)) {
				t.Fatalf("versions=%v entries=%v error=%v valid=%t", test.versions, entries, err, test.valid)
			}
		})
	}
}

func TestChangelogBindsExactBuildMetadataToTag(t *testing.T) {
	const version = "1.2.3-rc.1+build.001"
	root := initReleaseRepository(t, version)
	if err := checkChangelog(root, []string{"CHANGELOG.md", "v" + version}); err != nil {
		t.Fatal(err)
	}
	gitRepository(t, root, "tag", "v1.2.3-rc.1+build.002")
	if err := checkChangelog(root, []string{"CHANGELOG.md", "v1.2.3-rc.1+build.002"}); err == nil || !strings.Contains(err.Error(), "first published section") {
		t.Fatalf("different build identity accepted: %v", err)
	}
	if err := printReleaseEpoch(root, []string{version}); err != nil {
		t.Fatal(err)
	}
	if err := printReleaseEpoch(root, []string{"1.2.3-rc.1+build.002"}); err == nil {
		t.Fatal("release epoch ignored build identity")
	}
}

func TestRunChecksChangelogAndReleaseEpoch(t *testing.T) {
	root := t.TempDir()
	changelog := "# Changelog\n\n## [Unreleased]\n\n## [1.2.3] - 2026-08-07\n\n- Release.\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "changelog"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "release-epoch", "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "protected-lifecycle"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"unknown"}, {"--root", root, "release-epoch"}, {"--root", root, "changelog", "a", "b", "c"}} {
		if err := run(args); err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
	if err := run([]string{"--unknown"}); err == nil {
		t.Fatal("invalid flag accepted")
	}
}

func TestRepositoryOwnsReleaseChecks(t *testing.T) {
	root := initReleaseRepository(t, "1.2.3")
	if err := run([]string{"--root", root, "changelog"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "release-epoch", "1.2.3"}); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteReportsErrors(t *testing.T) {
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stderr.Close(); err != nil {
			t.Errorf("close stderr: %v", err)
		}
	})
	if execute(nil, stderr) != 1 {
		t.Fatal("invalid invocation succeeded")
	}
	if execute([]string{"--root", initReleaseRepository(t, "1.2.3"), "changelog"}, stderr) != 0 {
		t.Fatal("valid invocation failed")
	}
}

func TestMainDelegatesProcessStatus(t *testing.T) {
	previousArgs := os.Args
	previousExit := exit
	t.Cleanup(func() { os.Args, exit = previousArgs, previousExit })
	os.Args = []string{"repository", "--root", initReleaseRepository(t, "1.2.3"), "changelog"}
	status := -1
	exit = func(code int) { status = code }
	main()
	if status != 0 {
		t.Fatalf("main status = %d", status)
	}
}

func TestMalformedChangelog(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(bad, []byte("## [1.0.0] - 2026-08-07\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseChangelog(bad); err == nil {
		t.Fatal("changelog without Unreleased accepted")
	}
	if err := checkChangelog(root, nil); err == nil {
		t.Fatal("changelog command accepted malformed content")
	}
}

func TestChangelogTagBindingAndVersionEdges(t *testing.T) {
	setHostileGitConfig(t)
	root := t.TempDir()
	initUnsignedRepository(t, root)
	changelog := "## [Unreleased]\n\n## [1.2.3] - 2026-08-07\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "CHANGELOG.md"}, {"commit", "-q", "-m", "release"}, {"tag", "v1.2.3"}} {
		gitRepository(t, root, args...)
	}
	if err := checkChangelog(root, []string{"CHANGELOG.md", "v1.2.3"}); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"bad", "1.2.3", "v1.2.3-01", "v1.2.3+", "v9.9.9"} {
		if err := checkChangelog(root, []string{"CHANGELOG.md", tag}); err == nil {
			t.Fatalf("invalid tag accepted: %s", tag)
		}
	}
}

func TestChangelogOrdersReleasePrecedence(t *testing.T) {
	ordered := []string{
		"2.0.0",
		"1.1.0",
		"1.0.1",
		"1.0.0",
		"1.0.0-rc.80",
		"1.0.0-rc.79",
		"1.0.0-rc.2",
		"1.0.0-rc.1.1",
		"1.0.0-rc.1",
		"1.0.0-beta",
		"1.0.0-2",
		"1.0.0-1",
		"0.9.9",
	}
	for _, reverse := range []bool{false, true} {
		var content strings.Builder
		content.WriteString("## [Unreleased]\n")
		for index := range ordered {
			if reverse {
				index = len(ordered) - index - 1
			}
			fmt.Fprintf(&content, "\n## [%s] - 2026-08-07\n", ordered[index])
		}
		path := filepath.Join(t.TempDir(), "CHANGELOG.md")
		if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		entries, err := parseChangelog(path)
		if (err != nil) != reverse || (!reverse && len(entries) != len(ordered)) {
			t.Fatalf("reverse=%t entries=%v error=%v", reverse, entries, err)
		}
	}
}

func TestRepositoryRejectsMalformedInputs(t *testing.T) {
	root := t.TempDir()
	write := func(content string) string {
		t.Helper()
		path := filepath.Join(root, "CHANGELOG.md")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for name, content := range map[string]string{
		"missing":           "",
		"malformed":         "## [Unreleased]\n\n## [1.0.0] 2026-08-07\n",
		"invalid version":   "## [Unreleased]\n\n## [01.0.0] - 2026-08-07\n",
		"duplicate":         "## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n## [1.0.0] - 2026-08-06\n",
		"missing release":   "## [Unreleased]\n",
		"ascending release": "## [Unreleased]\n\n## [1.0.0] - 2026-08-06\n\n## [1.1.0] - 2026-08-07\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseChangelog(write(content)); err == nil {
				t.Fatal("malformed changelog accepted")
			}
		})
	}
	if _, err := parseChangelog(filepath.Join(root, "absent.md")); err == nil {
		t.Fatal("absent changelog accepted")
	}
	write("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n")
	for _, args := range [][]string{{}, {"1.0.0", "a", "b"}, {"missing"}} {
		if err := printReleaseEpoch(root, args); err == nil {
			t.Fatalf("invalid release epoch accepted: %v", args)
		}
	}
	custom := filepath.Join(root, "CUSTOM.md")
	if err := os.WriteFile(custom, []byte("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := printReleaseEpoch(root, []string{"1.0.0", "CUSTOM.md"}); err != nil {
		t.Fatal(err)
	}
	write("## [Unreleased]\n\n## [1.0.0] - 2026-08-07\n\n## [1.0.0] - 2026-08-06\n")
	if err := printReleaseEpoch(root, []string{"1.0.0"}); err == nil {
		t.Fatal("duplicate release epoch accepted")
	}
	if err := checkChangelog(root, []string{"a", "b", "c"}); err == nil {
		t.Fatal("surplus changelog arguments accepted")
	}
}

func TestChangelogTagEnvironmentPrecedenceAndHeadBinding(t *testing.T) {
	root := initReleaseRepository(t, "1.2.3")
	for _, variable := range []string{"AIGW_CHANGELOG_RELEASE_TAG", "GITHUB_REF_NAME", "CI_COMMIT_TAG"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv("AIGW_CHANGELOG_RELEASE_TAG", "")
			t.Setenv("GITHUB_REF_TYPE", "")
			t.Setenv("GITHUB_REF_NAME", "")
			t.Setenv("CI_COMMIT_TAG", "")
			t.Setenv(variable, "v1.2.3")
			if variable == "GITHUB_REF_NAME" {
				t.Setenv("GITHUB_REF_TYPE", "tag")
			}
			if err := checkChangelog(root, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(root, "later"), []byte("later\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRepository(t, root, "add", "later")
	gitRepository(t, root, "commit", "-q", "-m", "later")
	if err := checkChangelog(root, []string{"CHANGELOG.md", "v1.2.3"}); err == nil {
		t.Fatal("tag not identifying HEAD accepted")
	}
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := checkChangelog(root, []string{"CHANGELOG.md", "v1.2.3"}); err == nil {
		t.Fatal("unavailable repository tag accepted")
	}
}

func TestChangelogScannerReportsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	content := "## [Unreleased]\n" + strings.Repeat("a", 70*1024)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseChangelog(path); err == nil || !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("scanner error = %v", err)
	}
}

func initReleaseRepository(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	initUnsignedRepository(t, root)
	content := fmt.Sprintf("## [Unreleased]\n\n## [%s] - 2026-08-07\n", version)
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRepository(t, root, "add", "CHANGELOG.md")
	gitRepository(t, root, "commit", "-q", "-m", "release")
	gitRepository(t, root, "tag", "v"+version)
	return root
}

func initUnsignedRepository(t *testing.T, root string) {
	t.Helper()
	gitRepository(t, root, "init", "-q")
	for _, setting := range [][2]string{
		{"user.name", "Actor"},
		{"user.email", "actor@example.com"},
		{"commit.gpgsign", "false"},
		{"tag.gpgsign", "false"},
		{"core.hooksPath", filepath.Join(root, ".disabled-hooks")},
	} {
		gitRepository(t, root, "config", setting[0], setting[1])
	}
}

func setHostileGitConfig(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	hooks := filepath.Join(root, "hooks")
	if err := os.Mkdir(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 97\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config")
	for _, setting := range [][2]string{
		{"commit.gpgsign", "true"},
		{"core.hooksPath", hooks},
		{"gpg.format", "ssh"},
		{"tag.gpgsign", "true"},
		{"user.signingkey", filepath.Join(root, "missing-signing-key")},
	} {
		command := exec.Command("git", "config", "--file", config, setting[0], setting[1])
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("hostile git config: %v: %s", err, output)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func gitRepository(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
