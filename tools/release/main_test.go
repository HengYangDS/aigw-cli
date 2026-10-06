package main

import (
	"aigw-cli/internal/process"
	"aigw-cli/tools/release/construction"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/robustio"
)

var nativeSourceFixtures struct {
	sync.Mutex
	directory string
	stages    map[string]string
}

func TestMain(m *testing.M) {
	if len(os.Args) == 6 && os.Args[1] == "prepare-performance-setup" {
		if os.Args[2] != os.Getenv("AIGW_TEST_PERFORMANCE_CONFIG_ROOT") || os.Args[3] != os.Getenv("AIGW_TEST_PERFORMANCE_SETTINGS_ROOT") {
			os.Exit(2)
		}
		if err := preparePerformanceSetup(os.Args[2], os.Args[3], os.Args[4], os.Args[5]); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
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

func preparePerformanceSetup(configRoot, settingsRoot, config, settings string) (result error) {
	roots := []string{configRoot, settingsRoot}
	paths := [][]string{{config}, {settings, settings + ".aigw-state.json"}}
	owned := make([]*os.Root, len(roots))
	defer func() {
		for _, root := range owned {
			if root != nil {
				result = errors.Join(result, root.Close())
			}
		}
	}()
	for index, root := range roots {
		if !filepath.IsAbs(root) {
			return fmt.Errorf("performance setup requires absolute owned directories")
		}
		var err error
		owned[index], err = os.OpenRoot(root)
		if err != nil {
			return fmt.Errorf("open performance setup directory: %w", err)
		}
		for file, path := range paths[index] {
			relative, err := filepath.Rel(root, path)
			if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
				return fmt.Errorf("performance setup input is outside its owned directory")
			}
			paths[index][file] = relative
			info, err := owned[index].Lstat(relative)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("performance setup input is not an owned regular file")
			}
		}
	}
	for index, files := range paths {
		for _, path := range files {
			if err := owned[index].Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("prepare empty performance configuration: %w", err)
			}
		}
	}
	return nil
}

func TestPerformanceSetupPreparation(t *testing.T) {
	preparer, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"owned", "missing", "foreign", "directory", "unbound"} {
		configRoot, settingsRoot := t.TempDir(), t.TempDir()
		config, settings := filepath.Join(configRoot, "config.toml"), filepath.Join(settingsRoot, "settings.json")
		paths := []string{config, settings, settings + ".aigw-state.json"}
		for _, path := range paths {
			mustWriteFile(t, path, []byte("owned"), 0o600)
		}
		preserved := []string{filepath.Join(configRoot, "credentials"), filepath.Join(settingsRoot, "unrelated.json")}
		for _, path := range preserved {
			mustWriteFile(t, path, []byte("retained"), 0o600)
		}
		selected := settings
		invalid := mode == "foreign" || mode == "directory" || mode == "unbound"
		if mode == "foreign" {
			selected = filepath.Join(t.TempDir(), "foreign.toml")
			mustWriteFile(t, selected, []byte("foreign"), 0o600)
		}
		if mode == "directory" {
			selected = settingsRoot
		}
		if mode == "missing" {
			if err := os.Remove(config); err != nil {
				t.Fatal(err)
			}
		}
		environment := map[string]string{"AIGW_TEST_PERFORMANCE_CONFIG_ROOT": configRoot, "AIGW_TEST_PERFORMANCE_SETTINGS_ROOT": settingsRoot}
		if mode == "unbound" {
			environment["AIGW_TEST_PERFORMANCE_SETTINGS_ROOT"] = t.TempDir()
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		stdout, stderr, err := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
			Executable: preparer, Args: []string{"prepare-performance-setup", configRoot, settingsRoot, config, selected},
			Env: environmentWith(os.Environ(), environment),
		})
		cancel()
		if (err != nil) != invalid || len(stdout) != 0 || (!invalid && len(stderr) != 0) {
			t.Fatalf("preparation ownership decision: %v; stdout=%q stderr=%q", err, stdout, stderr)
		}
		for _, path := range paths {
			_, err := os.Stat(path)
			if errors.Is(err, os.ErrNotExist) == invalid {
				t.Fatalf("preparation changed the wrong managed input %s: %v", path, err)
			}
		}
		for _, path := range preserved {
			if string(readFile(t, path)) != "retained" {
				t.Fatalf("preparation changed an unrelated input: %s", path)
			}
		}
	}
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

func TestNativeLifecycleBaselineSelection(t *testing.T) {
	t.Run("source fixture", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
		t.Setenv("AIGW_ACCEPTANCE_RELEASE", "")
		got, err := nativeLifecycleBaseline(func() string { return "/built/fixture" })
		if err != nil || got != "/built/fixture" {
			t.Fatalf("source fixture = %q, %v", got, err)
		}
	})
	t.Run("prebuilt lifecycle requires its admitted predecessor", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_RELEASE", t.TempDir())
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
		built := false
		_, err := nativeLifecycleBaseline(func() string {
			built = true
			return "/built/fixture"
		})
		if err == nil || built {
			t.Fatalf("prebuilt lifecycle substituted a source fixture: built=%v, error=%v", built, err)
		}
	})
	t.Run("explicit released binary", func(t *testing.T) {
		baseline := filepath.Join(t.TempDir(), executableName())
		mustWriteFile(t, baseline, []byte("baseline"), 0o700)
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", baseline)
		built := false
		got, err := nativeLifecycleBaseline(func() string {
			built = true
			return "/built/fixture"
		})
		if err != nil || got != baseline {
			t.Fatalf("released baseline = %q, %v", got, err)
		}
		if built {
			t.Fatal("explicit released baseline still built its source fallback")
		}
	})
	t.Run("unavailable release is not replaced by fixture", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", filepath.Join(t.TempDir(), "missing"))
		if _, err := nativeLifecycleBaseline(func() string { return "/built/fixture" }); err == nil {
			t.Fatal("missing requested baseline was silently replaced")
		}
	})
	t.Run("directory is not a release executable", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", t.TempDir())
		if _, err := nativeLifecycleBaseline(func() string { return "/built/fixture" }); err == nil {
			t.Fatal("directory accepted as a release executable")
		}
	})
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

func TestNativeProductSelectionDoesNotBuildUnselectedFixtures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	program, _, _ := nativeReleaseCandidate(t, root, version)
	selected, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		test, subtest string
		baseline      bool
	}{
		{"TestNativeProductJourney", "ephemeral_endpoint_credentials", false},
		{"TestNativeProductJourney", "delayed_token_and_client_activation", false},
		{"TestNativeProductJourney", "one_selected_account_does_not_require_every_token", false},
		{"TestNativeProductJourney", "claude", false},
		{"TestNativeProductJourney", "portable_artifact_lifecycle", true},
		{"TestNativeRollbackConfigurationAdmission", "", true},
	} {
		t.Run(fixture.test+"/"+fixture.subtest, func(t *testing.T) {
			missing := filepath.Join(t.TempDir(), "unselected")
			baseline := missing
			if fixture.baseline {
				baseline = requireNativeLifecycleBaseline(t, func() string { return buildNativeProgram(t, root, "0.0.0") })
			}
			pattern, marker := "^"+fixture.test+"$", "--- PASS: "+fixture.test
			if fixture.subtest != "" {
				pattern += "/^" + fixture.subtest + "$"
				marker += "/" + fixture.subtest
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, selected, "-test.run="+pattern, "-test.count=1", "-test.timeout=20s", "-test.v")
			command.WaitDelay = 2 * time.Second
			command.Dir = filepath.Join(root, "tools", "release")
			command.Env = environmentWith(os.Environ(), map[string]string{
				"PATH":                       missing,
				"AIGW_ACCEPTANCE_RELEASE":    filepath.Dir(filepath.Dir(program)),
				"AIGW_ACCEPTANCE_BASELINE":   baseline,
				"AIGW_VERIFY_SYSTEM_KEYRING": "0",
			})
			output, err := command.CombinedOutput()
			if ctx.Err() != nil || err != nil || !bytes.Contains(output, []byte(marker)) || bytes.Contains(output, []byte("FailNow on a parent test")) {
				t.Fatalf("selected native fixture violated build or failure ownership: %v\n%s", err, output)
			}
		})
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
