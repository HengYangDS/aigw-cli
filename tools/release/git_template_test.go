package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSignedReleaseFixtureIgnoresHostGitTemplate(t *testing.T) {
	template := t.TempDir()
	for name, content := range map[string]string{
		filepath.Join("hooks", "pre-commit"): "#!/bin/sh\necho host template hook ran >&2\nexit 1\n",
		filepath.Join("info", "exclude"):     "*\n",
		"config":                             "[core]\n\trepositoryformatversion = 0\n[aigw]\n\thostTemplate = injected\n",
	} {
		path := filepath.Join(template, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	global := filepath.Join(t.TempDir(), "config")
	if output, err := exec.Command("git", "config", "--file", global, "init.templateDir", template).CombinedOutput(); err != nil {
		t.Fatalf("host template configuration: %v: %s", err, output)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	prepareSignedRelease(t, "0.1.0")
	if output, err := exec.Command("git", "config", "--local", "--get", "aigw.hostTemplate").CombinedOutput(); err == nil {
		t.Fatalf("fixture repository inherited host template configuration: %s", output)
	}
	if _, err := os.Stat(filepath.Join(".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatalf("fixture repository inherited the host template hook: %v", err)
	}
}
