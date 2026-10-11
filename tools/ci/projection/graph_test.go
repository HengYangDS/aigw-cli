package projection

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestVerificationRoutingCoversReviewAndMaintainerPaths(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab struct {
		Quality  gitLabJob  `yaml:"quality"`
		Darwin   gitLabJob  `yaml:"native-darwin"`
		Linux    *gitLabJob `yaml:"native-linux"`
		Windows  *gitLabJob `yaml:"native-windows"`
		Workflow struct {
			Rules []struct {
				If   string `yaml:"if"`
				When string `yaml:"when"`
			} `yaml:"rules"`
		} `yaml:"workflow"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	wantGitLabWorkflow := []struct {
		If   string
		When string
	}{
		{If: "$CI_COMMIT_TAG"},
		{If: `$CI_PIPELINE_SOURCE == "merge_request_event" && ($CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "dev" || $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "main")`},
		{If: `$CI_PIPELINE_SOURCE == "push" && ($CI_COMMIT_BRANCH == "dev" || $CI_COMMIT_BRANCH == "main")`},
		{If: `($CI_PIPELINE_SOURCE == "web" || $CI_PIPELINE_SOURCE == "api") && $CI_COMMIT_REF_PROTECTED == "true"`},
		{If: `($CI_PIPELINE_SOURCE == "web" || $CI_PIPELINE_SOURCE == "api") && $CI_COMMIT_REF_PROTECTED == "false"`},
		{When: "never"},
	}
	if len(gitlab.Workflow.Rules) != len(wantGitLabWorkflow) {
		t.Fatalf("GitLab verification routes = %#v, want %#v", gitlab.Workflow.Rules, wantGitLabWorkflow)
	}
	for index, want := range wantGitLabWorkflow {
		got := gitlab.Workflow.Rules[index]
		if got.If != want.If || got.When != want.When {
			t.Errorf("GitLab verification route %d = %#v, want %#v", index, got, want)
		}
	}
	for name, job := range map[string]gitLabJob{
		"quality":       gitlab.Quality,
		"native-darwin": gitlab.Darwin,
	} {
		if len(job.Rules) != 5 || job.Rules[1].If != wantGitLabWorkflow[1].If {
			t.Errorf("GitLab %s must verify reviews into both integration and release: %#v", name, job.Rules)
			continue
		}
		if got := job.Rules[2].If; got != wantGitLabWorkflow[2].If {
			t.Errorf("GitLab %s accepted-push rule = %q", name, got)
		}
	}
	if gitlab.Linux == nil || gitlab.Windows == nil {
		t.Fatal("GitLab review graph lacks a required native platform")
	}

	var github struct {
		On struct {
			Push struct {
				Branches []string `yaml:"branches"`
				Tags     []string `yaml:"tags"`
			} `yaml:"push"`
			PullRequest struct {
				Branches []string `yaml:"branches"`
			} `yaml:"pull_request"`
			WorkflowDispatch any `yaml:"workflow_dispatch"`
		} `yaml:"on"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(github.On.Push.Branches, []string{"dev", "main"}) || !slices.Equal(github.On.Push.Tags, []string{"v*"}) {
		t.Fatalf("GitHub protected-branch routes = %q, want [dev main]", github.On.Push.Branches)
	}
	if !slices.Equal(github.On.PullRequest.Branches, []string{"dev", "main"}) {
		t.Errorf("GitHub review targets = %q, want [dev main]", github.On.PullRequest.Branches)
	}
	if github.On.WorkflowDispatch == nil {
		t.Fatal("GitHub verification lacks the explicit maintainer dispatch route")
	}
}

func TestGitHubLinuxNativeJourneyUsesTheLockedProductCommand(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	githubCommands := make([]string, 0, len(workflow.Jobs["native-linux"].Steps))
	for _, step := range workflow.Jobs["native-linux"].Steps {
		if step.Name == "Prepare locked dependencies" || step.Name == "Run native Linux acceptance" {
			githubCommands = append(githubCommands, step.Run)
		}
	}
	if len(githubCommands) != 2 || githubCommands[0] != "mise run bootstrap" {
		t.Fatalf("GitHub native Linux commands = %q, want locked dependencies then one product journey", githubCommands)
	}
	for _, required := range []string{
		"dbus-run-session", "SetAlias default /org/freedesktop/secrets/collection/session",
		"AIGW_VERIFY_SYSTEM_KEYRING=1", "mise exec --locked -- go run ./tools/ci native --platform linux",
	} {
		if !strings.Contains(githubCommands[1], required) {
			t.Fatalf("native Linux command lacks its isolated credential contract %q", required)
		}
	}
	var gitlab struct {
		Linux gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(gitlab.Linux.Script, func(call string) bool {
		return strings.Contains(call, "dbus-run-session") && strings.Contains(call, "AIGW_VERIFY_SYSTEM_KEYRING=1") &&
			strings.Contains(call, "go run ./tools/ci native --platform linux")
	})
	if index < 0 {
		t.Fatal("GitLab native Linux command lacks the same isolated credential contract")
	}
	for _, peer := range []struct{ name, command string }{
		{"GitHub", githubCommands[1]},
		{"GitLab", gitlab.Linux.Script[index]},
	} {
		t.Run(peer.name, func(t *testing.T) {
			_, body, found := strings.Cut(peer.command, "<<'AIGW_SECRET_SERVICE'\n")
			if !found {
				t.Fatal("native Linux command lacks its Secret Service session body")
			}
			body, found = strings.CutSuffix(strings.TrimSpace(body), "AIGW_SECRET_SERVICE")
			if !found {
				t.Fatal("native Linux Secret Service session is not terminated")
			}
			command := exec.CommandContext(t.Context(), "bash", "-n")
			command.Stdin = strings.NewReader(body)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("invalid native Linux Secret Service body: %v: %s", err, output)
			}
		})
	}
}

func TestFullNativeQualityIsExplicitAndUsesTheExistingEntryPoint(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var github struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type     string    `yaml:"type"`
					Default  yaml.Node `yaml:"default"`
					Required bool      `yaml:"required"`
					Options  []string  `yaml:"options"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Run string            `yaml:"run"`
				If  string            `yaml:"if"`
				Env map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	input := github.On.Dispatch.Inputs["full_quality"]
	if input.Type != "boolean" || input.Default.Value != "false" || input.Required {
		t.Fatal("full native quality must be an optional disabled-by-default input")
	}
	selection := github.On.Dispatch.Inputs["native_platform"]
	if selection.Type != "choice" || selection.Required || selection.Default.Value != "all" || !slices.Equal(selection.Options, []string{"all", "darwin", "linux", "windows"}) {
		t.Fatalf("native qualification must default to the complete platform set: %#v", selection)
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		sourceCondition := "(github.event_name != 'workflow_dispatch' || (inputs.baseline_tag == '' && inputs.candidate_tag == '' && inputs.input_release == '' && !inputs.windows_clients && !inputs.macos_keychain && !inputs.performance))"
		ordinary, full := 0, 0
		for _, step := range github.Jobs["native-"+platform].Steps {
			base := "mise exec --locked -- go run ./tools/ci native --platform " + platform
			run := step.Run
			if strings.HasPrefix(run, "dbus-run-session") && !strings.Contains(run, "--candidate=") {
				run = base
				if strings.Contains(step.Run, "--full-quality") {
					run += " --full-quality"
				}
			}
			switch run {
			case base:
				ordinary++
				if step.If != sourceCondition+" && (github.event_name != 'workflow_dispatch' || !inputs.full_quality)" {
					t.Fatalf("%s ordinary native selection = %q", platform, step.If)
				}
			case base + " --full-quality":
				full++
				if !slices.Equal([]string{step.If, step.Env["CGO_ENABLED"]}, []string{sourceCondition + " && github.event_name == 'workflow_dispatch' && inputs.full_quality", "1"}) {
					t.Fatalf("%s full native selection or compiler capability is incomplete: %#v", platform, step)
				}
			}
		}
		if ordinary != 1 || full != 1 {
			t.Fatalf("%s native paths = %d/%d, want one mutually exclusive pair", platform, ordinary, full)
		}
	}
	var gitlab struct {
		Darwin gitLabJob `yaml:"native-darwin"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	wantRule := `($CI_PIPELINE_SOURCE == "web" || $CI_PIPELINE_SOURCE == "api") && ($AIGW_NATIVE_PLATFORM == null || $AIGW_NATIVE_PLATFORM == "" || $AIGW_NATIVE_PLATFORM == "all" || $AIGW_NATIVE_PLATFORM == "darwin")`
	if len(gitlab.Darwin.Rules) != 5 || gitlab.Darwin.Rules[3].If != wantRule {
		t.Fatalf("GitLab Darwin lacks equivalent manual platform selection: %#v", gitlab.Darwin.Rules)
	}
	want := `mise exec --locked -- go run ./tools/ci native --platform darwin --full-quality="${AIGW_FULL_NATIVE_QUALITY:-false}"`
	if !strings.Contains(strings.Join(gitlab.Darwin.Script, "\n"), want) {
		t.Fatal("GitLab Darwin lacks the same explicit native-quality entrypoint")
	}
}

func TestGitHubDarwinSystemCredentialJourneyRequiresAnEphemeralHost(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type     string    `yaml:"type"`
					Default  yaml.Node `yaml:"default"`
					Required bool      `yaml:"required"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["native-darwin"].Steps
	ordinary := slices.IndexFunc(steps, func(step struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Run native macOS acceptance"
	})
	if ordinary < 0 {
		t.Fatal("GitHub native macOS acceptance step is missing")
	}
	input, present := workflow.On.Dispatch.Inputs["macos_keychain"]
	if !present || input.Type != "boolean" || input.Default.Value != "false" || input.Required {
		t.Fatal("retained macOS Keychain qualification must be explicit and disabled by default")
	}
	if got := steps[ordinary].Env["AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"]; got != "ephemeral-host" {
		t.Fatalf("ordinary GitHub macOS credential scope = %q, want ephemeral-host", got)
	}
	if _, present := steps[ordinary].Env["AIGW_VERIFY_SYSTEM_KEYRING"]; present {
		t.Fatal("ordinary GitHub macOS acceptance must not read credential values")
	}
	historical := slices.IndexFunc(steps, func(step struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Run selected native artifact acceptance"
	})
	if historical < 0 {
		t.Fatal("GitHub selected native macOS artifact acceptance step is missing")
	}
	if got := steps[historical].Env["AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"]; got != "ephemeral-host" {
		t.Fatalf("historical GitHub macOS credential scope = %q, want ephemeral-host", got)
	}
	if got := steps[historical].Env["AIGW_VERIFY_SYSTEM_KEYRING"]; got != "${{ inputs.macos_keychain && '1' || '0' }}" {
		t.Fatalf("historical GitHub macOS credential selection = %q", got)
	}
}

func TestVerificationProjectsIndependentQualityAndNativeFacts(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []string{".linux-toolchain", "stages", "variables", "workflow"} {
		delete(gitlab, metadata)
	}
	wantGitLabJobs := []string{"accepted-ref-parity", "native-darwin", "native-linux", "native-windows", "quality", "release-assets", "release-version"}
	if got := slices.Sorted(maps.Keys(gitlab)); !slices.Equal(got, wantGitLabJobs) {
		t.Fatalf("GitLab jobs = %q, want %q", got, wantGitLabJobs)
	}
	var qualityJob gitLabJob
	qualityNode := gitlab["quality"]
	if err := qualityNode.Decode(&qualityJob); err != nil {
		t.Fatal(err)
	}
	if got := qualityJob.Script; !slices.Contains(got, "mise exec --locked -- go run ./tools/ci quality") {
		t.Fatalf("GitLab quality job = %#v", got)
	}

	var github struct {
		Jobs map[string]struct {
			Needs []string `yaml:"needs"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	wantGitHubJobs := []string{"accepted-ref-parity", "native-darwin", "native-linux", "native-windows", "quality"}
	if got := slices.Sorted(maps.Keys(github.Jobs)); !slices.Equal(got, wantGitHubJobs) {
		t.Fatalf("GitHub jobs = %q, want %q", got, wantGitHubJobs)
	}
	quality := github.Jobs["quality"]
	if !slices.ContainsFunc(quality.Steps, func(step struct {
		Run string `yaml:"run"`
	}) bool {
		return step.Run == "mise exec --locked -- go run ./tools/ci quality"
	}) {
		t.Fatalf("GitHub quality job = %#v", quality.Steps)
	}
}

func TestGitHubVerificationChecksOutTheExactProductCommit(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	const productRef = "${{ github.event.pull_request.head.sha || github.sha }}"
	for name, job := range workflow.Jobs {
		index := slices.IndexFunc(job.Steps, func(step struct {
			Uses string            `yaml:"uses"`
			With map[string]string `yaml:"with"`
		}) bool {
			return strings.HasPrefix(step.Uses, "actions/checkout@")
		})
		if index < 0 {
			t.Fatalf("%s lacks checkout", name)
		}
		if got := job.Steps[index].With["ref"]; got != productRef {
			t.Fatalf("%s checkout ref = %q, want exact product ref %q", name, got, productRef)
		}
	}
}

func TestGitHubWorkflowsDeclareTheCanonicalInitialBranch(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{1, 2} {
		var workflow struct {
			Env map[string]string `yaml:"env"`
		}
		if err := yaml.Unmarshal([]byte(projections[index].Content), &workflow); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("git", "config", "--default=unselected", "--get", "init.defaultBranch")
		command.Dir = t.TempDir()
		command.Env = slices.DeleteFunc(os.Environ(), func(value string) bool {
			name, _, _ := strings.Cut(value, "=")
			return strings.HasPrefix(name, "GIT_CONFIG_") || name == "GIT_DIR" || name == "GIT_WORK_TREE"
		})
		command.Env = append(command.Env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		for name, value := range workflow.Env {
			command.Env = append(command.Env, name+"="+value)
		}
		branch, err := command.Output()
		if err != nil || strings.TrimSpace(string(branch)) != "main" {
			t.Fatalf("GitHub projection %d initial branch = %q, error = %v", index, branch, err)
		}
	}
}

type gitLabJob struct {
	BeforeScript []string          `yaml:"before_script"`
	Extends      []string          `yaml:"extends"`
	Image        string            `yaml:"image"`
	Needs        []gitLabNeed      `yaml:"needs"`
	Script       []string          `yaml:"script"`
	Tags         []string          `yaml:"tags"`
	Variables    map[string]string `yaml:"variables"`
	Rules        []struct {
		If        string            `yaml:"if"`
		When      string            `yaml:"when"`
		Variables map[string]string `yaml:"variables"`
	} `yaml:"rules"`
}

func TestSemanticGraphDefinesExactClaimsAndEvidenceReuse(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	export := func(expression string, target any) {
		t.Helper()
		output, err := projectionCommand(root, expression).Output()
		if err != nil {
			t.Fatalf("export %s: %v", expression, err)
		}
		if err := yaml.Unmarshal(output, target); err != nil {
			t.Fatalf("decode %s: %v", expression, err)
		}
	}

	type job struct {
		Stage  string   `yaml:"stage"`
		Rank   int      `yaml:"rank"`
		Needs  []string `yaml:"needs"`
		Claims []string `yaml:"claims"`
	}
	var graph map[string]job
	export("graph", &graph)
	wantGraph := map[string]job{
		"accepted-ref-parity": {Stage: "verify", Needs: []string{}, Claims: []string{"accepted-ref-parity"}},
		"quality":             {Stage: "verify", Needs: []string{}, Claims: []string{"source-quality"}},
		"native-darwin":       {Stage: "verify", Needs: []string{}, Claims: []string{"go-source-compatibility", "native-product-journey", "lifecycle-acceptance"}},
		"native-linux":        {Stage: "verify", Needs: []string{}, Claims: []string{"go-source-compatibility", "native-product-journey", "lifecycle-acceptance"}},
		"native-windows":      {Stage: "verify", Needs: []string{}, Claims: []string{"go-source-compatibility", "native-product-journey", "lifecycle-acceptance"}},
		"release-version":     {Stage: "verify", Needs: []string{}, Claims: []string{"release-metadata"}},
		"release-assets":      {Stage: "release", Rank: 1, Needs: []string{"quality", "native-darwin", "native-linux", "native-windows", "release-version"}, Claims: []string{"artifact-verification"}},
	}
	if !reflect.DeepEqual(graph, wantGraph) {
		t.Fatalf("CI graph = %#v, want %#v", graph, wantGraph)
	}

	var matrix struct {
		VersionSource string `yaml:"versionSource"`
		ToolchainLock string `yaml:"toolchainLock"`
		Platforms     map[string]struct {
			Job     string `yaml:"job"`
			Command string `yaml:"command"`
		} `yaml:"platforms"`
	}
	export("goSourceMatrix", &matrix)
	if matrix.VersionSource != "go.mod" || matrix.ToolchainLock != "mise.lock" {
		t.Fatalf("Go matrix authority = %#v", matrix)
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		entry, present := matrix.Platforms[platform]
		if !present || entry.Job != "native-"+platform || entry.Command != "mise exec --locked -- go run ./tools/ci native --platform "+platform {
			t.Fatalf("Go matrix %s = %#v", platform, entry)
		}
	}
	if len(matrix.Platforms) != 3 {
		t.Fatalf("Go matrix platforms = %#v", matrix.Platforms)
	}

	var reuse struct {
		CrossRun      bool     `yaml:"crossRun"`
		InvalidatedBy []string `yaml:"invalidatedBy"`
		SamePipeline  []struct {
			Producer  string   `yaml:"producer"`
			Consumers []string `yaml:"consumers"`
			Identity  string   `yaml:"identity"`
		} `yaml:"samePipeline"`
	}
	export("evidenceReuse", &reuse)
	wantInvalidators := []string{"source", "platform", "environment", "dependency-locks", "toolchain", "release-identity", "claimed-fact"}
	if reuse.CrossRun || !slices.Equal(reuse.InvalidatedBy, wantInvalidators) || len(reuse.SamePipeline) != 0 {
		t.Fatalf("evidence reuse = %#v", reuse)
	}
}

type gitLabNeed struct {
	Job string `yaml:"job"`
}
