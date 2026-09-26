package main

import (
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/tools/release/artifact"
)

func TestUntaggedCandidateArtifactAcceptanceBindsSignedHead(t *testing.T) {
	artifacts := prepareSignedRelease(t, "0.1.0")
	t.Setenv("CI_COMMIT_TAG", "")
	if output, err := exec.Command("git", "tag", "-d", "v0.1.0").CombinedOutput(); err != nil {
		t.Fatalf("remove fixture-only release tag: %v: %s", err, output)
	}
	source := artifact.SourceTrust{Repository: ".", AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}
	if err := artifact.VerifyCandidateProvenance(t.Context(), artifacts, source); err != nil {
		t.Fatalf("signed untagged candidate verification: %v", err)
	}
	if err := run([]string{"verify-artifacts", artifacts}, io.Discard); err == nil || !strings.Contains(err.Error(), "requires v<semver> tag") {
		t.Fatalf("release verification silently selected candidate mode: %v", err)
	}
	if err := artifact.VerifyProvenance(t.Context(), artifacts, "", source); err == nil || !strings.Contains(err.Error(), "requires v<semver> tag") {
		t.Fatalf("publication provenance admitted an untagged candidate: %v", err)
	}
	want := gzip.ErrHeader
	if runtime.GOOS == "windows" {
		want = zip.ErrFormat
	}
	if err := run([]string{"accept-native", "--artifacts", artifacts, "--candidate"}, io.Discard); !errors.Is(err, want) {
		t.Fatalf("candidate did not reach supplied archive decoding: %v", err)
	}
	baseline := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(baseline, []byte("baseline"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", baseline)
	performance := filepath.Join(t.TempDir(), "measurements")
	if err := run([]string{"accept-native", "--artifacts", artifacts, "--candidate", "--performance", performance}, io.Discard); !errors.Is(err, want) {
		t.Fatalf("pre-tag performance did not reach supplied archive decoding: %v", err)
	}
	command := exec.Command("git", "-c", "core.hooksPath=.git/hooks", "-c", "commit.gpgsign=false",
		"-c", "user.name=Verifier Test", "-c", "user.email=verifier@test.invalid",
		"commit", "--allow-empty", "-m", "test: unsigned successor")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("advance candidate source: %v: %s", err, output)
	}
	if err := artifact.VerifyCandidateProvenance(t.Context(), artifacts, source); err == nil || !strings.Contains(err.Error(), "verify-commit") {
		t.Fatalf("candidate verification accepted a different unsigned HEAD: %v", err)
	}
}

func TestCandidateArtifactVerificationRejectsExistingReleaseTag(t *testing.T) {
	artifacts := prepareSignedRelease(t, "0.1.0")
	t.Setenv("CI_COMMIT_TAG", "")
	for _, args := range [][]string{{"tag", "-d", "v0.1.0"}, {"tag", "v0.1.0"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("prepare invalid fixture tag: %v: %s", err, output)
		}
	}
	source := artifact.SourceTrust{Repository: ".", AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}
	if err := artifact.VerifyCandidateProvenance(t.Context(), artifacts, source); err == nil || !strings.Contains(err.Error(), "candidate provenance requires an untagged VERSION") {
		t.Fatalf("candidate mode bypassed an existing release tag: %v", err)
	}
	if err := run([]string{"accept-native", "--artifacts", artifacts, "--candidate"}, io.Discard); err == nil || !strings.Contains(err.Error(), "candidate provenance requires an untagged VERSION") {
		t.Fatalf("native candidate acceptance bypassed an existing release tag: %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "v0.1.0")
	if err := run([]string{"verify-artifacts", artifacts}, io.Discard); err == nil || !strings.Contains(err.Error(), "verify-tag") {
		t.Fatalf("tag mode accepted an unsigned release tag: %v", err)
	}
}

func TestCandidateAcceptanceRequiresExplicitArtifactAndNoTag(t *testing.T) {
	if err := run([]string{"accept-native", "--candidate"}, io.Discard); err == nil || !strings.Contains(err.Error(), "candidate acceptance requires --artifacts") {
		t.Fatalf("candidate mode without artifact input: %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "v0.1.0")
	if err := run([]string{"accept-native", "--artifacts", t.TempDir(), "--candidate"}, io.Discard); err == nil || !strings.Contains(err.Error(), "candidate acceptance cannot select a release tag") {
		t.Fatalf("candidate mode with release tag: %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "")
	t.Setenv("GITHUB_REF_TYPE", "tag")
	t.Setenv("GITHUB_REF_NAME", "v0.1.0")
	if err := run([]string{"accept-native", "--artifacts", t.TempDir(), "--candidate"}, io.Discard); err == nil || !strings.Contains(err.Error(), "candidate acceptance cannot select a release tag") {
		t.Fatalf("candidate mode with GitHub release tag: %v", err)
	}
}
