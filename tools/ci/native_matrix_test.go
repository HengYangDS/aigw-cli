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
	"time"

	"github.com/pelletier/go-toml/v2"
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

func recordNative(t *testing.T, args ...string) []command {
	t.Helper()
	var calls []command
	if err := run(append([]string{"native"}, args...), &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	}); err != nil {
		t.Fatalf("native %v: %v", args, err)
	}
	return calls
}

func runsTheMatrix(calls []command) int {
	want := commandArguments([]command{selectionMatrixCommand})[0]
	count := 0
	for _, call := range commandArguments(calls) {
		if slices.Equal(call, want) {
			count++
		}
	}
	return count
}

func TestNativeSelectionMatrixRunsAfterTheSourceCheckInTheTestsEnvironment(t *testing.T) {
	nativeMatrixEnvironment(t)
	plain := commandArguments(recordNative(t))
	calls := recordNative(t, "--selection-matrix")
	want := slices.Insert(slices.Clone(plain), 1, commandArguments([]command{selectionMatrixCommand})[0])
	if got := commandArguments(calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("native --selection-matrix = %q, want the matrix between the source check and the tests: %q", got, want)
	}
	// The matrix sees the coverage run's isolated environment, never the job's
	// own temporary folder or Forge credentials.
	if matrix, tests := calls[1].Env, calls[len(calls)-1].Env; len(tests) == 0 || !slices.Equal(matrix, tests) {
		t.Fatalf("matrix environment = %q, want the coverage run's %q", matrix, tests)
	}
}

func TestNativeSelectionMatrixRunsOnceWithFullQuality(t *testing.T) {
	nativeMatrixEnvironment(t)
	full := commandArguments(recordNative(t, "--full-quality"))
	both := recordNative(t, "--selection-matrix", "--full-quality")
	if !reflect.DeepEqual(commandArguments(both), full) {
		t.Fatalf("full quality with the matrix flag = %q, want the full quality run %q", commandArguments(both), full)
	}
	if count := runsTheMatrix(both); count != 1 {
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
	for selected, want := range map[bool]int{false: 0, true: 1} {
		var calls []command
		host := nativeHost{getenv: os.Getenv, platform: runtime.GOOS, workdir: "/work", selectionMatrix: selected}
		if err := runNativeJob(host, &bytes.Buffer{}, func(call command) error {
			calls = append(calls, call)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if got := runsTheMatrix(calls); got != want {
			t.Fatalf("GitLab native job with selectionMatrix=%v ran the matrix %d times, want %d", selected, got, want)
		}
	}
}

func TestLinuxSecretServiceSessionCarriesTheMatrixChoice(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PROJECT_URL": "https://gitlab.example/aigw", "AIGW_CANDIDATE_ARTIFACTS": "/candidate"})
	for _, selected := range []bool{false, true} {
		var wrapped []command
		host := nativeHost{getenv: os.Getenv, platform: "linux", workdir: "/work", selectionMatrix: selected, attach: func(call command) error {
			wrapped = append(wrapped, call)
			return nil
		}}
		if err := runNativeJob(host, &bytes.Buffer{}, func(command) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if len(wrapped) != 1 || slices.Contains(wrapped[0].Args, "--selection-matrix") != selected {
			t.Fatalf("private session with selectionMatrix=%v = %#v", selected, wrapped)
		}
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

var nativePlatformArgument = regexp.MustCompile(`\./tools/ci native --platform (\S+)`)

// forgeJob is what the placement test reads of one job of a Forge.
type forgeJob struct {
	runner   string
	commands []string
}

// nativeMatrixPlacement maps each native platform to whether its command runs
// the selection matrix, and checks that exactly the native jobs on another
// runner than the Forge's quality job run it.
func nativeMatrixPlacement(t *testing.T, forge string, jobs map[string]forgeJob) map[string]bool {
	t.Helper()
	quality, ok := jobs["quality"]
	if !ok {
		t.Fatalf("%s has no quality job", forge)
	}
	placement := map[string]bool{}
	for name, job := range jobs {
		for _, line := range job.commands {
			match := nativePlatformArgument.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			flagged := strings.Contains(line, "--selection-matrix")
			if want := job.runner != quality.runner; flagged != want {
				t.Errorf("%s job %s runs the matrix: %v, want %v (its runner %q, quality's %q)", forge, name, flagged, want, job.runner, quality.runner)
			}
			placement[match[1]] = flagged
		}
	}
	return placement
}

type githubVerifyJob struct {
	RunsOn  string `yaml:"runs-on"`
	Timeout any    `yaml:"timeout-minutes"`
	Steps   []struct {
		Run string `yaml:"run"`
	} `yaml:"steps"`
}

func readGitHubVerify(t *testing.T) map[string]githubVerifyJob {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".github", "workflows", "verify.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]githubVerifyJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs
}

func TestEachForgeRunsTheSelectionMatrixOnEveryPlatformWithoutItsQualityJob(t *testing.T) {
	github := map[string]forgeJob{}
	for name, job := range readGitHubVerify(t) {
		var lines []string
		for _, step := range job.Steps {
			lines = append(lines, step.Run)
		}
		github[name] = forgeJob{runner: job.RunsOn, commands: lines}
	}
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".gitlab-ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	gitlab := map[string]forgeJob{}
	for name, value := range document {
		if job, ok := value.(map[string]any); ok {
			gitlab[name] = forgeJob{runner: strings.Join(anyStrings(job["tags"]), ","), commands: anyStrings(job["script"])}
		}
	}
	githubPlacement := nativeMatrixPlacement(t, "GitHub", github)
	gitlabPlacement := nativeMatrixPlacement(t, "GitLab", gitlab)
	if !reflect.DeepEqual(githubPlacement, gitlabPlacement) {
		t.Fatalf("the Forges run the matrix on different platforms: GitHub %v, GitLab %v", githubPlacement, gitlabPlacement)
	}
	if values := slices.Collect(maps.Values(githubPlacement)); !slices.Contains(values, true) || !slices.Contains(values, false) {
		t.Fatalf("matrix placement %v names no platform that runs it or none that leaves it to the quality job", githubPlacement)
	}
}

// runsTheSelectionMatrix reports whether a command line runs the matrix: the
// quality graph (quality, source, the repository check, full native quality),
// the native matrix flag, or the tagged tests themselves.
func runsTheSelectionMatrix(line string) bool {
	for _, form := range []string{"./tools/ci quality", "./tools/ci source", "mise run check", "--selection-matrix", "--full-quality", "-tags=selection_matrix"} {
		if strings.Contains(line, form) {
			return true
		}
	}
	return false
}

func anyStrings(value any) []string {
	items, _ := value.([]any)
	texts := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			texts = append(texts, text)
		}
	}
	return texts
}

// selectionMatrixTimeout reads the bound that the matrix's command gives Go.
func selectionMatrixTimeout(t *testing.T) time.Duration {
	t.Helper()
	at := slices.IndexFunc(selectionMatrixCommand.Args, func(arg string) bool { return strings.HasPrefix(arg, "-timeout=") })
	if at < 0 {
		t.Fatalf("the selection matrix has no bound: %q", selectionMatrixCommand.Args)
	}
	bound, err := time.ParseDuration(strings.TrimPrefix(selectionMatrixCommand.Args[at], "-timeout="))
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

// The matrix's bound is three times its slowest measured run, rounded up to
// whole five minutes; the rule is pinned here, not its result.
func TestSelectionMatrixBoundIsThreeSlowestRuns(t *testing.T) {
	bound, floor := selectionMatrixTimeout(t), 3*selectionMatrixSlowestRun
	if bound < floor || bound >= floor+5*time.Minute || bound%(5*time.Minute) != 0 {
		t.Fatalf("the selection matrix's bound %v is not three times its slowest run %v rounded up to five minutes", bound, selectionMatrixSlowestRun)
	}
}

// Every GitHub job and proof gate that runs the selection matrix has a budget
// above the matrix's own bound. This is the necessary condition only; the time
// that a job spends before the matrix is measured, not tested (the design).
func TestEveryBudgetAroundTheSelectionMatrixExceedsItsBound(t *testing.T) {
	bound := selectionMatrixTimeout(t)
	checked := 0
	for name, job := range readGitHubVerify(t) {
		runs := false
		for _, step := range job.Steps {
			runs = runs || runsTheSelectionMatrix(step.Run)
		}
		if !runs {
			continue
		}
		checked++
		if minutes, ok := job.Timeout.(int); !ok || time.Duration(minutes)*time.Minute <= bound {
			t.Errorf("GitHub job %s gives the matrix %v minutes, not more than its bound %v", name, job.Timeout, bound)
		}
	}
	if checked == 0 {
		t.Fatal("no GitHub job runs the selection matrix")
	}
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".ethos", "profile.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		Proof struct {
			Gates []struct {
				Command []string `toml:"command"`
				Timeout int      `toml:"timeout_seconds"`
			} `toml:"gates"`
		} `toml:"proof"`
	}
	if err := toml.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	gates := 0
	for _, gate := range profile.Proof.Gates {
		if runsTheSelectionMatrix(strings.Join(gate.Command, " ")) {
			gates++
			if time.Duration(gate.Timeout)*time.Second <= bound {
				t.Errorf("the proof gate %q gives the matrix %d s, not more than its bound %v", gate.Command, gate.Timeout, bound)
			}
		}
	}
	if gates == 0 {
		t.Fatal("no proof gate runs the quality graph")
	}
}
