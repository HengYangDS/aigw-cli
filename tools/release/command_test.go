package main

import (
	"aigw-cli/tools/release/artifact"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunBuildCIAndTagReadinessInputBoundaries(t *testing.T) {
	var output bytes.Buffer
	for _, args := range [][]string{{"build-ci"}, {"upload-gitlab"}, {"validate-readiness-tag", "extra"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
	t.Setenv("CI_COMMIT_TAG", "")
	if err := run([]string{"validate-readiness-tag"}, &output); err == nil || !strings.Contains(err.Error(), "v<semver>") {
		t.Fatalf("missing tag error = %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.3-rc.1")
	if err := run([]string{"validate-readiness-tag"}, &output); err != nil {
		t.Fatal(err)
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
}

func TestRunArtifactCommands(t *testing.T) {
	version := "1.2.3"
	left, right := writeArtifactFixture(t, version), writeArtifactFixture(t, version)
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
	document := filepath.Join(tmp, "readiness.md")
	if err := os.WriteFile(document, []byte("# readiness\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for _, args := range [][]string{
		{"validate-toolchain", module},
		{"validate-readiness", "1.2.3-rc.1"},
		{"validate-readiness-doc", document},
	} {
		if err := run(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"validate-toolchain"},
		{"validate-readiness"},
		{"validate-readiness-doc"},
	} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func TestRunPublicationCommands(t *testing.T) {
	const version = "0.1.0"
	artifacts := writeArtifactFixture(t, version)
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

	t.Setenv("GITHUB_API_URL", github.URL)
	t.Setenv("GITHUB_REPOSITORY", "acme/aigw")
	t.Setenv("CI_COMMIT_TAG", "v"+version)
	t.Setenv("GH_TOKEN", "secret")
	t.Setenv("CI_API_V4_URL", gitlab.URL)
	t.Setenv("CI_PROJECT_ID", "7")
	t.Setenv("CI_JOB_TOKEN", "secret")
	var output bytes.Buffer
	for _, args := range [][]string{{"publish-github", artifacts}, {"upload-gitlab", artifacts}, {"publish-gitlab", artifacts}} {
		if err := run(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if !strings.Contains(output.String(), "GitHub release verified (created=false)") || !strings.Contains(output.String(), "GitLab release verified (created=false)") {
		t.Fatalf("output = %q", output.String())
	}
	for _, args := range [][]string{{"publish-github"}, {"upload-gitlab"}, {"publish-gitlab"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
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
		{"validate-readiness-doc"},
		{"validate-artifacts"},
		{"compare-artifacts"},
	}
	for _, args := range cases {
		if err := run(args, &output); err == nil {
			t.Errorf("invalid command accepted: %v", args)
		}
	}
}

func writeArtifactFixture(t *testing.T, version string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range artifact.Names(version) {
		if name == "checksums.txt" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := artifact.RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	return directory
}
