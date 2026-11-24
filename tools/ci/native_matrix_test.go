package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// nativeMatrixEnvironment clears the host inputs that a native run reads.
func nativeMatrixEnvironment(t *testing.T) {
	t.Helper()
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "")
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
}

// invocations reduces commands to what they run, without their environment.
func invocations(calls []command) []command {
	reduced := make([]command, 0, len(calls))
	for _, call := range calls {
		reduced = append(reduced, command{Name: call.Name, Args: call.Args})
	}
	return reduced
}

func recordNative(t *testing.T, args ...string) []command {
	t.Helper()
	var calls []command
	if err := run(append([]string{"native"}, args...), &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	}); err != nil {
		t.Fatalf("native %v: %v", args, err)
	}
	return invocations(calls)
}

func selectionMatrixGateCommand(t *testing.T) command {
	t.Helper()
	gate, ok := repositoryQualityGraph.gate(selectionMatrixGate)
	if !ok {
		t.Fatal("the quality graph has no selection-matrix gate")
	}
	return gate.Command
}

func TestNativeSelectionMatrixRunsTheQualityGraphsGateAfterTheSourceCheck(t *testing.T) {
	nativeMatrixEnvironment(t)
	plain := recordNative(t)
	matrix := recordNative(t, "--selection-matrix")
	want := slices.Insert(slices.Clone(plain), 1, selectionMatrixGateCommand(t))
	if !reflect.DeepEqual(matrix, want) {
		t.Fatalf("native --selection-matrix = %#v, want the gate between the source check and the tests: %#v", matrix, want)
	}
}

func TestNativeSelectionMatrixRunsOnceWithFullQuality(t *testing.T) {
	nativeMatrixEnvironment(t)
	full := recordNative(t, "--full-quality")
	both := recordNative(t, "--selection-matrix", "--full-quality")
	if !reflect.DeepEqual(both, full) {
		t.Fatalf("full quality with the matrix flag = %#v, want the full quality run %#v", both, full)
	}
	gate := selectionMatrixGateCommand(t)
	if count := len(slices.DeleteFunc(slices.Clone(both), func(call command) bool { return !reflect.DeepEqual(call, gate) })); count != 1 {
		t.Fatalf("full quality ran the selection matrix %d times, want once", count)
	}
}

func TestNativeSelectionMatrixSkipsAnArtifactOnlyJourney(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PIPELINE_SOURCE": "web", "CI_PROJECT_URL": "https://gitlab.example/aigw", "AIGW_CANDIDATE_ARTIFACTS": "/candidate"})
	var calls []command
	err := runNativeJob(nativeHost{getenv: os.Getenv, platform: "darwin", workdir: "/work", selectionMatrix: true}, &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	})
	if err != nil || len(calls) != 1 || !slices.Contains(calls[0].Args, "accept-native") {
		t.Fatalf("artifact-only native job with the matrix flag = %#v, %v; want only the release owner", calls, err)
	}
}

func TestNativeJobFromEnvironmentRunsTheSelectionMatrix(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PIPELINE_SOURCE": "push", "CI_PROJECT_URL": "https://gitlab.example/aigw", "AIGW_VERIFY_SYSTEM_KEYRING": "0"})
	for _, selected := range []bool{false, true} {
		var calls []command
		host := nativeHost{getenv: os.Getenv, platform: runtime.GOOS, workdir: "/work", selectionMatrix: selected}
		if err := runNativeJob(host, &bytes.Buffer{}, func(call command) error {
			calls = append(calls, call)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		gate := selectionMatrixGateCommand(t)
		runs := slices.ContainsFunc(invocations(calls), func(call command) bool { return reflect.DeepEqual(call, gate) })
		if runs != selected {
			t.Fatalf("GitLab native job with selectionMatrix=%v ran the matrix: %v (%#v)", selected, runs, calls)
		}
	}
}

func TestLinuxSecretServiceSessionKeepsTheSelectionMatrix(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PROJECT_URL": "https://gitlab.example/aigw", "AIGW_CANDIDATE_ARTIFACTS": "/candidate"})
	var wrapped []command
	host := nativeHost{getenv: os.Getenv, platform: "linux", workdir: "/work", selectionMatrix: true, attach: func(call command) error {
		wrapped = append(wrapped, call)
		return nil
	}}
	if err := runNativeJob(host, &bytes.Buffer{}, func(command) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(wrapped) != 1 || !slices.Contains(wrapped[0].Args, "--selection-matrix") {
		t.Fatalf("private session = %#v, want the matrix flag carried into it", wrapped)
	}
}

func TestNativeAcceptsTheSelectionMatrixWithItsGitLabForm(t *testing.T) {
	other := "linux"
	if runtime.GOOS == other {
		other = "darwin"
	}
	err := run([]string{"native", "--platform", other, "--selection-matrix", "--from-env"}, &bytes.Buffer{}, func(command) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "requires "+other+" host") {
		t.Fatalf("native --selection-matrix --from-env was not parsed: %v", err)
	}
}

func TestQualityGraphReportsAnUnknownGate(t *testing.T) {
	if _, ok := (qualityGraph{}).gate(selectionMatrixGate); ok {
		t.Fatal("an empty quality graph reported a selection-matrix gate")
	}
}

var nativePlatformArgument = regexp.MustCompile(`\./tools/ci native --platform (\S+)`)

// nativeMatrixPlacement maps each native job's platform to whether its command
// runs the selection matrix, and checks that the jobs which do run on another
// runner than the Forge's quality job.
func nativeMatrixPlacement(t *testing.T, forge string, runner func(job map[string]any) string, commands func(job map[string]any) []string, jobs map[string]map[string]any) map[string]bool {
	t.Helper()
	quality, ok := jobs["quality"]
	if !ok {
		t.Fatalf("%s has no quality job", forge)
	}
	placement := map[string]bool{}
	for name, job := range jobs {
		for _, line := range commands(job) {
			match := nativePlatformArgument.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			flagged := strings.Contains(line, "--selection-matrix")
			if want := runner(job) != runner(quality); flagged != want {
				t.Errorf("%s job %s runs the matrix: %v, want %v (its runner %q, quality's %q)", forge, name, flagged, want, runner(job), runner(quality))
			}
			placement[match[1]] = flagged
		}
	}
	return placement
}

func TestEachForgeRunsTheSelectionMatrixOnEveryPlatformWithoutItsQualityJob(t *testing.T) {
	root := repositoryRoot(t)
	read := func(relative string) []byte {
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var github struct {
		Jobs map[string]map[string]any `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(read(".github/workflows/verify.yml"), &github); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(read(".gitlab-ci.yml"), &document); err != nil {
		t.Fatal(err)
	}
	gitlab := map[string]map[string]any{}
	for name, value := range document {
		if job, ok := value.(map[string]any); ok {
			gitlab[name] = job
		}
	}
	githubPlacement := nativeMatrixPlacement(t, "GitHub", func(job map[string]any) string {
		runner, _ := job["runs-on"].(string)
		return runner
	}, func(job map[string]any) []string {
		var lines []string
		steps, _ := job["steps"].([]any)
		for _, step := range steps {
			if fields, ok := step.(map[string]any); ok {
				if line, ok := fields["run"].(string); ok {
					lines = append(lines, line)
				}
			}
		}
		return lines
	}, github.Jobs)
	gitlabPlacement := nativeMatrixPlacement(t, "GitLab", func(job map[string]any) string {
		tags, _ := job["tags"].([]any)
		return joinAny(tags)
	}, func(job map[string]any) []string {
		var lines []string
		script, _ := job["script"].([]any)
		for _, line := range script {
			if text, ok := line.(string); ok {
				lines = append(lines, text)
			}
		}
		return lines
	}, gitlab)
	if !reflect.DeepEqual(githubPlacement, gitlabPlacement) {
		t.Fatalf("the Forges run the matrix on different platforms: GitHub %v, GitLab %v", githubPlacement, gitlabPlacement)
	}
	if values := slices.Collect(maps.Values(githubPlacement)); !slices.Contains(values, true) || !slices.Contains(values, false) {
		t.Fatalf("matrix placement %v names no platform that runs it or none that leaves it to the quality job", githubPlacement)
	}
}

func joinAny(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, ",")
}
