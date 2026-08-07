package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTouchTimestampUsesUTC(t *testing.T) {
	got, err := touchTimestamp("1784246400")
	if err != nil {
		t.Fatal(err)
	}
	if got != "202607170000.00" {
		t.Fatalf("timestamp = %q", got)
	}
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
	if len(lines) != 15 || !strings.HasPrefix(lines[0], "aigw_1.2.3_darwin_universal.pkg\t") {
		t.Fatalf("asset list = %q", content)
	}
}

func TestProjectGitLabResponseModes(t *testing.T) {
	expected := releaseDocument("v1.2.3", "https://gitlab.example/assets")
	complete, err := projectGitLabResponse(expected, "complete")
	if err != nil {
		t.Fatal(err)
	}
	if complete.TagName != expected.TagName || len(complete.Assets.Links) != 15 {
		t.Fatalf("complete = %+v", complete)
	}
	missing, err := projectGitLabResponse(expected, "missing-asset")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Assets.Links) != 14 {
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
	payload.Assets.Links = payload.Assets.Links[:14]
	if err := validateReleaseDocument(payload, "v1.2.3"); err == nil {
		t.Fatal("incomplete release document accepted")
	}
}

func TestWriteMSIMetadataRequiresDeterministicFields(t *testing.T) {
	tmp := t.TempDir()
	environment := filepath.Join(tmp, "Environment.idt")
	summary := filepath.Join(tmp, "Summary.idt")
	content := "Property\tValue\r\ni2\tl255\r\n_SummaryInformation\tProperty\r\n9\told\r\n12\told\r\n13\told\r\n"
	if err := os.WriteFile(summary, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeMSIMetadata(environment, summary, "ENV-GUID", "PACKAGE-GUID", time.Unix(1784246400, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(summary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "9\tPACKAGE-GUID\r\n") || !strings.Contains(string(got), "12\t2026/07/17 00:00:00\r\n") {
		t.Fatalf("summary = %q", got)
	}
}

func TestCandidateManifestAndCommandDispatch(t *testing.T) {
	manifest := candidateManifest{Schema: 1, Kind: "aigw-verified-candidate", Version: "1.2.3", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), CreatedUTC: "2026-08-07T00:00:00Z", ArtifactsDir: "artifacts", ChecksumsPath: "artifacts/checksums.txt", ChecksumsSHA256: strings.Repeat("c", 64), ArtifactCount: 15}
	if err := validateCandidateManifest(manifest); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*candidateManifest){
		"fixed":    func(value *candidateManifest) { value.Schema = 2 },
		"version":  func(value *candidateManifest) { value.Version = "bad value" },
		"commit":   func(value *candidateManifest) { value.Commit = "bad" },
		"checksum": func(value *candidateManifest) { value.ChecksumsSHA256 = "bad" },
	} {
		t.Run(name, func(t *testing.T) {
			value := manifest
			mutate(&value)
			if err := validateCandidateManifest(value); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
	tmp := t.TempDir()
	candidate := filepath.Join(tmp, "candidate.json")
	if err := writeJSON(candidate, manifest); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"validate-candidate-manifest", candidate}, &output); err != nil || !strings.Contains(output.String(), manifest.Version) {
		t.Fatalf("validate output=%q err=%v", output.String(), err)
	}
	if err := run([]string{"unknown"}, &output); err == nil {
		t.Fatal("unknown command accepted")
	}
}

func TestReleaseKitFileAndRedirectEdges(t *testing.T) {
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
	if err := run([]string{"touch-timestamp", "0"}, &output); err != nil {
		t.Fatal(err)
	}
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
	manifest := filepath.Join(tmp, "candidate.json")
	if err := run([]string{"write-candidate-manifest", manifest, "1.2.3", strings.Repeat("a", 40), strings.Repeat("b", 40), "2026-08-07T00:00:00Z", strings.Repeat("c", 64), "15"}, &output); err != nil {
		t.Fatal(err)
	}
	summary := filepath.Join(tmp, "Summary.idt")
	if err := os.WriteFile(summary, []byte("Property\tValue\r\ni2\tl255\r\n_SummaryInformation\tProperty\r\n9\told\r\n12\told\r\n13\told\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"msi-metadata", "-environment", filepath.Join(tmp, "Environment.idt"), "-summary", summary, "-environment-guid", "ENV", "-package-guid", "PACKAGE", "-epoch", "0"}, &output); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(tmp, "actual.json")
	if err := run([]string{"project-gitlab-response", releasePath, "complete", actual}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify-gitlab-release", releasePath, actual, filepath.Join(tmp, "assets.tsv"), "v1.2.3"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"touch-timestamp"}, {"same-authority"}, {"resolve-redirect"}, {"write-gitlab-release"}, {"verify-gitlab-release"}, {"project-gitlab-response"}, {"validate-gitlab-release"}, {"write-candidate-manifest"}, {"validate-candidate-manifest"}, {"validate-release-sources", "extra"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid command accepted: %v", args)
		}
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
		{"touch-timestamp", "invalid"},
		{"msi-metadata", "-environment", "x"},
		{"msi-metadata", "-environment", "x", "-summary", "y", "-environment-guid", "e", "-package-guid", "p", "-epoch", "invalid"},
		{"same-authority", "https://example.test", "invalid"},
		{"resolve-redirect", "https://example.test", missing},
		{"project-gitlab-response", invalidRelease, "complete", missing},
		{"project-gitlab-response", missing, "complete", missing},
		{"validate-gitlab-release", invalidRelease, "v1"},
		{"validate-candidate-manifest", invalidRelease},
		{"write-candidate-manifest", missing, "v", "c", "t", "time", "sum", "invalid"},
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
	shortExpected.Assets.Links = shortExpected.Assets.Links[:14]
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
	if _, err := touchTimestamp("invalid"); err == nil {
		t.Fatal("invalid epoch accepted")
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

func TestMSIMetadataFailurePaths(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "missing")
	if err := writeMSIMetadata(filepath.Join(missing, "Environment.idt"), "unused", "E", "P", time.Unix(0, 0)); err == nil {
		t.Fatal("missing environment directory accepted")
	}
	if err := writeMSIMetadata(filepath.Join(tmp, "Environment.idt"), filepath.Join(tmp, "missing.idt"), "E", "P", time.Unix(0, 0)); err == nil {
		t.Fatal("missing summary accepted")
	}
	summary := filepath.Join(tmp, "Summary.idt")
	if err := os.WriteFile(summary, []byte("Property\tValue\nmissing\tfields\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeMSIMetadata(filepath.Join(tmp, "Environment.idt"), summary, "E", "P", time.Unix(0, 0)); err == nil {
		t.Fatal("incomplete summary accepted")
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
