package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestForgeCommandsReportOutputFailure(t *testing.T) {
	fixture := newForgeFixture(t)
	remote := newBareRepository(t)
	gitTest(t, fixture.repository, "remote", "add", "peer", remote)
	gitTest(t, fixture.repository, "push", "-q", "peer", "main:main")
	commit := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
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
		{"verify commits", []string{"commits", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}},
		{"verify tag", []string{"tag", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}},
		{"verify tags", []string{"tags", "--allowed-signers", fixture.allowedSigners}},
		{"verify refs", []string{"refs", "--remote", "peer", "--expect", "main=" + commit}},
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

func TestRemoteObservationRejectsAnUnknownRemote(t *testing.T) {
	fixture := newForgeFixture(t)
	if _, err := remoteReference(fixture.repository, "missing", "refs/heads/main"); err == nil {
		t.Fatal("missing remote observation accepted")
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
