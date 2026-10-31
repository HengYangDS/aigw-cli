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
)

func TestVerifyArtifactsWithPublicTrust(t *testing.T) {
	artifacts := prepareSignedRelease(t, "0.1.0")
	for _, name := range []string{"GH_TOKEN", "GITLAB_TOKEN", "CI_JOB_TOKEN", "SSH_AUTH_SOCK"} {
		t.Setenv(name, "")
	}
	if err := run([]string{"verify-artifacts", artifacts}, io.Discard); err != nil {
		t.Fatalf("offline verification without signing or publication credentials: %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "v9.9.9")
	if err := run([]string{"verify-artifacts", artifacts}, io.Discard); err == nil {
		t.Fatal("verification accepted a different release tag")
	}
	t.Setenv("CI_COMMIT_TAG", "")
	t.Setenv("GITHUB_REF_TYPE", "tag")
	t.Setenv("GITHUB_REF_NAME", "v0.1.0")
	if err := run([]string{"verify-artifacts", artifacts}, io.Discard); err != nil {
		t.Fatalf("GitHub tag selection failed to verify release provenance: %v", err)
	}
}

func TestNativeArtifactAcceptanceRequiresPublicTrustBeforeExecution(t *testing.T) {
	artifacts := prepareSignedRelease(t, "0.1.0")
	t.Run("current source must match the release", func(t *testing.T) {
		before := readFile(t, "go.mod")
		if err := os.WriteFile("go.mod", []byte("module example.invalid/changed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.WriteFile("go.mod", before, 0o600); err != nil {
				t.Fatal(err)
			}
		}()
		if err := run([]string{"accept-native", "--artifacts", artifacts}, io.Discard); err == nil || !strings.Contains(err.Error(), "committed source") {
			t.Fatalf("changed release source admission = %v", err)
		}
	})
	t.Setenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE", "")
	if err := run([]string{"accept-native", "--artifacts", artifacts}, io.Discard); err == nil || !strings.Contains(err.Error(), "artifact authorization") {
		t.Fatalf("native artifact acceptance bypassed public trust: %v", err)
	}
}

func TestNativeArtifactAcceptanceSeparatesVerifierAndProductRevisions(t *testing.T) {
	artifacts := prepareSignedRelease(t, "0.1.0")
	command := exec.Command("git", "-c", "core.hooksPath=.git/hooks", "-c", "commit.gpgsign=false",
		"-c", "user.name=Verifier Test", "-c", "user.email=verifier@test.invalid",
		"commit", "--allow-empty", "-m", "test: independent verifier revision")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("advance verifier revision: %v: %s", err, output)
	}
	want := gzip.ErrHeader
	if runtime.GOOS == "windows" {
		want = zip.ErrFormat
	}
	if err := run([]string{"accept-native", "--artifacts", artifacts}, io.Discard); !errors.Is(err, want) {
		t.Fatalf("trusted release must reach archive decoding under a different verifier revision: %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "")
	if err := run([]string{"accept-native", "--artifacts", artifacts, "--tag", "v0.1.0"}, io.Discard); !errors.Is(err, want) {
		t.Fatalf("explicit published tag lost product identity: %v", err)
	}
	if err := run([]string{"accept-native", "--artifacts", artifacts, "--tag", "v0.1.0", "--candidate"}, io.Discard); err == nil {
		t.Fatal("tagged artifacts were admitted as an untagged candidate")
	}
}

func TestNativePerformanceRequiresExplicitCandidateAndBaseline(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	for _, args := range [][]string{
		{"accept-native", "--performance", t.TempDir()},
		{"accept-native", "--artifacts", t.TempDir(), "--performance", t.TempDir()},
	} {
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "candidate artifact and published baseline") {
			t.Fatalf("performance input admission = %v", err)
		}
	}
}

func TestVerifyArtifactsRequiresSourceAndSignatureTrust(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for name, value := range map[string]string{
		"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE": "",
		"AIGW_RELEASE_ARTIFACT_SIGNER":               "",
		"AIGW_RELEASE_ALLOWED_SIGNERS_FILE":          "",
		"CI_COMMIT_TAG":                              "v1.2.3",
	} {
		t.Setenv(name, value)
	}
	if err := run([]string{"verify-artifacts", root}, io.Discard); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing source version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify-artifacts", root}, io.Discard); err == nil || !strings.Contains(err.Error(), "artifact authorization") {
		t.Fatalf("missing public trust: %v", err)
	}
}
