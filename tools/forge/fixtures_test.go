package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type forgeFixture struct {
	repository     string
	email          string
	key            string
	allowedSigners string
}

func newForgeFixture(t *testing.T) forgeFixture {
	t.Helper()
	repository := t.TempDir()
	email := "forge@example.invalid"
	key := filepath.Join(t.TempDir(), "signing-key")
	runCommand(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(public))
	allowed := filepath.Join(t.TempDir(), "allowed-signers")
	if err := os.WriteFile(allowed, []byte(email+" namespaces=\"git\" "+fields[0]+" "+fields[1]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "init", "-q", "-b", "main")
	gitTest(t, repository, "config", "user.name", "Forge Fixture")
	gitTest(t, repository, "config", "user.email", email)
	gitTest(t, repository, "config", "gpg.format", "ssh")
	gitTest(t, repository, "config", "gpg.ssh.program", "ssh-keygen")
	gitTest(t, repository, "config", "commit.gpgsign", "true")
	gitTest(t, repository, "config", "tag.gpgsign", "true")
	gitTest(t, repository, "config", "user.signingkey", key)
	gitTest(t, repository, "config", "core.hooksPath", filepath.Join(repository, ".disabled-hooks"))
	if err := os.MkdirAll(filepath.Join(repository, ".ethos"), 0o700); err != nil {
		t.Fatal(err)
	}
	policy := `[commit_policy]
subject_pattern = "^(feat|fix|docs|test|refactor|perf|build|ci|chore|revert)(\\([a-z0-9-]+\\))?!?: .+"
signing_required = true
signing_format = "ssh"
`
	if err := os.WriteFile(filepath.Join(repository, ".ethos", "workspace.toml"), []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "file"), []byte("value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "file", ".ethos/workspace.toml")
	gitTest(t, repository, "commit", "-q", "-m", "feat: initial product object")
	gitTest(t, repository, "tag", "-s", "-a", "v1.2.3", "-m", "release v1.2.3")
	return forgeFixture{repository: repository, email: email, key: key, allowedSigners: allowed}
}

func newSigningIdentity(t *testing.T, email string) forgeFixture {
	t.Helper()
	key := filepath.Join(t.TempDir(), "signing-key")
	runCommand(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(public))
	allowed := filepath.Join(t.TempDir(), "allowed-signers")
	if err := os.WriteFile(allowed, []byte(email+" namespaces=\"git\" "+fields[0]+" "+fields[1]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return forgeFixture{email: email, key: key, allowedSigners: allowed}
}

func writeCommitWithSubjectForTest(t *testing.T, repository, name, content, subject string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repository, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", name)
	gitTest(t, repository, "commit", "-q", "-m", subject)
}

func writeCommitForTest(t *testing.T, repository, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repository, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", name)
	gitTest(t, repository, "commit", "-q", "-m", "test: advance product object")
}

func newBareRepository(t *testing.T) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "peer.git")
	runCommand(t, "git", "init", "-q", "--bare", repository)
	gitTest(t, repository, "config", "core.hooksPath", filepath.Join(repository, "hooks"))
	return repository
}

func setHostileGitHooks(t *testing.T) {
	t.Helper()
	config := filepath.Join(t.TempDir(), "config")
	command := exec.Command("git", "config", "--file", config, "core.hooksPath", filepath.Join(t.TempDir(), "host-hooks"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("hostile git config: %v: %s", err, output)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func seedRemoteBranches(t *testing.T, remote string) {
	t.Helper()
	repository := t.TempDir()
	gitTest(t, repository, "init", "-q", "-b", "main")
	gitTest(t, repository, "config", "user.name", "Previous Publisher")
	gitTest(t, repository, "config", "user.email", "previous@example.invalid")
	gitTest(t, repository, "config", "commit.gpgsign", "false")
	gitTest(t, repository, "config", "core.hooksPath", filepath.Join(repository, ".disabled-hooks"))
	if err := os.WriteFile(filepath.Join(repository, "old"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "old")
	gitTest(t, repository, "commit", "-q", "-m", "old history")
	gitTest(t, repository, "branch", "dev", "main")
	gitTest(t, repository, "remote", "add", "peer", remote)
	gitTest(t, repository, "push", "-q", "peer", "main", "dev")
}

func gitTest(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	arguments = append([]string{"-C", repository}, arguments...)
	runCommand(t, "git", arguments...)
}

func gitOutputForTest(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func runCommand(t *testing.T, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(arguments, " "), err, output)
	}
}
