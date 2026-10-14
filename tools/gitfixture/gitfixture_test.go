package gitfixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type tests func() int

func (run tests) Run() int {
	return run()
}

func TestRunKeepsHostSettingsWithoutHostHooksOrTemplates(t *testing.T) {
	host := t.TempDir()
	hooks, template := filepath.Join(host, "hooks"), filepath.Join(host, "template")
	for _, path := range []string{
		filepath.Join(hooks, "reference-transaction"),
		filepath.Join(hooks, "pre-commit"),
		filepath.Join(template, "hooks", "pre-commit"),
	} {
		writeFile(t, path, "#!/bin/sh\necho host hook ran >&2\nexit 1\n")
	}
	writeFile(t, filepath.Join(template, "config"), "[core]\n\trepositoryformatversion = 0\n[aigw]\n\ttemplate = injected\n")
	global := filepath.Join(host, "config")
	for _, setting := range [][2]string{{"core.hooksPath", hooks}, {"init.templateDir", template}, {"aigw.host", "kept"}} {
		git(t, "config", "--file", global, setting[0], setting[1])
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_TEMPLATE_DIR", template)
	scratch := isolateScratch(t)

	repository := filepath.Join(t.TempDir(), "repository")
	code := Run(tests(func() int {
		git(t, "init", "-q", "-b", "main", repository)
		writeFile(t, filepath.Join(repository, "fixture"), "fixture\n")
		commit := []string{"-C", repository, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"}
		git(t, "-C", repository, "add", "fixture")
		git(t, commit...)
		if got := git(t, "-C", repository, "config", "--get", "aigw.host"); got != "kept" {
			t.Errorf("host global setting = %q, want kept", got)
		}
		if output, err := exec.Command("git", "-C", repository, "config", "--get", "aigw.template").CombinedOutput(); err == nil {
			t.Errorf("fixture inherited host template configuration: %s", output)
		}
		owned := filepath.Join(repository, ".fixture-hooks")
		writeFile(t, filepath.Join(owned, "post-commit"), "#!/bin/sh\n: > fixture-hook-ran\n")
		git(t, "-C", repository, "config", "core.hooksPath", owned)
		git(t, append(commit, "--allow-empty")...)
		if _, err := os.Stat(filepath.Join(repository, "fixture-hook-ran")); err != nil {
			t.Errorf("fixture-owned hook did not run: %v", err)
		}
		return 0
	}))
	if code != 0 {
		t.Fatalf("Run = %d, want 0", code)
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Fatalf("isolated Git policy survived the run: %v %v", entries, err)
	}
}

func TestRunOwnsGitPolicyWhenGitIsUnavailable(t *testing.T) {
	t.Setenv("PATH", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("GIT_CONFIG_GLOBAL", "")
	isolateScratch(t)
	code := Run(tests(func() int {
		content, err := os.ReadFile(os.Getenv("GIT_CONFIG_GLOBAL"))
		if err != nil || strings.Contains(string(content), "[include]") || !strings.Contains(string(content), "hooksPath") {
			t.Errorf("owned Git configuration = %q, %v", content, err)
		}
		return 0
	}))
	if code != 0 {
		t.Fatalf("Run = %d, want tests to run without a Git executable", code)
	}
}

func TestRunFailsWhenItCannotRemoveItsPolicy(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("Root can remove entries from a directory without write permission")
	}
	global := filepath.Join(t.TempDir(), "config")
	writeFile(t, global, "")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	isolateScratch(t)
	var owned string
	var held *os.File
	code := Run(tests(func() int {
		var err error
		// Windows refuses to delete an open file; other hosts refuse to delete
		// entries of a directory without write permission.
		if held, err = os.Open(os.Getenv("GIT_CONFIG_GLOBAL")); err != nil {
			t.Fatal(err)
		}
		owned = filepath.Dir(os.Getenv("GIT_CONFIG_GLOBAL"))
		if err := os.Chmod(owned, 0o500); err != nil {
			t.Fatal(err)
		}
		return 0
	}))
	if held != nil {
		_ = held.Close()
	}
	if owned == "" {
		t.Fatalf("Run = %d without running tests", code)
	}
	if err := os.Chmod(owned, 0o700); err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatal("Run reported success while its isolated Git policy remained")
	}
}

func TestRunRefusesAnUnreadableHostConfiguration(t *testing.T) {
	global := filepath.Join(t.TempDir(), "config")
	writeFile(t, global, "[broken\n")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_TEMPLATE_DIR", "")
	scratch := isolateScratch(t)
	ran := false
	if code := Run(tests(func() int { ran = true; return 0 })); code == 0 || ran {
		t.Fatalf("Run = %d with tests run %t, want a refusal before tests", code, ran)
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Fatalf("refused isolation left scratch behind: %v %v", entries, err)
	}
}

func TestRunRefusesUnavailableScratch(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "file")
	writeFile(t, blocked, "not a directory\n")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, blocked)
	}
	ran := false
	if code := Run(tests(func() int { ran = true; return 0 })); code == 0 || ran {
		t.Fatalf("Run = %d with tests run %t, want a refusal before tests", code, ran)
	}
}

func isolateScratch(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"GIT_CONFIG_GLOBAL", "GIT_TEMPLATE_DIR"} {
		t.Setenv(name, os.Getenv(name))
	}
	scratch := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, scratch)
	}
	return scratch
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
