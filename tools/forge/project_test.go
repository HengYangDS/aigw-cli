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
	gitTest(t, fixture.repository, "branch", "dev", "main")
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

func TestProjectionAdvancesMainAndDevWithOneAtomicPush(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	log := filepath.Join(t.TempDir(), "git.log")
	useGitWrapper(t, "record")
	t.Setenv("AIGW_TEST_GIT_LOG", log)
	if err := project(projectionOption(fixture, "main", "peer")); err != nil {
		raw, _ := os.ReadFile(log)
		t.Fatalf("%v\n%s", err, raw)
	}
	for _, branch := range []string{"main", "dev"} {
		if tip := gitTestOutput(t, remote, "rev-parse", "refs/heads/"+branch); tip == "" {
			t.Fatalf("projection did not create %s", branch)
		}
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var pushes []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.Contains(line, " push ") {
			pushes = append(pushes, line)
		}
	}
	if len(pushes) != 1 || !strings.Contains(pushes[0], "--atomic") || !strings.Contains(pushes[0], ":refs/heads/main") || !strings.Contains(pushes[0], ":refs/heads/dev") {
		t.Fatalf("expected one atomic main/dev push, got %q", pushes)
	}
}

func TestPromoteReleaseAdvancesMainToExactDevTip(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
	gitTest(t, fixture.repository, "switch", "-q", "dev")
	writeCommit(t, fixture.repository, "release.txt", "ready\n", "prepare release")
	dev := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/dev")

	if err := promoteRelease(releasePromotionOptions{
		repository: fixture.repository,
		expectMain: gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main"),
		expectDev:  dev,
	}); err != nil {
		t.Fatal(err)
	}
	if main := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main"); main != dev {
		t.Fatalf("main=%s dev=%s", main, dev)
	}
}

func TestPromoteReleaseFailsClosedWithoutChangingMain(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
	gitTest(t, fixture.repository, "switch", "-q", "dev")
	writeCommit(t, fixture.repository, "release.txt", "ready\n", "prepare release")
	main := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
	dev := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/dev")

	for name, option := range map[string]releasePromotionOptions{
		"stale main": {repository: fixture.repository, expectMain: strings.Repeat("0", 40), expectDev: dev},
		"stale dev":  {repository: fixture.repository, expectMain: main, expectDev: strings.Repeat("0", 40)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := promoteRelease(option); err == nil {
				t.Fatal("stale release coordinates accepted")
			}
			if current := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main"); current != main {
				t.Fatalf("main moved: %s != %s", current, main)
			}
		})
	}

	gitTest(t, fixture.repository, "switch", "-q", "main")
	writeCommit(t, fixture.repository, "diverged.txt", "diverged\n", "diverge main")
	diverged := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
	if err := promoteRelease(releasePromotionOptions{repository: fixture.repository, expectMain: diverged, expectDev: dev}); err == nil {
		t.Fatal("diverged main accepted")
	}
	if current := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main"); current != diverged {
		t.Fatalf("diverged main moved: %s != %s", current, diverged)
	}
}

func TestPromoteReleaseCommandRequiresExactCoordinates(t *testing.T) {
	fixture := forgeFixture(t)
	if err := run([]string{"promote-release", "--repository", fixture.repository}); err == nil || !strings.Contains(err.Error(), "expect-main") {
		t.Fatalf("missing exact coordinates=%v", err)
	}
	if err := runReleasePromotion([]string{"--unknown"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("malformed promotion flags=%v", err)
	}
	if err := runReleasePromotion([]string{"--repository", fixture.repository, "--expect-main", "main", "--expect-dev", "dev", "extra"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("promotion positional argument=%v", err)
	}
	gitTest(t, fixture.repository, "branch", "dev", "main")
	tip := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
	if err := runReleasePromotion([]string{"--repository", fixture.repository, "--expect-main", tip, "--expect-dev", tip}); err != nil {
		t.Fatal(err)
	}
}

func TestPromoteReleaseRejectsDirtyRepository(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
	main := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
	if err := os.WriteFile(filepath.Join(fixture.repository, "dirty"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := promoteRelease(releasePromotionOptions{repository: fixture.repository, expectMain: main, expectDev: main}); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty repository=%v", err)
	}
}

func TestPromoteReleasePropagatesGitFailures(t *testing.T) {
	for _, mode := range []string{"status", "rev-parse-commit", "merge-base"} {
		t.Run(mode, func(t *testing.T) {
			fixture := forgeFixture(t)
			gitTest(t, fixture.repository, "branch", "dev", "main")
			main := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
			dev := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/dev")
			useGitWrapper(t, mode)
			if err := promoteRelease(releasePromotionOptions{repository: fixture.repository, expectMain: main, expectDev: dev}); err == nil {
				t.Fatalf("%s failure accepted", mode)
			}
		})
	}
	fixture := forgeFixture(t)
	tip := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
	if err := promoteRelease(releasePromotionOptions{repository: fixture.repository, expectMain: tip, expectDev: tip}); err == nil {
		t.Fatal("missing dev branch accepted")
	}
}

func TestProjectionRejectsNonRepository(t *testing.T) {
	if err := project(projectionOptions{repository: t.TempDir(), branch: "main"}); err == nil || !strings.Contains(err.Error(), "run inside a Git worktree") {
		t.Fatalf("non-repository projection=%v", err)
	}
}

func TestPromoteReleaseIsIdempotentAtAcceptedTip(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
	tip := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main")
	if err := promoteRelease(releasePromotionOptions{repository: fixture.repository, expectMain: tip, expectDev: tip}); err != nil {
		t.Fatal(err)
	}
	if main := gitTestOutput(t, fixture.repository, "rev-parse", "refs/heads/main"); main != tip {
		t.Fatalf("main=%s tip=%s", main, tip)
	}
}

func TestProjectionPreflightsMainAndDevBeforeAnyPush(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	log := filepath.Join(t.TempDir(), "git.log")
	useGitWrapper(t, "fail-dev-preflight")
	t.Setenv("AIGW_TEST_GIT_LOG", log)
	if err := project(projectionOption(fixture, "main", "peer")); err == nil {
		t.Fatal("dev preflight failure accepted")
	}
	if raw, err := os.ReadFile(log); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, " push ") {
				t.Fatalf("push occurred before all branch preconditions passed: %s", raw)
			}
		}
	}
	for _, branch := range []string{"main", "dev"} {
		command := exec.Command("git", "-C", remote, "show-ref", "--verify", "refs/heads/"+branch)
		if output, err := command.CombinedOutput(); err == nil {
			t.Fatalf("%s moved despite failed transaction preflight: %s", branch, output)
		}
	}
}

func TestProjectionBranchPolicy(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	for _, branch := range []string{"work/change", "candidate/dev", "feature/freeform"} {
		t.Run(strings.ReplaceAll(branch, "/", "_"), func(t *testing.T) {
			gitTest(t, fixture.repository, "branch", branch, "main")
			option := projectionOption(fixture, branch, "peer")
			if err := project(option); err == nil || !strings.Contains(err.Error(), "main or proposal") {
				t.Fatalf("branch %s: %v", branch, err)
			}
		})
	}
}

func projectionOption(fixture forgeTestFixture, branch, remote string) projectionOptions {
	return projectionOptions{
		repository: fixture.repository, branch: branch, remote: remote,
		sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
		actorName: "Peer Actor", actorEmail: fixture.email,
		signingKey: fixture.key, signingProgram: "ssh-keygen",
		sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners,
	}
}

func TestProjectionCreatesNonDefaultTargetBranch(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	branch := "proposal/semantic-boundaries"
	gitTest(t, fixture.repository, "branch", branch, "main")
	gitTest(t, fixture.repository, "config", "user.email", "other@example.invalid")
	writeCommit(t, fixture.repository, "main-only", "main only\n", "main only")
	gitTest(t, fixture.repository, "remote", "add", "github", remote)
	option := projectionOptions{
		repository: fixture.repository, branch: branch, remote: "github",
		sourceProvider: "gitlab", targetProvider: "github", sourceEmail: fixture.email,
		actorName: "GitHub Actor", actorEmail: fixture.email,
		signingKey: fixture.key, signingProgram: "ssh-keygen",
		sourceSigners: fixture.allowedSigners, targetSigners: fixture.allowedSigners,
	}
	if err := project(option); err != nil {
		t.Fatal(err)
	}
	if err := project(option); err != nil {
		t.Fatal(err)
	}
	if projected := gitTestOutput(t, remote, "rev-parse", "refs/heads/"+branch); projected == "" {
		t.Fatal("projection did not create non-default target branch")
	}
}

func TestProjectionCommandAndProviderBoundary(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev", "main")
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

func TestProjectionRejectsMissingDevAndAtomicPushFailure(t *testing.T) {
	fixture := forgeFixture(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init bare: %v: %s", err, output)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	option := projectionOption(fixture, "main", "peer")
	if err := project(option); err == nil || !strings.Contains(err.Error(), "source ref is unavailable") {
		t.Fatalf("missing dev branch=%v", err)
	}
	gitTest(t, fixture.repository, "branch", "dev", "main")
	useGitWrapper(t, "push")
	if err := project(option); err == nil {
		t.Fatal("atomic push failure accepted")
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
	local := filepath.Join(t.TempDir(), "repository.git")
	for _, accepted := range []string{"file://" + filepath.ToSlash(local), local, "https://github.com/example/repository.git", "ssh://git@gitlab.example/example/repository.git", "git@forge.example:example/repository.git"} {
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
