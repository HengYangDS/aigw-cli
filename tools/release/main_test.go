package main

import (
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

func TestExecuteReturnsPortableProcessStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if status := execute([]string{"unknown"}, &stdout, &stderr); status != 2 || !strings.Contains(stderr.String(), "unknown release command") {
		t.Fatalf("failure status=%d stderr=%q", status, stderr.String())
	}
	stderr.Reset()
	if status := execute([]string{"same-authority", "https://example.test", "https://example.test"}, &stdout, &stderr); status != 0 {
		t.Fatalf("success status=%d stderr=%q", status, stderr.String())
	}
}

func TestArtifactMatrixRejectsMissingExtraAndCorruptFiles(t *testing.T) {
	version := "1.2.3"
	if err := validateArtifactMatrix(filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing matrix accepted")
	}
	directory := writeArtifactFixture(t, version)
	if err := validateArtifactMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, artifactNames(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("extra artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, artifactNames(version)[0]), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	checksumPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
		t.Fatalf("invalid checksum manifest=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	content, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(string(content), "\n", 2)[0]
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(first+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("duplicate checksum=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(strings.Repeat("0", 64)+"  unknown\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected entries") {
		t.Fatalf("unexpected checksum=%v", err)
	}
}

func TestCompareArtifactMatrices(t *testing.T) {
	version := "1.2.3"
	left, right := writeArtifactFixture(t, version), writeArtifactFixture(t, version)
	if err := compareArtifactMatrices(left, right, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, artifactNames(version)[0]), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rewriteChecksums(right, version); err != nil {
		t.Fatal(err)
	}
	if err := compareArtifactMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different matrix=%v", err)
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

func TestReleasePolicyCommands(t *testing.T) {
	tmp := t.TempDir()
	module := filepath.Join(tmp, "go.mod")
	if err := os.WriteFile(module, []byte("module example\n\ngo 1.26.5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateToolchain(module, "go1.26.5"); err != nil {
		t.Fatal(err)
	}
	if err := validateToolchain(module, "go0.0.0"); err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("wrong toolchain=%v", err)
	}
	if err := os.WriteFile(module, []byte("module example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateToolchain(module, "go1.26.5"); err == nil || !strings.Contains(err.Error(), "no Go version") {
		t.Fatalf("missing version=%v", err)
	}
	if err := validateToolchain(filepath.Join(tmp, "missing.mod"), "go1.26.5"); err == nil {
		t.Fatal("missing go.mod accepted")
	}

	if err := validateReleaseReadiness("1.2.3-rc.1"); err != nil {
		t.Fatal(err)
	}
	if err := validateReleaseReadiness("1.2.3"); err == nil {
		t.Fatal("unsigned GA accepted")
	}
	document := filepath.Join(tmp, "readiness.md")
	if err := os.WriteFile(document, []byte("# Release readiness\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateReleaseReadinessDocument(document); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(document, []byte("Current status (2026-07-14)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateReleaseReadinessDocument(document); err == nil {
		t.Fatal("stale readiness document accepted")
	}
	if err := validateReleaseReadinessDocument(filepath.Join(tmp, "missing-readiness")); err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("missing readiness document=%v", err)
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

func writeArtifactFixture(t *testing.T, version string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range artifactNames(version) {
		if name == "checksums.txt" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := rewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestSameAuthorityNormalizesDefaultPorts(t *testing.T) {
	got, err := sameAuthority("https://gitlab.example/api/v4", "https://gitlab.example:443/assets/a")
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("default HTTPS port should identify the same authority")
	}
}

func TestAuthorityNormalizesHTTPAndRejectsInvalidPort(t *testing.T) {
	got, err := authority("http://EXAMPLE.test./asset")
	if err != nil || got != "http://example.test:80" {
		t.Fatalf("authority=%q err=%v", got, err)
	}
	if _, err := authority("https://example.test:bad"); err == nil {
		t.Fatal("invalid port accepted")
	}
}

func TestResolveRedirectRejectsDowngradeAndCredentials(t *testing.T) {
	for _, location := range []string{"http://other.example/a", "https://user@other.example/a"} {
		if _, err := resolveRedirect("https://gitlab.example/a", location); err == nil {
			t.Fatalf("unsafe redirect accepted: %s", location)
		}
	}
}

func TestVerifyGitLabReleaseWritesCanonicalAssetList(t *testing.T) {
	expected := releaseDocument("v1.2.3", "https://gitlab.example/api/v4/projects/7/packages/generic/aigw/1.2.3")
	actual := map[string]any{"tag_name": "v1.2.3", "assets": map[string]any{"links": []map[string]string{}}}
	for _, link := range expected.Assets.Links {
		actual["assets"].(map[string]any)["links"] = append(actual["assets"].(map[string]any)["links"].([]map[string]string), map[string]string{"url": link.URL})
	}
	tmp := t.TempDir()
	expectedPath := filepath.Join(tmp, "expected.json")
	actualPath := filepath.Join(tmp, "actual.json")
	listPath := filepath.Join(tmp, "assets.tsv")
	writeFixtureJSON(t, expectedPath, expected)
	writeFixtureJSON(t, actualPath, actual)
	if err := verifyGitLabRelease(expectedPath, actualPath, listPath, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(artifactNames("1.2.3")) || !strings.HasPrefix(lines[0], "aigw_1.2.3_darwin_amd64.tar.gz\t") {
		t.Fatalf("asset list = %q", content)
	}
}

func TestVerifyGitLabReleaseRejectsEqualSizeAssetSubstitution(t *testing.T) {
	expected := releaseDocument("v1.2.3", "https://gitlab.example/assets")
	actual := remoteRelease{TagName: expected.TagName}
	for _, link := range expected.Assets.Links {
		actual.Assets.Links = append(actual.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: link.URL})
	}
	actual.Assets.Links[0].URL = "https://gitlab.example/assets/substitute.tar.gz"
	tmp := t.TempDir()
	expectedPath := filepath.Join(tmp, "expected.json")
	actualPath := filepath.Join(tmp, "actual.json")
	writeFixtureJSON(t, expectedPath, expected)
	writeFixtureJSON(t, actualPath, actual)
	if err := verifyGitLabRelease(expectedPath, actualPath, filepath.Join(tmp, "assets.tsv"), "v1.2.3"); err == nil || !strings.Contains(err.Error(), "missing asset") {
		t.Fatalf("substituted asset error = %v", err)
	}
}

func TestProjectGitLabResponseModes(t *testing.T) {
	expected := releaseDocument("v1.2.3", "https://gitlab.example/assets")
	complete, err := projectGitLabResponse(expected, "complete")
	if err != nil {
		t.Fatal(err)
	}
	if complete.TagName != expected.TagName || len(complete.Assets.Links) != len(artifactNames("1.2.3")) {
		t.Fatalf("complete = %+v", complete)
	}
	missing, err := projectGitLabResponse(expected, "missing-asset")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Assets.Links) != len(artifactNames("1.2.3"))-1 {
		t.Fatalf("missing links = %d", len(missing.Assets.Links))
	}
	if _, err := projectGitLabResponse(expected, "unknown"); err == nil {
		t.Fatal("unknown fixture mode accepted")
	}
}

func TestValidateReleaseDocument(t *testing.T) {
	payload := releaseDocument("v1.2.3", "https://gitlab.example/assets")
	if err := validateReleaseDocument(payload, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	payload.Assets.Links = payload.Assets.Links[:len(payload.Assets.Links)-1]
	if err := validateReleaseDocument(payload, "v1.2.3"); err == nil {
		t.Fatal("incomplete release document accepted")
	}
}

func TestReleaseFileAndRedirectEdges(t *testing.T) {
	tmp := t.TempDir()
	headers := filepath.Join(tmp, "headers")
	if err := os.WriteFile(headers, []byte("HTTP/1.1 302 Found\r\nLocation: /next\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	location, err := readLocation(headers)
	if err != nil || location != "/next" {
		t.Fatalf("location=%q err=%v", location, err)
	}
	resolved, err := resolveRedirect("https://example.test/base", location)
	if err != nil || resolved != "https://example.test/next" {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	if _, err := authority("ftp://example.test"); err == nil {
		t.Fatal("non-HTTP authority accepted")
	}
	if _, err := sameAuthority("bad", "https://example.test"); err == nil {
		t.Fatal("invalid authority accepted")
	}
	if got := lastPath("https://example.test/a/b.bin"); got != "b.bin" {
		t.Fatalf("last path=%q", got)
	}
	if got := lastPath(":"); got != "" {
		t.Fatalf("invalid last path=%q", got)
	}
	path := filepath.Join(tmp, "multi.json")
	if err := os.WriteFile(path, []byte("{}{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var target map[string]any
	if err := readJSON(path, &target); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}

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
	var output bytes.Buffer
	err := run([]string{"build", "invalid-version", "1784246400", filepath.Join(t.TempDir(), "dist")}, &output)
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
	if _, err := parseEpoch("-1"); err == nil {
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
