package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/artifact"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRetainedCredentialCommandDoesNotReloadClientProjection(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		t.Run(client, func(t *testing.T) {
			journey := newNativeJourney(t, program, server.URL, true)
			var projection string
			if client == configuration.ClientCodex {
				projection, _ = journey.prepareCodexLifecycle()
			} else {
				projection = journey.settings
			}
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
			journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
			retained := journey.retainedCredential(client)
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "changed-after-capture")
			if err := os.Remove(projection); err != nil {
				t.Fatal(err)
			}
			journey.requireCredential(retained, "native-journey-token")
		})
	}
}

func TestNativeJourneyOwnsWorkingDirectory(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	journey := newNativeJourney(t, program, "https://unused.example.test", false)
	actual, err := os.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(journey.root)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(actual, expected) {
		t.Fatal("native journey inherited the repository working directory")
	}
	if output := journey.run("--help"); !bytes.Contains(output, []byte("Usage")) {
		t.Fatalf("native help is unavailable: %s", output)
	}
	journey.uninstallAndRequireOwnedFilesAbsent()
}

func TestRunBuildCIAndTagReadinessInputBoundaries(t *testing.T) {
	var output bytes.Buffer
	for _, args := range [][]string{{"build-ci"}, {"upload-gitlab"}, {"publish-github"}, {"publish-gitlab"}, {"verify-artifacts"}, {"validate-version-tag", "extra"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
	t.Setenv("CI_COMMIT_TAG", "")
	if err := run([]string{"validate-version-tag"}, &output); err == nil || !strings.Contains(err.Error(), "v<semver>") {
		t.Fatalf("missing tag error = %v", err)
	}
	for _, tc := range []struct {
		tag   string
		ready bool
	}{
		{"v1.2.3-rc.1+build.7", true},
		{"v1.2.3+build-rc.1", true},
		{"v0.1.0", true},
		{"v1.2.3-rc.01", false},
	} {
		t.Setenv("CI_COMMIT_TAG", tc.tag)
		if err := run([]string{"validate-version-tag"}, &output); (err == nil) != tc.ready {
			t.Errorf("tag=%q readiness=%v, ready=%t", tc.tag, err, tc.ready)
		}
	}

	workspace := filepath.Join(t.TempDir(), "workspace")
	outputPath := filepath.Join(t.TempDir(), "dist")
	t.Setenv("CI_COMMIT_TAG", "")
	t.Setenv("CI_COMMIT_SHORT_SHA", "")
	if err := run([]string{"build-ci", workspace, outputPath}, &output); err == nil || !strings.Contains(err.Error(), "CI build requires") {
		t.Fatalf("build-ci identity error = %v", err)
	}
}

func TestRunBuildUsesPublicArgumentContract(t *testing.T) {
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/aigw-cli")
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "https://github.example")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "org/aigw-cli")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("invalid-version\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var output bytes.Buffer
	err = run([]string{"build", filepath.Join(t.TempDir(), "dist")}, &output)
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("build error = %v", err)
	}

	for _, args := range [][]string{{"accept-native"}, {"accept-native", "--clients"}} {
		err = run(args, &output)
		if err == nil || !strings.Contains(err.Error(), "invalid release version") {
			t.Fatalf("native acceptance must validate the same source identity: %v", err)
		}
	}
}

func TestRunArtifactCommands(t *testing.T) {
	version := "1.2.3"
	key := artifactSigningKey(t)
	left, right := writeArtifactFixture(t, version, key), writeArtifactFixture(t, version, key)
	var output bytes.Buffer
	if err := run([]string{"validate-artifacts", left, version}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"compare-artifacts", left, right, version}, &output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"validate-artifacts"}, {"compare-artifacts"}, {"verify-macos-distribution"}, {"verify-macos-distribution", "artifacts", "1.2.3"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func TestRunReleasePolicyCommands(t *testing.T) {
	for _, name := range []string{"AIGW_CHANGELOG_RELEASE_TAG", "CI_COMMIT_TAG", "GITHUB_REF_NAME", "GITHUB_REF_TYPE"} {
		t.Setenv(name, "")
	}
	prepareSignedRelease(t, "1.2.3")
	module := "go.mod"
	if err := os.WriteFile(module, []byte("module example\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for _, args := range [][]string{
		{"validate-toolchain", module},
		{"validate-version", "1.2.3-rc.1"},
		{"validate-changelog"},
	} {
		if err := run(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"validate-toolchain"},
		{"validate-version"},
		{"validate-changelog", "extra"},
	} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func TestRunPublicationCommands(t *testing.T) {
	const version = "0.1.0-rc.1"
	artifacts := prepareSignedRelease(t, version)
	github := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected GitHub method %s", request.Method)
		}
		if strings.HasPrefix(request.URL.Path, "/assets/") {
			http.ServeFile(response, request, filepath.Join(artifacts, filepath.Base(request.URL.Path)))
			return
		}
		assets := make([]map[string]string, 0, len(artifact.Names(version)))
		for _, name := range artifact.Names(version) {
			assets = append(assets, map[string]string{"name": name, "url": "http://" + request.Host + "/assets/" + name})
		}
		if err := json.NewEncoder(response).Encode(map[string]any{"id": 1, "assets": assets}); err != nil {
			t.Fatal(err)
		}
	}))
	defer github.Close()
	gitlab := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPut:
			response.WriteHeader(http.StatusCreated)
			return
		case strings.HasPrefix(request.URL.Path, "/packages/"):
			http.ServeFile(response, request, filepath.Join(artifacts, filepath.Base(request.URL.Path)))
			return
		}
		links := make([]map[string]string, 0, len(artifact.Names(version)))
		for _, name := range artifact.Names(version) {
			links = append(links, map[string]string{"url": "http://" + request.Host + "/packages/" + name})
		}
		if err := json.NewEncoder(response).Encode(map[string]any{"tag_name": "v" + version, "assets": map[string]any{"links": links}}); err != nil {
			t.Fatal(err)
		}
	}))
	defer gitlab.Close()

	for name, value := range map[string]string{
		"GITHUB_API_URL": github.URL, "GITHUB_REPOSITORY": "acme/aigw",
		"CI_COMMIT_TAG": "v" + version, "GH_TOKEN": "secret",
		"CI_API_V4_URL": gitlab.URL, "CI_PROJECT_ID": "7", "CI_JOB_TOKEN": "", "GITLAB_TOKEN": "local-release-token",
	} {
		t.Setenv(name, value)
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for _, scenario := range []struct {
		name   string
		writer io.Writer
		cause  error
	}{
		{"publish-github", &output, nil},
		{"upload-gitlab", &output, nil},
		{"publish-gitlab", &output, nil},
		{"publish-github", closed, os.ErrClosed},
		{"publish-gitlab", closed, os.ErrClosed},
	} {
		err := run([]string{scenario.name, artifacts}, scenario.writer)
		if !errors.Is(err, scenario.cause) {
			t.Errorf("%s: error=%v, want cause=%v", scenario.name, err, scenario.cause)
		}
	}
	if output.String() != "GitHub release verified (created=false)\nGitLab release verified (created=false)\n" {
		t.Fatalf("publication report = %q", output.String())
	}
}

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
}

func TestNativePerformanceRequiresExplicitPublishedInputs(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	for _, args := range [][]string{
		{"accept-native", "--performance", t.TempDir()},
		{"accept-native", "--artifacts", t.TempDir(), "--performance", t.TempDir()},
	} {
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "published candidate and baseline") {
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

func TestRunReportsCommandFailures(t *testing.T) {
	var output bytes.Buffer
	cases := [][]string{
		nil,
		{"build"},
		{"validate-release-sources", "extra"},
	}
	for _, args := range cases {
		if err := run(args, &output); err == nil {
			t.Errorf("invalid command accepted: %v", args)
		}
	}
}

func prepareSignedRelease(t *testing.T, version string) string {
	t.Helper()
	key := artifactSigningKey(t)
	artifacts := writeArtifactFixture(t, version, key)
	publicKey := readFile(t, key+".pub")
	allowedSigners := filepath.Join(t.TempDir(), "allowed-signers")
	if err := os.WriteFile(allowedSigners, []byte(`release@test.invalid namespaces="git,aigw-release" `+string(publicKey)), 0o600); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for name, content := range map[string]string{
		"VERSION": version + "\n", "go.mod": "module example.invalid/aigw\n", "go.sum": "sum\n",
		"CHANGELOG.md":      "# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [" + version + "] - 2026-01-01\n\n### Fixed\n\n- Fix.\n",
		"package-lock.json": "{}\n", "mise.lock": "lockfile_version = 1\n", "mise.toml": "[tools]\ngo = \"1.27.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", source, "-c", "core.hooksPath=" + filepath.Join(source, ".git", "hooks"),
			"-c", "user.name=Release Test", "-c", "user.email=release@test.invalid", "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "user.signingkey=" + key}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-q", "-S", "-m", "test: release source")
	git("tag", "-s", "-m", "Release "+version, "v"+version)
	if err := artifact.WriteProvenance(source, artifacts, filepath.Join(artifacts, "aigw_"+version+".provenance.json"), version, git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")); err != nil {
		t.Fatal(err)
	}
	if err := artifact.RewriteChecksums(artifacts, version); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(artifacts, "checksums.txt")
	if err := os.Remove(manifest + ".sig"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("ssh-keygen", "-Y", "sign", "-n", artifact.SignatureNamespace, "-f", key, manifest).CombinedOutput(); err != nil {
		t.Fatalf("sign manifest: %v: %s", err, output)
	}
	t.Chdir(source)
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE": allowedSigners,
		"AIGW_RELEASE_ARTIFACT_SIGNER":               "release@test.invalid",
		"AIGW_RELEASE_ALLOWED_SIGNERS_FILE":          allowedSigners,
		"CI_COMMIT_TAG":                              "v" + version,
	} {
		t.Setenv(name, value)
	}
	return artifacts
}

func writeArtifactFixture(t *testing.T, version, key string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range artifact.Names(version) {
		if name == "checksums.txt" || name == "checksums.txt.sig" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := artifact.RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, "checksums.txt")
	if output, err := exec.Command("ssh-keygen", "-Y", "sign", "-n", artifact.SignatureNamespace, "-f", key, manifest).CombinedOutput(); err != nil {
		t.Fatalf("sign checksum manifest: %v: %s", err, output)
	}
	return directory
}

func artifactSigningKey(t *testing.T) string {
	t.Helper()
	key := filepath.Join(t.TempDir(), "release-signing-key")
	if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("generate signing key: %v: %s", err, output)
	}
	return key
}
