package main

import (
	"aigw-cli/tools/release/construction"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireNativeLifecycleBaseline(t *testing.T, buildFixture func() string) string {
	t.Helper()
	baseline, err := nativeLifecycleBaseline(buildFixture)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("AIGW_ACCEPTANCE_BASELINE") == "" {
		return baseline
	}
	// Portable lifecycle commands cannot mutate an installer-owned source path.
	// Staging exact bytes tests that lifecycle, not the installer link or native
	// credential authorization of the original installation.
	data, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(t.TempDir(), executableName())
	if err := os.WriteFile(staged, data, 0o700); err != nil {
		t.Fatal(err)
	}
	return staged
}

func TestExplicitNativeBaselineStagesExactBytesOutsideItsInstallation(t *testing.T) {
	selected := filepath.Join(t.TempDir(), executableName())
	want := []byte("published predecessor bytes")
	mustWriteFile(t, selected, want, 0o700)
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", selected)
	staged := requireNativeLifecycleBaseline(t, func() string {
		t.Fatal("explicit baseline unexpectedly built a source fixture")
		return ""
	})
	if staged == selected || !filepath.IsAbs(staged) {
		t.Fatalf("explicit baseline was not staged separately: %q", staged)
	}
	if got := readFile(t, staged); !bytes.Equal(got, want) {
		t.Fatalf("staged baseline bytes = %q, want %q", got, want)
	}
	if got := readFile(t, selected); !bytes.Equal(got, want) {
		t.Fatalf("selected baseline was modified: %q", got)
	}
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
