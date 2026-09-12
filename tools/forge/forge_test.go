package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgeCommandsReportOutputFailureWithoutRevertingPublication(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	commit := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
	tag := gitOutputForTest(t, fixture.repository, "rev-parse", "v1.2.3")
	closed, err := os.CreateTemp(t.TempDir(), "closed-result")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name      string
		arguments []string
	}{
		{"publish branches", []string{"project", "--remote", "peer", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}},
		{"verify commits", []string{"commits", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}},
		{"verify tag", []string{"tag", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}},
		{"verify tags", []string{"tags", "--allowed-signers", fixture.allowedSigners}},
		{"verify refs", []string{"refs", "--remote", "peer", "--expect", "main=" + commit}},
		{"publish tag", []string{"publish-tag", "--remote", "peer", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}},
		{"unchanged tag", []string{"publish-tag", "--remote", "peer", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			original := os.Stdout
			t.Cleanup(func() { os.Stdout = original })
			os.Stdout = closed
			err := run(append(scenario.arguments, "--repository", fixture.repository))
			os.Stdout = original
			if !errors.Is(err, os.ErrClosed) {
				t.Fatalf("result output failure = %v", err)
			}
		})
	}
	for ref, want := range map[string]string{"refs/heads/main": commit, "refs/heads/dev": commit, "refs/tags/v1.2.3": tag} {
		if got := gitOutputForTest(t, remote, "rev-parse", ref); got != want {
			t.Fatalf("report failure changed published %s: %s, want %s", ref, got, want)
		}
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

func TestReferenceVerificationRequiresEveryExpectedRemoteObject(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	want := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
	gitTest(t, fixture.repository, "push", "-q", "peer", "main:main", "main:dev")

	arguments := []string{
		"refs", "--repository", fixture.repository, "--remote", "peer",
		"--expect", "main=" + want, "--expect", "dev=" + want,
	}
	if err := run(arguments); err != nil {
		t.Fatal(err)
	}

	writeCommitForTest(t, fixture.repository, "next", "next\n")
	newer := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
	if err := run([]string{
		"refs", "--repository", fixture.repository, "--remote", "peer",
		"--expect", "main=" + newer, "--expect", "dev=" + newer,
	}); err == nil || !strings.Contains(err.Error(), "remote ") {
		t.Fatalf("stale peer refs accepted: %v", err)
	}
}

func TestReferenceVerificationRejectsInvalidInput(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	want := gitOutputForTest(t, fixture.repository, "rev-parse", "main")

	for name, arguments := range map[string][]string{
		"usage":        {"refs"},
		"remote":       {"refs", "--repository", fixture.repository, "--remote", "missing", "--expect", "main=" + want},
		"expectation":  {"refs", "--repository", fixture.repository, "--remote", "peer", "--expect", "bad"},
		"duplicate":    {"refs", "--repository", fixture.repository, "--remote", "peer", "--expect", "main=" + want, "--expect", "main=" + want},
		"branch":       {"refs", "--repository", fixture.repository, "--remote", "peer", "--expect", "-bad=" + want},
		"not a commit": {"refs", "--repository", fixture.repository, "--remote", "peer", "--expect", "main=deadbeef"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid reference verification accepted")
			}
		})
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

func TestProposalPublicationVerifiesOnlyCommitsAfterTheAcceptedRemoteBase(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "commit", "--amend", "-q", "-m", "old history")
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "main:dev")
	gitTest(t, fixture.repository, "switch", "-q", "-c", "proposal/range")
	writeCommitWithSubjectForTest(t, fixture.repository, "current", "current\n", "fix: verify the proposal range")

	arguments := []string{
		"project", "--repository", fixture.repository,
		"--source", "proposal/range", "--remote", "peer",
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}
	if err := run(arguments); err != nil {
		t.Fatalf("valid proposal range: %v", err)
	}

	writeCommitWithSubjectForTest(t, fixture.repository, "invalid", "invalid\n", "update everything")
	if err := run(arguments); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("invalid proposal subject: %v", err)
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
	}); err == nil || !strings.Contains(err.Error(), "release tag is malformed") {
		t.Fatalf("qualified namespace: %v", err)
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

func TestProjectionRejectsDirtyStatusFailureAndPushFailure(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	base := projectionOptions{repository: fixture.repository, source: "main", remote: "peer", email: fixture.email, allowedSigners: fixture.allowedSigners, expectedTips: map[string]string{}}
	if err := project(projectionOptions{repository: "missing", source: "main", remote: "peer", email: fixture.email, allowedSigners: fixture.allowedSigners}); err == nil {
		t.Fatal("unreadable status accepted")
	}
	hook := filepath.Join(remote, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := project(base); err == nil {
		t.Fatal("rejected peer accepted")
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
	if ancestor, err := isAncestor(fixture.repository, "broken", strings.Repeat("0", 40), gitOutputForTest(t, fixture.repository, "rev-parse", "main")); ancestor || err == nil {
		t.Fatalf("failed fetch was reported as an ancestry result: %t, %v", ancestor, err)
	}
	if ancestor, err := isAncestor(fixture.repository, "peer", gitOutputForTest(t, fixture.repository, "rev-parse", "main"), strings.Repeat("0", 40)); ancestor || err == nil {
		t.Fatalf("invalid comparison was reported as divergence: %t, %v", ancestor, err)
	}
}

func TestAncestryObservationPreservesLocalReferences(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "main:main")
	ancestor := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
	writeCommitForTest(t, fixture.repository, "successor", "successor\n")
	descendant := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
	gitTest(t, fixture.repository, "update-ref", "refs/aigw/forge-observation/main", descendant)
	before := gitOutputForTest(t, fixture.repository, "for-each-ref", "--format=%(refname) %(objectname)")
	fetchHead := filepath.Join(fixture.repository, ".git", "FETCH_HEAD")
	if err := os.WriteFile(fetchHead, []byte("operator fetch observation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if related, err := isAncestor(fixture.repository, "peer", ancestor, descendant); !related || err != nil {
		t.Fatalf("valid ancestry was not established: %t, %v", related, err)
	}
	if after := gitOutputForTest(t, fixture.repository, "for-each-ref", "--format=%(refname) %(objectname)"); after != before {
		t.Fatalf("ancestry observation changed local refs:\nbefore: %s\nafter: %s", before, after)
	}
	if content, err := os.ReadFile(fetchHead); err != nil || string(content) != "operator fetch observation\n" {
		t.Fatalf("ancestry observation replaced FETCH_HEAD: %q, %v", content, err)
	}
	other := newForgeFixture(t)
	gitTest(t, other.repository, "remote", "add", "peer", remote)
	gitTest(t, other.repository, "push", "-q", "peer", "main:other")
	foreign := gitOutputForTest(t, other.repository, "rev-parse", "main")
	if related, err := isAncestor(fixture.repository, "peer", foreign, descendant); related || err != nil {
		t.Fatalf("fetched peer divergence was not established: %t, %v", related, err)
	}
	if after := gitOutputForTest(t, fixture.repository, "for-each-ref", "--format=%(refname) %(objectname)"); after != before {
		t.Fatalf("object fetch changed local refs:\nbefore: %s\nafter: %s", before, after)
	}
	if content, err := os.ReadFile(fetchHead); err != nil || string(content) != "operator fetch observation\n" {
		t.Fatalf("object fetch replaced FETCH_HEAD: %q, %v", content, err)
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
	setHostileGitHooks(t)
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
