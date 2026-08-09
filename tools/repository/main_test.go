package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestParseVersionOrdersPrereleases(t *testing.T) {
	rc80, _ := parseVersion("0.1.0-rc.80")
	rc79, _ := parseVersion("0.1.0-rc.79")
	if compareVersion(rc80, rc79) <= 0 {
		t.Fatal("rc.80 must sort after rc.79")
	}
}

func TestRunChecksChangelogAndReleaseEpoch(t *testing.T) {
	root := t.TempDir()
	changelog := "# Changelog\n\n## [Unreleased]\n\n## [1.2.3] - 2026-08-07\n\n- Release.\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "changelog"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "release-epoch", "1.2.3"}); err != nil {
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

func TestRepositoryOwnsFormerShellForwarders(t *testing.T) {
	root := initReleaseRepository(t, "1.2.3")
	if err := run([]string{"--root", root, "go-format"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "english-text"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "changelog"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "release-epoch", "1.2.3"}); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRejectsCredentialShapesAndProductSurfaceDrift(t *testing.T) {
	root := productSurfaceRepository(t)
	if err := checkProductSurface(root); err != nil {
		t.Fatal(err)
	}
	if err := checkCredentials(root); err != nil {
		t.Fatal(err)
	}

	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		gitRepository(t, root, "add", relative)
	}

	write("internal/leak.go", "package internal\n\nconst token = \"sk-abcdefghijklmnopqrstuvwxyz012345\"\n")
	if err := checkCredentials(root); err == nil || !strings.Contains(err.Error(), "outside test source") {
		t.Fatalf("production credential shape accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "internal", "leak.go")); err != nil {
		t.Fatal(err)
	}
	gitRepository(t, root, "add", "internal/leak.go")

	write("internal/leak_test.go", "package internal\n\nconst token = \"sk-abcdefghijklmnopqrstuvwxyz012345\"\n")
	if err := checkCredentials(root); err == nil || !strings.Contains(err.Error(), "test fixture") {
		t.Fatalf("test credential shape accepted: %v", err)
	}

	write("README.md", "# Product\n\nProprietary\n")
	if err := checkProductSurface(root); err == nil {
		t.Fatal("proprietary product surface accepted")
	}
}

func TestRepositoryContractErrorSurfaces(t *testing.T) {
	if err := checkCredentials(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing credential repository accepted")
	}
	root := productSurfaceRepository(t)
	for _, command := range []string{"credentials", "product-surface"} {
		if err := run([]string{"--root", root, command}); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}
	for relative, sentinel := range map[string]string{
		"internal/secrets/store_test.go":     "aigw-test-secret-never-leaks",
		"internal/diagnostics/probe_test.go": "aigw-test-gateway-token-never-leaks",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.WriteFile(path, []byte("package fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitRepository(t, root, "add", relative)
		if err := checkCredentials(root); err == nil || !strings.Contains(err.Error(), "redaction sentinel is missing") {
			t.Fatalf("missing sentinel %s accepted: %v", sentinel, err)
		}
		if err := os.WriteFile(path, []byte("package fixture\nconst sentinel = \""+sentinel+"\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitRepository(t, root, "add", relative)
	}
	if err := os.Remove(filepath.Join(root, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	if err := checkProductSurface(root); err == nil || !strings.Contains(err.Error(), "LICENSE") {
		t.Fatalf("missing product surface accepted: %v", err)
	}

	for _, relative := range []string{"README.md", "docs/README.md"} {
		root := productSurfaceRepository(t)
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.WriteFile(path, []byte("Proprietary\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := checkProductSurface(root); err == nil {
			t.Fatalf("invalid %s accepted", relative)
		}
	}

	root = productSurfaceRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "nested", "license.md"), []byte("Proprietary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkProductSurface(root); err == nil || !strings.Contains(err.Error(), "proprietary licensing residue") {
		t.Fatalf("nested proprietary residue accepted: %v", err)
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
	os.Args = []string{"repository", "--root", productSurfaceRepository(t), "product-surface"}
	status := -1
	exit = func(code int) { status = code }
	main()
	if status != 0 {
		t.Fatalf("main status = %d", status)
	}
}

func TestEnglishTextAndMalformedChangelog(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "-C", root, "init", "-q")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("English\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command = exec.Command("git", "-C", root, "add", "README.md")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	if err := checkEnglishText(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("\u4e2d\u6587\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkEnglishText(root); err == nil || !strings.Contains(err.Error(), "English-only") {
		t.Fatalf("non-English error=%v", err)
	}
	bad := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(bad, []byte("## [1.0.0] - 2026-08-07\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseChangelog(bad); err == nil {
		t.Fatal("changelog without Unreleased accepted")
	}
}

func TestChangelogTagBindingAndVersionEdges(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "-C", root, "init", "-q")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for _, setting := range [][]string{{"user.name", "Actor"}, {"user.email", "actor@example.com"}} {
		command = exec.Command("git", "-C", root, "config", setting[0], setting[1])
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git config: %v: %s", err, output)
		}
	}
	changelog := "## [Unreleased]\n\n## [1.2.3] - 2026-08-07\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "CHANGELOG.md"}, {"commit", "-q", "-m", "release"}, {"tag", "v1.2.3"}} {
		command = exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := checkChangelog(root, []string{"CHANGELOG.md", "v1.2.3"}); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"bad", "v9.9.9"} {
		if err := checkChangelog(root, []string{"CHANGELOG.md", tag}); err == nil {
			t.Fatalf("invalid tag accepted: %s", tag)
		}
	}
	for _, raw := range []string{"01.0.0", "1.0", "1.0.0-rc..1"} {
		if _, err := parseVersion(raw); err == nil {
			t.Fatalf("invalid version accepted: %s", raw)
		}
	}
	stable, _ := parseVersion("1.0.0")
	prerelease, _ := parseVersion("1.0.0-rc.1")
	if compareVersion(stable, prerelease) <= 0 || compareVersion(stable, stable) != 0 {
		t.Fatal("semantic version comparison failed")
	}
}

func TestSemanticVersionOrderingIsComplete(t *testing.T) {
	ordered := []string{
		"2.0.0",
		"1.1.0",
		"1.0.1",
		"1.0.0",
		"1.0.0-rc.2",
		"1.0.0-rc.1.1",
		"1.0.0-rc.1",
		"1.0.0-beta",
		"1.0.0-2",
		"1.0.0-1",
		"0.9.9",
	}
	for index := 0; index < len(ordered)-1; index++ {
		left, err := parseVersion(ordered[index])
		if err != nil {
			t.Fatal(err)
		}
		right, err := parseVersion(ordered[index+1])
		if err != nil {
			t.Fatal(err)
		}
		if compareVersion(left, right) <= 0 || compareVersion(right, left) >= 0 {
			t.Fatalf("ordering failed: %s > %s", ordered[index], ordered[index+1])
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

func TestEnglishTextFailureSurfaces(t *testing.T) {
	if err := checkEnglishText(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing repository accepted")
	}
	root := t.TempDir()
	gitRepository(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "gone.txt"), []byte("English\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRepository(t, root, "add", "gone.txt")
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := checkEnglishText(root); err != nil {
		t.Fatal(err)
	}
}

func initReleaseRepository(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	gitRepository(t, root, "init", "-q")
	gitRepository(t, root, "config", "user.name", "Actor")
	gitRepository(t, root, "config", "user.email", "actor@example.com")
	content := fmt.Sprintf("## [Unreleased]\n\n## [%s] - 2026-08-07\n", version)
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRepository(t, root, "add", "CHANGELOG.md")
	gitRepository(t, root, "commit", "-q", "-m", "release")
	gitRepository(t, root, "tag", "v"+version)
	return root
}

func productSurfaceRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRepository(t, root, "init", "-q")
	files := map[string]string{
		"LICENSE":                            "MIT License\n\nCopyright (c) 2026 AIGW CLI Contributors\n\nTHE SOFTWARE IS PROVIDED \"AS IS\"\n",
		"README.md":                          "# Product\n\n[MIT](LICENSE)\n\nMIT License\n",
		"CHANGELOG.md":                       "## [Unreleased]\n",
		"CONTRIBUTING.md":                    "MIT License\n",
		"docs/README.md":                     "[LICENSE](../LICENSE)\n",
		"internal/secrets/store_test.go":     "package secrets\n\nconst sentinel = \"aigw-test-secret-never-leaks\"\n",
		"internal/diagnostics/probe_test.go": "package diagnostics\n\nconst sentinel = \"aigw-test-gateway-token-never-leaks\"\n",
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitRepository(t, root, "add", ".")
	return root
}

func gitRepository(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
