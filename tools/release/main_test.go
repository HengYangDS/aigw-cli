package main

import (
	"aigw-cli/tools/release/construction"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNativeInputSelectionRequiresOneTransportAndExactIdentities(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	producer := strings.Repeat("a", 40)
	archive := filepath.Join(t.TempDir(), "public-inputs.tar")
	selection := []string{"--candidate", "--candidate-source", producer, "--input-sha256", strings.Repeat("b", 64), "--baseline-tag", "v1.2.3"}
	for _, transport := range [][]string{
		{"--input-archive", archive},
		{"--input-package", "native-inputs", "--peer", "gitlab", "--repository", "group/product"},
		{"--input-release", "native-inputs-" + producer, "--peer", "github", "--repository", "group/product"},
	} {
		input, err := construction.ParseNativeAcceptance(append(slices.Clone(selection), transport...))
		if err != nil || input.CandidateSource != producer || input.BaselineTag != "v1.2.3" || !input.Candidate {
			t.Fatalf("valid native selection lost producer or predecessor: %#v, %v", input, err)
		}
	}
	for _, arguments := range [][]string{
		{"--unknown"}, {"positional"}, {"--input-sha256", strings.Repeat("b", 64)},
		{"--input-package", "native-inputs", "--peer", "github", "--repository", "group/product"},
		{"--input-release", "native-inputs-" + producer, "--peer", "gitlab", "--repository", "group/product"},
		append(slices.Clone(selection), "--input-archive", "relative.tar"),
		{"--input-archive", archive},
		{"--candidate", "--candidate-source", producer, "--input-archive", archive, "--input-sha256", "invalid", "--baseline-tag", "v1.2.3"},
		append(slices.Clone(selection), "--input-release", "unnamed-carrier", "--peer", "github", "--repository", "group/product"),
		append(slices.Clone(selection), "--input-release", "native-inputs-"+strings.Repeat("A", 40), "--peer", "github", "--repository", "group/product"),
		{"--peer", "github"}, {"--candidate-source", producer}, {"--baseline-artifacts", archive},
		{"--tag", "v1.2.3"}, {"--artifacts", archive, "--tag", "1.2.3"},
		{"--baseline-tag", "v1.2.3"}, {"--performance", t.TempDir()},
	} {
		if input, err := construction.ParseNativeAcceptance(arguments); err == nil {
			t.Fatalf("ambiguous or unbound native selection accepted: %q, %#v", arguments, input)
		}
	}
}

func TestNativeInputWorkspaceFailureAndInvalidSelectionHaveNoEffects(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile("VERSION", []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changelog := "# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [1.2.3] - 2026-07-17\n\n### Fixed\n\n- Preserve native inputs.\n"
	if err := os.WriteFile("CHANGELOG.md", []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(root, "caller-file")
	if err := os.WriteFile(blocker, []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, blocker)
	}
	for _, input := range []construction.NativeAcceptance{
		{CandidateSource: strings.Repeat("a", 40)},
		{BaselineTag: "v1.2.2", BaselineArtifacts: root},
		{},
	} {
		if err := construction.AcceptNative(t.Context(), input); err == nil {
			t.Fatal("invalid selection or unavailable native workspace was accepted")
		}
	}
	if body, err := os.ReadFile(blocker); err != nil || string(body) != "preserved" {
		t.Fatalf("native workspace failure changed caller bytes: %q, %v", body, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 3 || !slices.Equal([]string{entries[0].Name(), entries[1].Name(), entries[2].Name()}, []string{"CHANGELOG.md", "VERSION", "caller-file"}) {
		t.Fatalf("native workspace failure produced residue: %v, %v", entries, err)
	}
}

func requireNativeLifecycleBaseline(t *testing.T, buildFixture func() string) string {
	t.Helper()
	baseline, err := nativeLifecycleBaseline(buildFixture)
	if err != nil {
		t.Fatal(err)
	}
	return baseline
}

func mustWriteFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteReturnsPortableProcessStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if status := execute([]string{"unknown"}, &stdout, &stderr); status != 2 || !strings.Contains(stderr.String(), "unknown release command") {
		t.Fatalf("failure status=%d stderr=%q", status, stderr.String())
	}
	stderr.Reset()
	if status := execute([]string{"validate-version", "1.2.3-rc.1"}, &stdout, &stderr); status != 0 {
		t.Fatalf("success status=%d stderr=%q", status, stderr.String())
	}
}

func TestReleaseEnvironmentSelection(t *testing.T) {
	if envDefault("MISSING_RELEASE_ENV", "fallback") != "fallback" || firstNonEmpty("", "value") != "value" || firstNonEmpty() != "" {
		t.Fatal("environment selection failed")
	}
}

func buildNativeProgram(t *testing.T, root, version string) string {
	t.Helper()
	for _, name := range []string{"CI_COMMIT_TAG", "GITHUB_REF_TYPE"} {
		value, present := os.LookupEnv(name)
		t.Setenv(name, "")
		defer func() {
			var err error
			if present {
				err = os.Setenv(name, value)
			} else {
				err = os.Unsetenv(name)
			}
			if err != nil {
				t.Errorf("restore release environment %s: %v", name, err)
			}
		}()
	}
	stage, err := construction.BuildNative(t.Context(), root, t.TempDir(), version)
	if err != nil {
		t.Fatalf("build native product %s: %v", version, err)
	}
	base, _ := nativeArchiveNames(version)
	return filepath.Join(stage, base, executableName())
}

func TestNativeFixturePreservesReleaseEnvironment(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_COMMIT_TAG", "v0.1.0-rc.116")
	t.Setenv("GITHUB_REF_TYPE", "tag")
	program := buildNativeProgram(t, root, "0.0.0")
	if _, err := os.Stat(program); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CI_COMMIT_TAG") != "v0.1.0-rc.116" || os.Getenv("GITHUB_REF_TYPE") != "tag" {
		t.Fatal("fixture construction changed the surrounding release context")
	}
}

func TestStablePublicationCommandsRequireExplicitMacOSIdentity(t *testing.T) {
	t.Setenv("AIGW_MACOS_SIGNING_IDENTITY", "")
	artifacts := prepareSignedRelease(t, "0.1.0")
	for name, value := range map[string]string{
		"GITHUB_REPOSITORY": "acme/aigw", "CI_COMMIT_TAG": "v0.1.0", "GH_TOKEN": "synthetic",
		"CI_API_V4_URL": "https://example.test", "CI_PROJECT_ID": "7", "CI_JOB_TOKEN": "", "GITLAB_TOKEN": "synthetic",
	} {
		t.Setenv(name, value)
	}
	for _, command := range []string{"publish-github", "publish-gitlab", "upload-gitlab"} {
		var output bytes.Buffer
		if err := run([]string{command, artifacts}, &output); err == nil || !strings.Contains(err.Error(), "explicit signing identity") {
			t.Fatalf("%s bypassed native distribution admission: %v", command, err)
		}
	}
}

func TestNativePerformanceRequiresExplicitPublishedInputs(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	for _, args := range [][]string{
		{"accept-native", "--performance", t.TempDir()},
		{"accept-native", "--artifacts", t.TempDir(), "--performance", t.TempDir()},
	} {
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "explicit published baseline") {
			t.Fatalf("performance input admission = %v", err)
		}
	}
}
