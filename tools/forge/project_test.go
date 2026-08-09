package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectionCreatesAndAdvancesTarget(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "github", remote)
	gitTest(t, fixture.repository, "push", "-q", "github", "refs/tags/v1.2.3:refs/tags/v1.2.3")
	option := projectionOptions{
		repository: fixture.repository, branch: "main", remote: "github",
		sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
		actorName: "GitHub Actor", actorEmail: fixture.email,
		signingKey: fixture.key, signingProgram: "ssh-keygen",
		sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners,
	}
	if err := project(option); err != nil {
		t.Fatal(err)
	}
	first := gitTestOutput(t, remote, "rev-parse", "refs/heads/main")
	if first == "" {
		t.Fatal("projection did not create target branch")
	}
	if err := project(option); err != nil {
		t.Fatal(err)
	}
	if second := gitTestOutput(t, remote, "rev-parse", "refs/heads/main"); second != first {
		t.Fatalf("stable projection changed: %s != %s", second, first)
	}
}

func TestProjectionCommandAndProviderBoundary(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	args := []string{
		"project", "--repository", fixture.repository, "--branch", "main", "--remote", "peer",
		"--source-provider", "gitlab", "--target-provider", "github",
		"--source-email", fixture.email, "--actor-name", "Peer Actor", "--actor-email", fixture.email,
		"--signing-key", fixture.key, "--source-allowed-signers", fixture.allowedSigners,
		"--target-allowed-signers", fixture.allowedSigners,
	}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	args[10] = "gitlab"
	if err := run(args); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("equal providers=%v", err)
	}
}

func TestProjectionRejectsInvalidState(t *testing.T) {
	fixture := forgeFixture(t)
	option := projectionOptions{repository: fixture.repository, branch: "main", remote: "missing", sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email, actorName: "Actor", actorEmail: fixture.email, signingKey: fixture.key, sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners}
	if err := project(option); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("missing remote=%v", err)
	}
	gitTest(t, fixture.repository, "remote", "add", "github", "relative")
	option.remote = "github"
	if err := project(option); err == nil || !strings.Contains(err.Error(), "explicit local") {
		t.Fatalf("invalid remote=%v", err)
	}
	gitTest(t, fixture.repository, "remote", "set-url", "github", "file://"+filepath.Join(t.TempDir(), "remote.git"))
	if err := os.WriteFile(filepath.Join(fixture.repository, "dirty"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := project(option); err == nil || !strings.Contains(err.Error(), "dirty canonical") {
		t.Fatalf("dirty state=%v", err)
	}
}

func TestProjectionRejectsSourceAndRemoteDivergence(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	option := projectionOptions{
		repository: fixture.repository, branch: "main", remote: "peer",
		sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
		actorName: "Peer Actor", actorEmail: fixture.email, signingKey: fixture.key,
		sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners,
	}
	rogue := forgeKey(t, "rogue@example.invalid")
	option.sourceSigners = rogue.allowedSigners
	if err := project(option); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("source trust=%v", err)
	}
	option.sourceSigners = fixture.allowedSigners
	other := t.TempDir()
	gitTest(t, other, "init", "-q", "-b", "main")
	gitTest(t, other, "config", "user.name", "Other")
	gitTest(t, other, "config", "user.email", fixture.email)
	gitTest(t, other, "config", "core.hooksPath", filepath.Join(other, ".disabled-hooks"))
	writeCommit(t, other, "other", "other\n", "other")
	gitTest(t, other, "remote", "add", "target", remote)
	gitTest(t, other, "push", "-q", "target", "main")
	if err := project(option); err == nil || !strings.Contains(err.Error(), "diverges") {
		t.Fatalf("divergence=%v", err)
	}
}

func TestProjectionRemoteValidation(t *testing.T) {
	for _, accepted := range []string{"file:///tmp/repository.git", "/tmp/repository.git", "https://github.com/example/repository.git", "ssh://git@gitlab.example/example/repository.git", "git@forge.example:example/repository.git"} {
		if err := validateProjectionRemote(accepted); err != nil {
			t.Errorf("%s: %v", accepted, err)
		}
	}
	for _, rejected := range []string{"https://", "ssh://", "relative", "https://forge.example/repository?token=x"} {
		if err := validateProjectionRemote(rejected); err == nil {
			t.Errorf("accepted %s", rejected)
		}
	}
}

func TestProjectionUsage(t *testing.T) {
	if err := run([]string{"project"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("usage=%v", err)
	}
	if err := runProjection([]string{"--unknown"}); err == nil {
		t.Fatal("malformed projection flag accepted")
	}
}

func TestProjectionGitFailureBoundaries(t *testing.T) {
	for _, mode := range []string{"status", "config", "rev-parse-symbolic", "rev-parse-commit", "clone", "remote", "ls-remote", "fetch", "merge-base", "push"} {
		t.Run(mode, func(t *testing.T) {
			fixture := forgeFixture(t)
			remote := filepath.Join(t.TempDir(), "remote.git")
			if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
				t.Fatalf("git init bare: %v: %s", err, output)
			}
			gitTest(t, fixture.repository, "remote", "add", "peer", remote)
			if mode == "fetch" || mode == "merge-base" {
				gitTest(t, fixture.repository, "push", "-q", "peer", "main")
			}
			useGitWrapper(t, mode)
			option := projectionOptions{
				repository: fixture.repository, branch: "main", remote: "peer",
				sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
				actorName: "Peer Actor", actorEmail: fixture.email, signingKey: fixture.key,
				sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners,
			}
			if err := project(option); err == nil {
				t.Fatalf("%s failure accepted", mode)
			}
		})
	}
}

func TestProjectionRejectsUntrustedRemoteHistory(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "main")
	rogue := forgeKey(t, "forge@example.invalid")
	option := projectionOptions{
		repository: fixture.repository, branch: "main", remote: "peer",
		sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
		actorName: "Peer Actor", actorEmail: fixture.email, signingKey: fixture.key,
		sourceSigners: fixture.allowedSigners, targetSigners: rogue.allowedSigners,
	}
	if err := project(option); err == nil || !strings.Contains(err.Error(), "No principal matched") {
		t.Fatalf("untrusted remote history=%v", err)
	}
}

func TestProjectionTagVerificationBoundaries(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "refs/tags/v1.2.3:refs/tags/v1.2.3")
	option := projectionOptions{
		repository: fixture.repository, branch: "main", remote: "peer",
		sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
		actorName: "Peer Actor", actorEmail: fixture.email, signingKey: fixture.key,
		sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners,
	}
	repository := bareReplay(t, fixture, option)
	canonical := gitTestOutput(t, fixture.repository, "rev-parse", "main")
	for _, mode := range []string{"ls-remote", "fetch", "cat-file", "rev-parse", "verify-tag"} {
		t.Run(mode, func(t *testing.T) {
			useGitWrapper(t, mode)
			if err := verifyProjectionTags(fixture.repository, repository, canonical, option); err == nil {
				t.Fatalf("%s failure accepted", mode)
			}
		})
	}

	lightweight := "v1.2.4"
	gitTest(t, fixture.repository, "tag", lightweight)
	gitTest(t, fixture.repository, "push", "-q", "peer", "refs/tags/"+lightweight+":refs/tags/"+lightweight)
	if err := verifyProjectionTags(fixture.repository, repository, gitTestOutput(t, fixture.repository, "rev-parse", "main"), option); err == nil || !strings.Contains(err.Error(), "annotated") {
		t.Fatalf("lightweight target tag=%v", err)
	}
}

func bareReplay(t *testing.T, fixture forgeTestFixture, option projectionOptions) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "projection.git")
	if err := replay(options{
		source: fixture.repository, revision: "main", output: repository, ref: "refs/heads/main",
		actorName: option.actorName, actorEmail: option.actorEmail, signingKey: option.signingKey,
		signingProgram: "ssh-keygen", allowedSigners: option.targetSigners,
	}); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "remote", "add", "target", gitTestOutput(t, fixture.repository, "remote", "get-url", "peer"))
	return repository
}
