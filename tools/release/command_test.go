package main

import (
	"aigw-cli/tools/release/artifact"
	"bytes"
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

func TestRunBuildCIAndTagReadinessInputBoundaries(t *testing.T) {
	var output bytes.Buffer
	for _, args := range [][]string{{"build-ci"}, {"upload-gitlab"}, {"publish-github"}, {"publish-gitlab"}, {"validate-readiness-tag", "extra"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
	t.Setenv("CI_COMMIT_TAG", "")
	if err := run([]string{"validate-readiness-tag"}, &output); err == nil || !strings.Contains(err.Error(), "v<semver>") {
		t.Fatalf("missing tag error = %v", err)
	}
	for _, tc := range []struct {
		tag   string
		ready bool
	}{
		{"v1.2.3-rc.1+build.7", true},
		{"v1.2.3+build-rc.1", false},
		{"v1.2.3-rc.01", false},
	} {
		t.Setenv("CI_COMMIT_TAG", tc.tag)
		if err := run([]string{"validate-readiness-tag"}, &output); (err == nil) != tc.ready {
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
	for _, args := range [][]string{{"validate-artifacts"}, {"compare-artifacts"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func TestRunReleasePolicyCommands(t *testing.T) {
	tmp := t.TempDir()
	module := filepath.Join(tmp, "go.mod")
	if err := os.WriteFile(module, []byte("module example\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for _, args := range [][]string{
		{"validate-toolchain", module},
		{"validate-readiness", "1.2.3-rc.1"},
	} {
		if err := run(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"validate-toolchain"},
		{"validate-readiness"},
	} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func TestRunPublicationCommands(t *testing.T) {
	const version = "0.1.0"
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
		if request.Method == http.MethodPut {
			response.WriteHeader(http.StatusCreated)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/packages/") {
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
		"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE": allowedSigners,
		"AIGW_RELEASE_ARTIFACT_SIGNER":               "release@test.invalid",
		"AIGW_RELEASE_ALLOWED_SIGNERS_FILE":          allowedSigners,
		"GITHUB_API_URL":                             github.URL, "GITHUB_REPOSITORY": "acme/aigw",
		"CI_COMMIT_TAG": "v" + version, "GH_TOKEN": "secret",
		"CI_API_V4_URL": gitlab.URL, "CI_PROJECT_ID": "7", "CI_JOB_TOKEN": "secret",
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

func TestReleaseEnvironmentSelection(t *testing.T) {
	if envDefault("MISSING_RELEASE_ENV", "fallback") != "fallback" || firstNonEmpty("", "value") != "value" || firstNonEmpty() != "" {
		t.Fatal("environment selection failed")
	}
}

func TestRunReportsCommandFailures(t *testing.T) {
	var output bytes.Buffer
	cases := [][]string{
		nil,
		{"build"},
		{"validate-release-sources", "extra"},
		{"validate-toolchain"},
		{"validate-readiness"},
		{"validate-artifacts"},
		{"compare-artifacts"},
	}
	for _, args := range cases {
		if err := run(args, &output); err == nil {
			t.Errorf("invalid command accepted: %v", args)
		}
	}
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
