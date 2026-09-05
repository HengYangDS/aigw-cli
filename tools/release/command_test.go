package main

import (
	"aigw-cli/tools/release/readiness"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitLabReleaseRejectsDrift(t *testing.T) {
	expected := releaseDocument("v1.2.3", "https://example.test/assets")
	for _, mode := range []string{"missing-asset", "extra-asset", "duplicate-asset", "wrong-tag"} {
		t.Run(mode, func(t *testing.T) {
			actual, err := projectGitLabResponse(expected, mode)
			if err != nil {
				t.Fatal(err)
			}
			tmp := t.TempDir()
			expectedPath, actualPath := filepath.Join(tmp, "expected.json"), filepath.Join(tmp, "actual.json")
			writeFixtureJSON(t, expectedPath, expected)
			writeFixtureJSON(t, actualPath, actual)
			if err := verifyGitLabRelease(expectedPath, actualPath, filepath.Join(tmp, "assets.tsv"), "v1.2.3"); err == nil {
				t.Fatal("release drift accepted")
			}
		})
	}
}

func TestRunCoversPublicCommands(t *testing.T) {
	tmp := t.TempDir()
	var output bytes.Buffer
	if err := run([]string{"same-authority", "https://example.test", "https://example.test:443/a"}, &output); err != nil {
		t.Fatal(err)
	}
	releasePath := filepath.Join(tmp, "release.json")
	if err := run([]string{"write-gitlab-release", "v1.2.3", "https://example.test/assets", releasePath}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"validate-gitlab-release", releasePath, "v1.2.3"}, &output); err != nil {
		t.Fatal(err)
	}
	projected := filepath.Join(tmp, "projected.json")
	if err := run([]string{"project-gitlab-response", releasePath, "complete", projected}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"project-gitlab-response", releasePath, "unknown", filepath.Join(tmp, "rejected.json")}, &output); err == nil {
		t.Fatal("unknown projection mode was accepted")
	}
	headers := filepath.Join(tmp, "headers")
	if err := os.WriteFile(headers, []byte("Location: /next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"resolve-redirect", "https://example.test/base", headers}, &output); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(tmp, "actual.json")
	if err := run([]string{"project-gitlab-response", releasePath, "complete", actual}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify-gitlab-release", releasePath, actual, filepath.Join(tmp, "assets.tsv"), "v1.2.3"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"build"}, {"same-authority"}, {"resolve-redirect"}, {"write-gitlab-release"}, {"verify-gitlab-release"}, {"project-gitlab-response"}, {"validate-gitlab-release"}, {"validate-release-sources", "extra"}, {"unknown"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid command accepted: %v", args)
		}
	}
}

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

func TestRunPrintsDifferentAuthority(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"same-authority", "https://left.example", "https://right.example"}, &output); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "no" {
		t.Fatalf("output = %q", output.String())
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

func TestValidateBuildReleaseSources(t *testing.T) {
	for _, name := range []string{"AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY", "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY"} {
		t.Setenv(name, "")
	}
	if err := validateBuildReleaseSources(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/project")
	if err := validateBuildReleaseSources(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "https://github.example.test")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "owner/project")
	if err := validateBuildReleaseSources(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "")
	if err := validateBuildReleaseSources(); err == nil || !strings.Contains(err.Error(), "GitHub release source is incomplete") {
		t.Fatalf("partial source error = %v", err)
	}
}

func TestValidateBuildReleaseSourcesRejectsInvalidAuthoritiesAndRepositories(t *testing.T) {
	for _, name := range []string{"AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY", "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY"} {
		t.Setenv(name, "")
	}
	cases := []struct {
		name, origin, repository, want string
	}{
		{"http origin", "http://gitlab.example.test", "group/project", "HTTPS authority"},
		{"origin path", "https://gitlab.example.test/api", "group/project", "HTTPS authority"},
		{"repository edge slash", "https://gitlab.example.test", "/group/project", "namespace/project path"},
		{"repository query", "https://gitlab.example.test", "group/project?x", "namespace/project path"},
		{"repository empty segment", "https://gitlab.example.test", "group//project", "namespace/project path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", tc.origin)
			t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", tc.repository)
			if err := validateBuildReleaseSources(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "https://github.example.test")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "group/subgroup/project")
	if err := validateBuildReleaseSources(); err == nil || !strings.Contains(err.Error(), "owner/repository") {
		t.Fatalf("nested GitHub repository error = %v", err)
	}
}

func TestRunCoversPublishCommands(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	remote := readReleaseFixture(t, artifacts, "0.1.0")
	github := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/assets/") {
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/assets/")])
			return
		}
		writeGitHubFixture(t, response, request.Host, remote)
	}))
	defer github.Close()
	gitlab := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/packages/") {
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/packages/")])
			return
		}
		writeGitLabFixture(t, response, "http://"+request.Host, remote)
	}))
	defer gitlab.Close()

	t.Setenv("GITHUB_API_URL", github.URL)
	t.Setenv("GITHUB_REPOSITORY", "acme/aigw")
	t.Setenv("CI_COMMIT_TAG", "v0.1.0")
	t.Setenv("GH_TOKEN", "secret")
	var output bytes.Buffer
	if err := run([]string{"publish-github", artifacts}, &output); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_API_V4_URL", gitlab.URL)
	t.Setenv("CI_PROJECT_ID", "7")
	t.Setenv("CI_JOB_TOKEN", "secret")
	if err := run([]string{"publish-gitlab", artifacts}, &output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"publish-github"}, {"publish-gitlab"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
	if envDefault("MISSING_RELEASE_ENV", "fallback") != "fallback" {
		t.Fatal("environment fallback not applied")
	}
	if firstNonEmpty("", "value") != "value" || firstNonEmpty() != "" {
		t.Fatal("firstNonEmpty failed")
	}
}

func TestRunReportsCommandFailures(t *testing.T) {
	tmp := t.TempDir()
	var output bytes.Buffer
	missing := filepath.Join(tmp, "missing")
	invalidRelease := filepath.Join(tmp, "invalid.json")
	if err := os.WriteFile(invalidRelease, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"same-authority", "https://example.test", "invalid"},
		{"resolve-redirect", "https://example.test", missing},
		{"project-gitlab-response", invalidRelease, "complete", missing},
		{"project-gitlab-response", missing, "complete", missing},
		{"validate-gitlab-release", invalidRelease, "v1"},
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

func TestReleaseValidationErrorDetails(t *testing.T) {
	valid := releaseDocument("v1.2.3", "https://example.test/assets")
	wrongTag := valid
	wrongTag.TagName = "v2"
	if err := validateReleaseDocument(wrongTag, "v1.2.3"); err == nil {
		t.Fatal("wrong tag accepted")
	}
	invalidLink := valid
	invalidLink.Assets.Links = append([]releaseLink(nil), valid.Assets.Links...)
	invalidLink.Assets.Links[0].DirectAssetPath = "relative"
	if err := validateReleaseDocument(invalidLink, "v1.2.3"); err == nil {
		t.Fatal("invalid asset link accepted")
	}

	tmp := t.TempDir()
	validPath := filepath.Join(tmp, "valid.json")
	writeFixtureJSON(t, validPath, valid)
	badJSON := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(badJSON, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyGitLabRelease(badJSON, validPath, filepath.Join(tmp, "out"), "v1.2.3"); err == nil {
		t.Fatal("invalid expected JSON accepted")
	}
	if err := verifyGitLabRelease(validPath, badJSON, filepath.Join(tmp, "out"), "v1.2.3"); err == nil {
		t.Fatal("invalid actual JSON accepted")
	}

	actual, err := projectGitLabResponse(valid, "complete")
	if err != nil {
		t.Fatal(err)
	}
	actualPath := filepath.Join(tmp, "actual.json")
	writeFixtureJSON(t, actualPath, actual)
	if err := verifyGitLabRelease(validPath, actualPath, filepath.Join(tmp, "out"), "v9"); err == nil {
		t.Fatal("wrong expected tag accepted")
	}

	shortExpected := valid
	shortExpected.Assets.Links = shortExpected.Assets.Links[:len(shortExpected.Assets.Links)-1]
	shortExpectedPath := filepath.Join(tmp, "short-expected.json")
	writeFixtureJSON(t, shortExpectedPath, shortExpected)
	if err := verifyGitLabRelease(shortExpectedPath, actualPath, filepath.Join(tmp, "out"), "v1.2.3"); err == nil {
		t.Fatal("short local release accepted")
	}

	invalidExpected := valid
	invalidExpected.Assets.Links = append([]releaseLink(nil), valid.Assets.Links...)
	invalidExpected.Assets.Links[0].DirectAssetPath = "/bad/name"
	invalidExpectedPath := filepath.Join(tmp, "invalid-expected.json")
	writeFixtureJSON(t, invalidExpectedPath, invalidExpected)
	if err := verifyGitLabRelease(invalidExpectedPath, actualPath, filepath.Join(tmp, "out"), "v1.2.3"); err == nil {
		t.Fatal("invalid local asset name accepted")
	}

	duplicateExpected := valid
	duplicateExpected.Assets.Links = append([]releaseLink(nil), valid.Assets.Links...)
	duplicateExpected.Assets.Links[1].URL = duplicateExpected.Assets.Links[0].URL
	duplicateExpectedPath := filepath.Join(tmp, "duplicate-expected.json")
	writeFixtureJSON(t, duplicateExpectedPath, duplicateExpected)
	if err := verifyGitLabRelease(duplicateExpectedPath, actualPath, filepath.Join(tmp, "out"), "v1.2.3"); err == nil {
		t.Fatal("duplicate local URL accepted")
	}

	invalidActual := actual
	invalidActual.Assets.Links[0].URL = ""
	invalidActualPath := filepath.Join(tmp, "invalid-actual.json")
	writeFixtureJSON(t, invalidActualPath, invalidActual)
	if err := verifyGitLabRelease(validPath, invalidActualPath, filepath.Join(tmp, "out"), "v1.2.3"); err == nil {
		t.Fatal("invalid remote URL accepted")
	}
}

func TestFileAndURLFailurePaths(t *testing.T) {
	if _, err := readiness.ParseEpoch("-1"); err == nil {
		t.Fatal("negative epoch accepted")
	}
	if _, err := authority("https://user@example.test"); err == nil {
		t.Fatal("credentials accepted")
	}
	if _, err := resolveRedirect(":", "/next"); err == nil {
		t.Fatal("invalid base URL accepted")
	}
	if _, err := resolveRedirect("https://example.test", "%"); err == nil {
		t.Fatal("invalid redirect accepted")
	}
	for _, location := range []string{"ftp://example.test/a", "https://example.test/a#fragment"} {
		if _, err := resolveRedirect("https://example.test", location); err == nil {
			t.Fatalf("unsafe redirect accepted: %s", location)
		}
	}

	tmp := t.TempDir()
	headers := filepath.Join(tmp, "headers")
	if err := os.WriteFile(headers, []byte("Content-Type: text/plain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLocation(headers); err == nil {
		t.Fatal("missing Location accepted")
	}
	if _, err := readLocation(tmp); err == nil {
		t.Fatal("directory accepted as headers")
	}
	if err := writeJSON(filepath.Join(tmp, "missing", "value.json"), map[string]string{"a": "b"}); err == nil {
		t.Fatal("write into missing directory succeeded")
	}
	if err := writeJSON(filepath.Join(tmp, "unsupported.json"), make(chan int)); err == nil {
		t.Fatal("unsupported JSON value accepted")
	}
	var target map[string]any
	if err := readJSON(filepath.Join(tmp, "missing.json"), &target); err == nil {
		t.Fatal("missing JSON accepted")
	}
}

func writeFixtureJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
