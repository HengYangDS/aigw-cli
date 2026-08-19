package main

import (
	"bytes"
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

func TestProductObjectVerificationHasNoForgeIdentity(t *testing.T) {
	fixture := newForgeFixture(t)

	if err := run([]string{
		"commits", "--repository", fixture.repository,
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"tag", "--repository", fixture.repository,
		"--tag", "v1.2.3",
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"tags", "--repository", fixture.repository,
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRemovedHistoryAndLifecycleCommandsStayAbsent(t *testing.T) {
	for _, command := range []string{"replay", "sync", "closeout", "promote-release"} {
		t.Run(command, func(t *testing.T) {
			if err := run([]string{command}); err == nil || !strings.Contains(err.Error(), "unknown forge command") {
				t.Fatalf("removed command %q: %v", command, err)
			}
		})
	}
}

func TestMainPublicationPreservesOneExactCommit(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	source := gitOutputForTest(t, fixture.repository, "rev-parse", "refs/heads/main")

	arguments := []string{
		"project", "--repository", fixture.repository,
		"--source", "main", "--remote", "peer",
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}
	if err := run(arguments); err != nil {
		t.Fatal(err)
	}
	if err := run(arguments); err != nil {
		t.Fatalf("idempotent publication: %v", err)
	}
	for _, branch := range []string{"main", "dev"} {
		if got := gitOutputForTest(t, remote, "rev-parse", "refs/heads/"+branch); got != source {
			t.Fatalf("%s=%s, want exact local %s", branch, got, source)
		}
	}
}

func TestProposalPublicationUsesOnlyItsMatchingRef(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "branch", "proposal/exact-objects", "main")
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)

	if err := run([]string{
		"project", "--repository", fixture.repository,
		"--source", "proposal/exact-objects", "--remote", "peer",
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
	want := gitOutputForTest(t, fixture.repository, "rev-parse", "refs/heads/proposal/exact-objects")
	if got := gitOutputForTest(t, remote, "rev-parse", "refs/heads/proposal/exact-objects"); got != want {
		t.Fatalf("proposal=%s, want %s", got, want)
	}
	if output, err := exec.Command("git", "-C", remote, "show-ref", "--verify", "refs/heads/main").CombinedOutput(); err == nil {
		t.Fatalf("proposal publication created main: %s", output)
	}
}

func TestDivergentPublicationRequiresFreshExactLeases(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	seedRemoteBranches(t, remote)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	oldMain := gitOutputForTest(t, remote, "rev-parse", "refs/heads/main")
	oldDev := gitOutputForTest(t, remote, "rev-parse", "refs/heads/dev")

	base := []string{
		"project", "--repository", fixture.repository,
		"--source", "main", "--remote", "peer",
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}
	if err := run(base); err == nil || !strings.Contains(err.Error(), "exact expected tip") {
		t.Fatalf("divergence without lease: %v", err)
	}
	arguments := append(append([]string(nil), base...),
		"--expect-remote-tip", "main="+oldMain,
		"--expect-remote-tip", "dev="+oldDev,
	)
	if err := run(arguments); err != nil {
		t.Fatal(err)
	}
	want := gitOutputForTest(t, fixture.repository, "rev-parse", "refs/heads/main")
	for _, branch := range []string{"main", "dev"} {
		if got := gitOutputForTest(t, remote, "rev-parse", "refs/heads/"+branch); got != want {
			t.Fatalf("%s=%s, want %s", branch, got, want)
		}
	}
}

func TestPublicationRejectsNonPublicBranchesAndDirtySource(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	base := []string{
		"project", "--repository", fixture.repository,
		"--remote", "peer", "--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}
	for _, branch := range []string{"dev", "candidate/dev", "work/change", "feature"} {
		arguments := append(append([]string(nil), base...), "--source", branch)
		if err := run(arguments); err == nil || !strings.Contains(err.Error(), "main or proposal") {
			t.Fatalf("branch %q: %v", branch, err)
		}
	}
	if err := os.WriteFile(filepath.Join(fixture.repository, "dirty"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	arguments := append(append([]string(nil), base...), "--source", "main")
	if err := run(arguments); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty publication: %v", err)
	}
}

func TestTagPublicationPreservesExactAnnotatedTagObject(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	arguments := []string{
		"publish-tag", "--repository", fixture.repository,
		"--remote", "peer", "--tag", "v1.2.3",
		"--allowed-signers", fixture.allowedSigners,
	}
	if err := run(arguments); err != nil {
		t.Fatal(err)
	}
	if err := run(arguments); err != nil {
		t.Fatalf("idempotent tag publication: %v", err)
	}
	want := gitOutputForTest(t, fixture.repository, "rev-parse", "refs/tags/v1.2.3")
	if got := gitOutputForTest(t, remote, "rev-parse", "refs/tags/v1.2.3"); got != want {
		t.Fatalf("tag=%s, want exact local object %s", got, want)
	}
}

func TestQualifiedTagNamespaceIsRejected(t *testing.T) {
	fixture := newForgeFixture(t)
	gitTest(t, fixture.repository, "tag", "-s", "-a", "github/v1.2.4", "-m", "obsolete")
	if err := run([]string{
		"tags", "--repository", fixture.repository,
		"--allowed-signers", fixture.allowedSigners,
	}); err == nil || !strings.Contains(err.Error(), "unexpected release tag") {
		t.Fatalf("qualified namespace: %v", err)
	}
}

func TestCommitVerificationRejectsInvalidInputsAndHistory(t *testing.T) {
	fixture := newForgeFixture(t)
	for name, arguments := range map[string][]string{
		"usage":      {"commits"},
		"email":      {"commits", "--email", "invalid", "--allowed-signers", fixture.allowedSigners},
		"trust":      {"commits", "--email", fixture.email, "--allowed-signers", "missing"},
		"repository": {"commits", "--repository", t.TempDir(), "--email", fixture.email, "--allowed-signers", fixture.allowedSigners},
		"revision":   {"commits", "--repository", fixture.repository, "--revision", "missing", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid commit verification accepted")
			}
		})
	}
	if err := os.WriteFile(filepath.Join(fixture.repository, ".mailmap"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), ".mailmap") {
		t.Fatalf("mailmap: %v", err)
	}
	if err := os.Remove(filepath.Join(fixture.repository, ".mailmap")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, fixture.repository, "config", "user.email", "wrong@example.invalid")
	writeCommitForTest(t, fixture.repository, "drift", "drift\n")
	if err := run([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "must use") {
		t.Fatalf("identity drift: %v", err)
	}
}

func TestTagVerificationRejectsInvalidShapesAndTrust(t *testing.T) {
	fixture := newForgeFixture(t)
	for name, arguments := range map[string][]string{
		"usage":      {"tag"},
		"malformed":  {"tag", "--repository", fixture.repository, "--tag", "latest", "--allowed-signers", fixture.allowedSigners},
		"missing":    {"tag", "--repository", fixture.repository, "--tag", "v9.9.9", "--allowed-signers", fixture.allowedSigners},
		"trust":      {"tag", "--repository", fixture.repository, "--tag", "v1.2.3", "--allowed-signers", "missing"},
		"tags usage": {"tags"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid tag verification accepted")
			}
		})
	}
	gitTest(t, fixture.repository, "-c", "tag.gpgsign=false", "tag", "v1.2.4")
	if err := run([]string{"tag", "--repository", fixture.repository, "--tag", "v1.2.4", "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "annotated") {
		t.Fatalf("lightweight tag: %v", err)
	}
	rogue := newSigningIdentity(t, "rogue@example.invalid")
	if err := run([]string{"tag", "--repository", fixture.repository, "--tag", "v1.2.3", "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted tag: %v", err)
	}
}

func TestProjectionRejectsInvalidInputsAndCoordinates(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	base := []string{"project", "--repository", fixture.repository, "--remote", "peer", "--source", "main", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}
	for name, arguments := range map[string][]string{
		"usage":        {"project"},
		"expected":     append(append([]string(nil), base...), "--expect-remote-tip", "bad"),
		"duplicate":    append(append([]string(nil), base...), "--expect-remote-tip", "main=one", "--expect-remote-tip", "main=two"),
		"remote":       {"project", "--repository", fixture.repository, "--remote", "missing", "--source", "main", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners},
		"source":       {"project", "--repository", fixture.repository, "--remote", "peer", "--source", "proposal/missing", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners},
		"commit trust": {"project", "--repository", fixture.repository, "--remote", "peer", "--source", "main", "--email", fixture.email, "--allowed-signers", "missing"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid projection accepted")
			}
		})
	}
}

func TestProjectionSupportsFastForwardAndRejectsStaleLease(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "main:main", "main:dev")
	writeCommitForTest(t, fixture.repository, "next", "next\n")
	arguments := []string{"project", "--repository", fixture.repository, "--remote", "peer", "--source", "main", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}
	if err := run(arguments); err != nil {
		t.Fatalf("fast-forward: %v", err)
	}

	other := newForgeFixture(t)
	gitTest(t, other.repository, "remote", "add", "peer", remote)
	stale := strings.Repeat("0", 40)
	arguments = []string{"project", "--repository", other.repository, "--remote", "peer", "--source", "main", "--email", other.email, "--allowed-signers", other.allowedSigners, "--expect-remote-tip", "main=" + stale, "--expect-remote-tip", "dev=" + stale}
	if err := run(arguments); err == nil || !strings.Contains(err.Error(), "exact expected tip") {
		t.Fatalf("stale lease: %v", err)
	}
}

func TestTagPublicationRejectsInvalidAndDivergentState(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	if err := run([]string{"publish-tag"}); err == nil {
		t.Fatal("incomplete tag publication accepted")
	}
	gitTest(t, fixture.repository, "push", "-q", "peer", "refs/tags/v1.2.3:refs/tags/v1.2.3")
	gitTest(t, fixture.repository, "tag", "-d", "v1.2.3")
	gitTest(t, fixture.repository, "tag", "-s", "-a", "v1.2.3", "-m", "replacement")
	arguments := []string{"publish-tag", "--repository", fixture.repository, "--remote", "peer", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}
	if err := run(arguments); err == nil || !strings.Contains(err.Error(), "exact expected object") {
		t.Fatalf("divergent tag: %v", err)
	}
	old := gitOutputForTest(t, remote, "rev-parse", "refs/tags/v1.2.3")
	arguments = append(arguments, "--expect-remote-tag", old)
	if err := run(arguments); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteAndGitFailureSurfaces(t *testing.T) {
	var stderr bytes.Buffer
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if code := execute([]string{"unknown"}, write); code != 1 {
		t.Fatalf("code=%d", code)
	}
	_ = write.Close()
	_, _ = stderr.ReadFrom(read)
	_ = read.Close()
	if !strings.Contains(stderr.String(), "unknown forge command") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if _, err := gitBytes(t.TempDir(), "not-a-command"); err == nil {
		t.Fatal("Git failure hidden")
	}
}

func TestEmptyInvocationAndSuccessfulExecute(t *testing.T) {
	if err := run(nil); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("empty invocation: %v", err)
	}
	fixture := newForgeFixture(t)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := execute([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}, write)
	_ = write.Close()
	_ = read.Close()
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestCommitVerificationRejectsUntrustedSignature(t *testing.T) {
	fixture := newForgeFixture(t)
	rogue := newSigningIdentity(t, fixture.email)
	if err := run([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted commit: %v", err)
	}
}

func TestEmptyTagSetAndUntrustedTagSet(t *testing.T) {
	fixture := newForgeFixture(t)
	gitTest(t, fixture.repository, "tag", "-d", "v1.2.3")
	if err := run([]string{"tags", "--repository", fixture.repository, "--allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatalf("empty tag set: %v", err)
	}
	gitTest(t, fixture.repository, "tag", "-s", "-a", "v1.2.3", "-m", "release")
	rogue := newSigningIdentity(t, fixture.email)
	if err := run([]string{"tags", "--repository", fixture.repository, "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted tag set: %v", err)
	}
}

func TestProjectionRejectsDirtyStatusFailureAndPushFailure(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	base := projectionOptions{repository: fixture.repository, source: "main", remote: "peer", email: fixture.email, allowedSigners: fixture.allowedSigners, expectedTips: map[string]string{}}
	if err := project(projectionOptions{repository: "missing", source: "main", remote: "peer", email: fixture.email, allowedSigners: fixture.allowedSigners}); err == nil {
		t.Fatal("unreadable status accepted")
	}
	if err := os.Chmod(remote, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(remote, 0o700) })
	if err := project(base); err == nil {
		t.Fatal("unwritable peer accepted")
	}
}

func TestTagPublicationRejectsMissingRemoteAndInvalidTag(t *testing.T) {
	fixture := newForgeFixture(t)
	for name, arguments := range map[string][]string{
		"remote": {"publish-tag", "--repository", fixture.repository, "--remote", "missing", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners},
		"tag":    {"publish-tag", "--repository", fixture.repository, "--remote", "missing", "--tag", "latest", "--allowed-signers", fixture.allowedSigners},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid tag publication accepted")
			}
		})
	}
}

func TestRemoteObservationAndAncestryFailureBoundaries(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	if _, err := remoteReference(fixture.repository, "missing", "refs/heads/main"); err == nil {
		t.Fatal("missing remote observation accepted")
	}
	gitTest(t, fixture.repository, "remote", "add", "broken", filepath.Join(t.TempDir(), "absent.git"))
	if isAncestor(fixture.repository, "broken", "main", strings.Repeat("0", 40), gitOutputForTest(t, fixture.repository, "rev-parse", "main")) {
		t.Fatal("failed fetch reported ancestry")
	}
}

func TestRemoteObservationRejectsMalformedRows(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "main:refs/heads/main", "main:refs/tags/main")
	if _, err := remoteReference(fixture.repository, "peer", "main"); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("ambiguous remote row: %v", err)
	}
}

func TestProjectionRejectsRemoteObservationFailure(t *testing.T) {
	fixture := newForgeFixture(t)
	invalid := filepath.Join(t.TempDir(), "invalid")
	if err := os.WriteFile(invalid, []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", invalid)
	if err := project(projectionOptions{repository: fixture.repository, source: "main", remote: "peer", email: fixture.email, allowedSigners: fixture.allowedSigners}); err == nil {
		t.Fatal("remote observation failure accepted")
	}
}

func TestTagPublicationRejectsMissingLocalTagAndMissingTrust(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	if err := run([]string{"publish-tag", "--repository", fixture.repository, "--remote", "peer", "--tag", "v9.9.9", "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing local tag: %v", err)
	}
	if err := run([]string{"publish-tag", "--repository", fixture.repository, "--remote", "peer", "--tag", "v1.2.3", "--allowed-signers", "missing"}); err == nil || !strings.Contains(err.Error(), "trust input") {
		t.Fatalf("missing trust: %v", err)
	}
}

func TestTagPublicationRejectsInvalidPeerRepository(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := filepath.Join(t.TempDir(), "not-a-repository")
	if err := os.WriteFile(remote, []byte("not git\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	if err := run([]string{"publish-tag", "--repository", fixture.repository, "--remote", "peer", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}); err == nil {
		t.Fatal("invalid tag peer accepted")
	}
}

func TestPublicationRejectsPeerHookMutation(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	if err := os.WriteFile(filepath.Join(remote, "hooks", "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := project(projectionOptions{repository: fixture.repository, source: "main", remote: "peer", email: fixture.email, allowedSigners: fixture.allowedSigners}); err == nil {
		t.Fatal("rejected branch push reported success")
	}
	if err := run([]string{"publish-tag", "--repository", fixture.repository, "--remote", "peer", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}); err == nil {
		t.Fatal("rejected tag push reported success")
	}
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
	if err := os.WriteFile(filepath.Join(repository, "file"), []byte("value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "file")
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
	return repository
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
