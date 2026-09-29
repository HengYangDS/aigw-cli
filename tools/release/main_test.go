package main

import (
	"aigw-cli/tools/release/construction"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/robustio"
)

var nativeSourceFixtures struct {
	sync.Mutex
	directory string
	stages    map[string]string
}

func TestMain(m *testing.M) {
	if handled, code := runInstalledClientFixture(os.Args[0], os.Args[1:]); handled {
		os.Exit(code)
	}
	if len(os.Args) == 4 && os.Args[1] == "credential" && os.Getenv("AIGW_TEST_EXTERNAL_CREDENTIAL") == "1" {
		if os.Args[2] != os.Getenv("AIGW_TEST_EXTERNAL_CLIENT") || os.Args[3] != os.Getenv("AIGW_TEST_EXTERNAL_FINGERPRINT") {
			os.Exit(2)
		}
		_, _ = fmt.Fprintln(os.Stdout, "native-real-client-token")
		os.Exit(0)
	}
	code := m.Run()
	if nativeSourceFixtures.directory != "" {
		if err := robustio.RemoveAll(nativeSourceFixtures.directory); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "remove native test fixtures:", err)
			code = 1
		}
	}
	os.Exit(code)
}

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
	stage := cachedNativeSource(t, root, version)
	base, _ := nativeArchiveNames(version)
	return filepath.Join(stage, base, executableName())
}

func cachedNativeSource(t *testing.T, root, version string) string {
	t.Helper()
	nativeSourceFixtures.Lock()
	defer nativeSourceFixtures.Unlock()
	key := root + "\x00" + version
	if stage := nativeSourceFixtures.stages[key]; stage != "" {
		return stage
	}
	if nativeSourceFixtures.directory == "" {
		directory, err := os.MkdirTemp("", "aigw-native-test-fixtures-*")
		if err != nil {
			t.Fatal(err)
		}
		nativeSourceFixtures.directory = directory
		nativeSourceFixtures.stages = make(map[string]string)
	}
	workspace, err := os.MkdirTemp(nativeSourceFixtures.directory, "source-*")
	if err != nil {
		t.Fatal(err)
	}
	stage, err := construction.BuildNative(t.Context(), root, workspace, version)
	if err != nil {
		t.Fatalf("build native product %s: %v", version, err)
	}
	nativeSourceFixtures.stages[key] = stage
	return stage
}

func TestNativeSourceCandidateReusesProductBytes(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	firstProgram, firstArchive, firstChecksums := nativeReleaseCandidate(t, root, version)
	secondProgram, secondArchive, secondChecksums := nativeReleaseCandidate(t, root, version)
	if firstProgram != secondProgram || firstArchive != secondArchive || firstChecksums != secondChecksums {
		t.Fatal("native candidate rebuilt identical source")
	}
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
	if repeated := buildNativeProgram(t, root, "0.0.0"); repeated != program {
		t.Fatalf("native fixture rebuilt identical source: first=%q repeated=%q", program, repeated)
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
