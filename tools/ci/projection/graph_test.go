package projection

import (
	"maps"
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
		Quality  gitLabJob `yaml:"quality"`
		Darwin   gitLabJob `yaml:"native-darwin"`
		Linux    gitLabJob `yaml:"native-linux"`
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
	wantGitLab := []struct {
		If   string
		When string
	}{
		{If: "$CI_COMMIT_TAG"},
		{If: `$CI_PIPELINE_SOURCE == "merge_request_event" && ($CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "dev" || $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "main")`},
		{If: `$CI_PIPELINE_SOURCE == "push" && ($CI_COMMIT_BRANCH == "dev" || $CI_COMMIT_BRANCH == "main")`},
		{If: `$CI_PIPELINE_SOURCE == "web" || $CI_PIPELINE_SOURCE == "api"`},
		{When: "never"},
	}
	if len(gitlab.Workflow.Rules) != len(wantGitLab) {
		t.Fatalf("GitLab verification routes = %#v, want %#v", gitlab.Workflow.Rules, wantGitLab)
	}
	for index, want := range wantGitLab {
		got := gitlab.Workflow.Rules[index]
		if got.If != want.If || got.When != want.When {
			t.Errorf("GitLab verification route %d = %#v, want %#v", index, got, want)
		}
	}
	for name, job := range map[string]gitLabJob{
		"quality": gitlab.Quality, "native-darwin": gitlab.Darwin, "native-linux": gitlab.Linux,
	} {
		if len(job.Rules) != len(wantGitLab) || job.Rules[1].If != wantGitLab[1].If {
			t.Errorf("GitLab %s must verify reviews into both integration and release: %#v", name, job.Rules)
		}
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

func TestLinuxNativeJourneysUseTheSameLockedProductCommand(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeLinux gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Run string `yaml:"run"`
				If  string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	githubCommands := make([]string, 0, len(workflow.Jobs["native-linux"].Steps))
	for _, step := range workflow.Jobs["native-linux"].Steps {
		if step.Run != "" && (step.If == "" || step.If == "github.event_name != 'workflow_dispatch' || !inputs.full_quality") {
			githubCommands = append(githubCommands, step.Run)
		}
	}
	want := []string{"mise run bootstrap", "mise exec --locked -- go run ./tools/ci native --platform linux"}
	for forge, commands := range map[string][]string{
		"GitHub": githubCommands,
		"GitLab": gitlab.NativeLinux.Script,
	} {
		if forge == "GitLab" {
			commands = slices.DeleteFunc(slices.Clone(commands), func(command string) bool {
				return command == `if [ "${AIGW_REFRESH_LOCKS:-false}" = true ]; then mise run dependencies:resolve; fi`
			})
			commands[len(commands)-1] = strings.TrimSuffix(commands[len(commands)-1], ` --full-quality="${AIGW_FULL_NATIVE_QUALITY:-false}"`)
		}
		if !reflect.DeepEqual(commands, want) {
			t.Fatalf("%s native Linux commands = %q, want locked dependencies then one product journey", forge, commands)
		}
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
					Type     string `yaml:"type"`
					Default  bool   `yaml:"default"`
					Required bool   `yaml:"required"`
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
	input, present := github.On.Dispatch.Inputs["full_quality"]
	if !present || input.Type != "boolean" || input.Default || input.Required {
		t.Fatal("full native quality must be an optional disabled-by-default input")
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		ordinary, full := 0, 0
		for _, step := range github.Jobs["native-"+platform].Steps {
			base := "mise exec --locked -- go run ./tools/ci native --platform " + platform
			switch step.Run {
			case base:
				ordinary++
				if step.If != "github.event_name != 'workflow_dispatch' || !inputs.full_quality" {
					t.Fatalf("%s ordinary native selection = %q", platform, step.If)
				}
			case base + " --full-quality":
				full++
				if step.If != "github.event_name == 'workflow_dispatch' && inputs.full_quality" || step.Env["CGO_ENABLED"] != "1" {
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
		Linux  gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]gitLabJob{"darwin": gitlab.Darwin, "linux": gitlab.Linux} {
		want := "mise exec --locked -- go run ./tools/ci native --platform " + platform + ` --full-quality="${AIGW_FULL_NATIVE_QUALITY:-false}"`
		if !slices.Contains(job.Script, want) {
			t.Fatalf("GitLab %s lacks the same explicit native-quality entrypoint", platform)
		}
	}
}

func TestGitHubDarwinSystemCredentialJourneyRequiresAnEphemeralHost(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
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
	index := slices.IndexFunc(steps, func(step struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Run native macOS acceptance"
	})
	if index < 0 {
		t.Fatal("GitHub native macOS acceptance step is missing")
	}
	want := map[string]string{
		"AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE": "ephemeral-host",
		"AIGW_VERIFY_SYSTEM_KEYRING":        "1",
	}
	if !reflect.DeepEqual(steps[index].Env, want) {
		t.Fatalf("GitHub native macOS credential admission = %#v, want %#v", steps[index].Env, want)
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
	wantGitLabJobs := []string{"accepted-ref-parity", "native-darwin", "native-linux", "quality", "release-assets", "release-readiness"}
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
		want := map[string]string{
			"GIT_CONFIG_COUNT":   "1",
			"GIT_CONFIG_KEY_0":   "init.defaultBranch",
			"GIT_CONFIG_VALUE_0": "main",
		}
		for name, value := range want {
			if got := workflow.Env[name]; got != value {
				t.Fatalf("GitHub projection %d %s = %q, want %q", index, name, got, value)
			}
		}
	}
}

type gitLabJob struct {
	BeforeScript []string          `yaml:"before_script"`
	Extends      []string          `yaml:"extends"`
	Image        gitLabImage       `yaml:"image"`
	Needs        []gitLabNeed      `yaml:"needs"`
	Script       []string          `yaml:"script"`
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
		"release-readiness":   {Stage: "verify", Needs: []string{}, Claims: []string{"release-metadata"}},
		"release-assets":      {Stage: "release", Rank: 1, Needs: []string{"quality", "native-darwin", "native-linux", "release-readiness"}, Claims: []string{"artifact-verification"}},
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

type gitLabImage struct {
	Name       string   `yaml:"name"`
	Entrypoint []string `yaml:"entrypoint"`
}

func TestForgeProjectionsFollowDeclaredNativeCapacity(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeWindows *gitLabJob `yaml:"native-windows"`
		Assets        gitLabJob  `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.NativeWindows != nil {
		t.Fatal("GitLab projection contains Windows without declared execution capacity")
	}
	if strings.Contains(projections[0].Content, "AIGW_GITLAB_WINDOWS_RUNNER_TAG") ||
		strings.Contains(projections[0].Content, "allow_failure:") {
		t.Fatal("GitLab projection retains a disabled Windows runner surface")
	}
	for _, need := range gitlab.Assets.Needs {
		if need.Job == "native-windows" {
			t.Fatal("GitLab package duplicates product-level native Windows admission")
		}
	}

	for _, projectionIndex := range []int{1} {
		var github struct {
			Jobs map[string]any `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projections[projectionIndex].Content), &github); err != nil {
			t.Fatal(err)
		}
		if _, present := github.Jobs["native-windows"]; !present {
			t.Fatalf("GitHub projection %d lacks required native Windows evidence", projectionIndex)
		}
	}
}
