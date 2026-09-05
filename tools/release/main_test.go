package main

import (
	"aigw-cli/tools/release/artifact"
	"bytes"
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
	if err := artifact.ValidateMatrix(filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing matrix accepted")
	}
	directory := writeArtifactFixture(t, version)
	if err := artifact.ValidateMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, artifact.Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("extra artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, artifact.Names(version)[0]), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	checksumPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
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
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("duplicate checksum=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(strings.Repeat("0", 64)+"  unknown\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected entries") {
		t.Fatalf("unexpected checksum=%v", err)
	}
}

func TestCompareArtifactMatrices(t *testing.T) {
	version := "1.2.3"
	left, right := writeArtifactFixture(t, version), writeArtifactFixture(t, version)
	if err := artifact.CompareMatrices(left, right, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, artifact.Names(version)[0]), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.RewriteChecksums(right, version); err != nil {
		t.Fatal(err)
	}
	if err := artifact.CompareMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs") {
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
	if len(lines) != len(artifact.Names("1.2.3")) || !strings.HasPrefix(lines[0], "aigw_1.2.3_darwin_amd64.tar.gz\t") {
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
	if complete.TagName != expected.TagName || len(complete.Assets.Links) != len(artifact.Names("1.2.3")) {
		t.Fatalf("complete = %+v", complete)
	}
	missing, err := projectGitLabResponse(expected, "missing-asset")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Assets.Links) != len(artifact.Names("1.2.3"))-1 {
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
