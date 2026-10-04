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

func TestCandidateAcceptanceSeparatesExplicitProductAndVerifierCommits(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "absent-global-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	artifacts := prepareSignedRelease(t, "0.1.0")
	if output, err := exec.Command("git", "config", "--local", "user.useConfigOnly", "true").CombinedOutput(); err != nil {
		t.Fatalf("require explicit fixture identity: %v: %s", err, output)
	}
	t.Setenv("CI_COMMIT_TAG", "")
	if output, err := exec.Command("git", "tag", "-d", "v0.1.0").CombinedOutput(); err != nil {
		t.Fatalf("remove fixture release tag: %v: %s", err, output)
	}
	commit, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	selected := strings.TrimSpace(string(commit))
	command := exec.Command("git", "-c", "core.hooksPath=.git/hooks", "-c", "commit.gpgsign=false",
		"-c", "user.name=Verifier Test", "-c", "user.email=verifier@test.invalid",
		"commit", "--allow-empty", "-m", "test: independent verifier")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("advance verifier: %v: %s", err, output)
	}
	want := gzip.ErrHeader
	if runtime.GOOS == "windows" {
		want = zip.ErrFormat
	}
	args := []string{"accept-native", "--artifacts", artifacts, "--candidate", "--candidate-source", selected}
	if err := run(args, io.Discard); !errors.Is(err, want) {
		t.Fatalf("explicit signed product did not reach supplied archive under another verifier: %v", err)
	}
	if output, err := exec.Command("git", "-c", "tag.gpgsign=false", "tag", "-a", "native-test-alias", selected, "-m", "Not a commit object").CombinedOutput(); err != nil {
		t.Fatalf("create fixture tag object: %v: %s", err, output)
	}
	alias, err := exec.Command("git", "rev-parse", "refs/tags/native-test-alias").Output()
	if err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = strings.TrimSpace(string(alias))
	if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "does not identify the exact commit object") {
		t.Fatalf("tag-object alias reached exact-commit acceptance: %v", err)
	}
	for _, invalid := range []string{"HEAD", selected[:12], strings.ToUpper(selected), strings.Repeat("z", 40), selected + "0"} {
		args[len(args)-1] = invalid
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "exact lowercase Git commit ID") {
			t.Fatalf("invalid candidate identity %q reached execution: %v", invalid, err)
		}
	}
	unsigned, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = strings.TrimSpace(string(unsigned))
	if err := run(args, io.Discard); err == nil || errors.Is(err, want) {
		t.Fatalf("unsigned selected product reached execution: %v", err)
	}
}

func TestCandidateAcceptanceRequiresExplicitArtifactAndNoTag(t *testing.T) {
	if err := run([]string{"accept-native", "--candidate-source", "HEAD"}, io.Discard); err == nil || !strings.Contains(err.Error(), "candidate source requires --candidate") {
		t.Fatalf("candidate source without candidate mode: %v", err)
	}
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

func TestNativePublishedBaselineBindsItsExplicitSignedSource(t *testing.T) {
	artifacts := prepareSignedRelease(t, "0.1.0")
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	want := gzip.ErrHeader
	if runtime.GOOS == "windows" {
		want = zip.ErrFormat
	}
	err := run([]string{"accept-native", "--baseline-artifacts", artifacts, "--baseline-tag", "v0.1.0"}, io.Discard)
	if !errors.Is(err, want) {
		t.Fatalf("authorized baseline did not reach exact archive decoding: %v", err)
	}
	for _, tag := range []string{"", "v9.9.9", "not-a-tag"} {
		err := run([]string{"accept-native", "--baseline-artifacts", artifacts, "--baseline-tag", tag}, io.Discard)
		if err == nil || errors.Is(err, want) {
			t.Fatalf("unbound baseline tag %q reached archive decoding: %v", tag, err)
		}
	}
	if err := os.WriteFile(filepath.Join(artifacts, "aigw_0.1.0.provenance.json"), []byte("unbound provenance"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"accept-native", "--baseline-artifacts", artifacts, "--baseline-tag", "v0.1.0"}, io.Discard); err == nil || errors.Is(err, want) {
		t.Fatalf("corrupt baseline provenance reached archive decoding: %v", err)
	}
}

func TestNativeClientAcceptanceRejectsMissingExecutableBeforeBuild(t *testing.T) {
	t.Chdir(filepath.Clean(filepath.Join("..", "..")))
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"AIGW_ACCEPTANCE_CLAUDE", "AIGW_ACCEPTANCE_CODEX", "AIGW_ACCEPTANCE_HERMES"}
	for _, missing := range keys {
		t.Run(missing, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, executable)
			}
			t.Setenv(missing, "")
			err := run([]string{"accept-native", "--clients", "--baseline-tag", "v0.3.1", "--peer", "github", "--repository", "team/product"}, io.Discard)
			if err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("missing native client was not identified before construction: %v", err)
			}
		})
	}
}
