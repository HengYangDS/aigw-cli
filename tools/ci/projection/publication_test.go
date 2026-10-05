package projection

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

func TestQualityJobsProjectEventCommitBases(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab struct {
		Quality      gitLabJob `yaml:"quality"`
		NativeDarwin gitLabJob `yaml:"native-darwin"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	wantGitLabBases := []string{
		"$CI_COMMIT_SHA^",
		"$CI_MERGE_REQUEST_DIFF_BASE_SHA",
		"$CI_COMMIT_BEFORE_SHA",
		"$CI_COMMIT_SHA^",
	}
	for index, want := range wantGitLabBases {
		if got := gitlab.Quality.Rules[index].Variables["AIGW_COMMIT_BASE"]; got != want {
			t.Fatalf("GitLab commit base rule %d = %q, want %q", index, got, want)
		}
	}
	for index, rule := range gitlab.NativeDarwin.Rules {
		if _, leaked := rule.Variables["AIGW_COMMIT_BASE"]; leaked {
			t.Fatalf("GitLab native rule %d received source-only commit-base state", index)
		}
	}

	var github struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	steps := github.Jobs["quality"].Steps
	index := slices.IndexFunc(steps, func(step struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Run quality and governance"
	})
	if index < 0 {
		t.Fatal("GitHub quality job lacks quality-and-governance step")
	}
	githubBase := steps[index].Env["AIGW_COMMIT_BASE"]
	for _, source := range []string{"github.event.pull_request.base.sha", "github.event.before", "inputs.commit_base"} {
		if !strings.Contains(githubBase, source) {
			t.Fatalf("GitHub commit base lacks %s: %q", source, githubBase)
		}
	}
	if strings.Count(githubBase, "format('{0}^', github.sha)") != 2 {
		t.Fatalf("GitHub manual run does not default to the selected commit's parent: %q", githubBase)
	}

	var manual struct {
		On struct {
			WorkflowDispatch struct {
				Inputs map[string]struct {
					Required bool `yaml:"required"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &manual); err != nil {
		t.Fatal(err)
	}
	if input, ok := manual.On.WorkflowDispatch.Inputs["commit_base"]; !ok || input.Required {
		t.Fatalf("GitHub manual diagnostics must allow an omitted commit_base: %#v", manual.On.WorkflowDispatch.Inputs)
	}
}

func TestGitLabReleaseAssetVerificationUsesLinuxContainer(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Assets       gitLabJob `yaml:"release-assets"`
		NativeDarwin gitLabJob `yaml:"native-darwin"`
		NativeLinux  gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pipeline.Assets.Extends, []string{".linux-toolchain"}) ||
		!slices.Equal(pipeline.Assets.Tags, pipeline.NativeLinux.Tags) ||
		slices.Equal(pipeline.Assets.Tags, pipeline.NativeDarwin.Tags) {
		t.Fatalf("release asset verifier is not isolated from the macOS Shell runner: %+v", pipeline.Assets)
	}
	if !slices.Contains(pipeline.Assets.Script, "mise exec --locked -- go run ./tools/release verify-artifacts dist") {
		t.Fatalf("release artifact verification was dropped: %q", pipeline.Assets.Script)
	}
}

func TestAcceptedPublicationChecksRefParityFromMain(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab struct {
		AcceptedRefParity gitLabJob `yaml:"accepted-ref-parity"`
		Quality           gitLabJob `yaml:"quality"`
		Darwin            gitLabJob `yaml:"native-darwin"`
		ReleaseAssets     gitLabJob `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	parity := gitlab.AcceptedRefParity
	if len(parity.Script) == 0 {
		t.Fatal("GitLab projection lacks accepted-ref-parity")
	}
	if len(parity.Rules) != 2 ||
		parity.Rules[0].If != `$CI_PIPELINE_SOURCE == "push" && $CI_COMMIT_BRANCH == "main"` ||
		parity.Rules[1].When != "never" {
		t.Fatalf("GitLab accepted parity rules = %#v", parity.Rules)
	}
	for name, job := range map[string]gitLabJob{
		"quality":       gitlab.Quality,
		"native-darwin": gitlab.Darwin,
	} {
		protectedPushRuleSeen := false
		for _, rule := range job.Rules {
			if rule.If == `$CI_PIPELINE_SOURCE == "push" && ($CI_COMMIT_BRANCH == "dev" || $CI_COMMIT_BRANCH == "main")` {
				protectedPushRuleSeen = true
			}
		}
		if !protectedPushRuleSeen {
			t.Fatalf("GitLab %s does not admit both protected branch pushes", name)
		}
	}
	for name, job := range map[string]gitLabJob{
		"release-assets": gitlab.ReleaseAssets,
	} {
		for _, rule := range job.Rules {
			if strings.Contains(rule.If, "$CI_COMMIT_BRANCH") {
				t.Fatalf("GitLab %s must remain tag-only, got branch rule", name)
			}
		}
	}

	var github struct {
		Jobs map[string]struct {
			If  string            `yaml:"if"`
			Env map[string]string `yaml:"env"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if got := github.Jobs["accepted-ref-parity"].If; got != "github.event_name == 'push' && github.ref_name == 'main'" {
		t.Fatalf("GitHub accepted parity condition = %q", got)
	}
	if got := github.Jobs["accepted-ref-parity"].Env["MISE_ENABLE_TOOLS"]; got != "go" {
		t.Fatalf("accepted-ref observation must install only its Go command dependency, got %q", got)
	}
	for name, job := range github.Jobs {
		if name == "accepted-ref-parity" {
			continue
		}
		if name == "release-version" {
			if job.If != "github.ref_type == 'tag'" {
				t.Fatalf("GitHub release version must run only for tags: %q", job.If)
			}
			continue
		}
		want := "github.ref_type == 'tag' || github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || (github.event_name == 'push' && (github.ref_name == 'dev' || github.ref_name == 'main'))"
		if platform, native := strings.CutPrefix(name, "native-"); native {
			want = "(" + want + ") && (github.event_name != 'workflow_dispatch' || github.ref_type == 'tag' || inputs.native_platform == '' || inputs.native_platform == 'all' || inputs.native_platform == '" + platform + "')"
		} else if name == "linux-secret-service" {
			want = "(" + want + ") && (github.event_name != 'workflow_dispatch' || github.ref_type == 'tag' || inputs.native_platform == '' || inputs.native_platform == 'all' || inputs.native_platform == 'linux')"
		} else if name == "quality" {
			want = "(" + want + ") && (github.event_name != 'workflow_dispatch' || github.ref_type == 'tag' || inputs.full_quality || inputs.refresh_locks || inputs.windows_clients || inputs.candidate_tag == '')"
		}
		if job.If != want {
			t.Fatalf("GitHub %s does not positively admit the full verification lifecycle: %q", name, job.If)
		}
	}
}

func TestVerificationConcurrencyPreservesIndependentManualRuns(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Concurrency struct {
			Group            string `yaml:"group"`
			CancelInProgress bool   `yaml:"cancel-in-progress"`
		} `yaml:"concurrency"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	const want = "verify-${{ github.workflow }}-${{ github.ref }}-${{ github.event_name == 'workflow_dispatch' && github.run_id || 'automatic' }}"
	if workflow.Concurrency.Group != want || !workflow.Concurrency.CancelInProgress {
		t.Fatalf("GitHub verification concurrency = %#v, want group %q with automatic-run cancellation", workflow.Concurrency, want)
	}
}

func TestManualHistoricalAcceptanceSelectsAnExplicitRelease(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Name string            `yaml:"name"`
		If   string            `yaml:"if"`
		Run  string            `yaml:"run"`
		Env  map[string]string `yaml:"env"`
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []step `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	const selection = "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.candidate_tag != '' || inputs.windows_clients || inputs.diagnostic_client != '' || inputs.macos_keychain || inputs.performance)"
	for _, platform := range []string{"darwin", "linux", "windows"} {
		steps := workflow.Jobs["native-"+platform].Steps
		index := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Run historical release acceptance" })
		if index < 0 {
			t.Fatalf("%s has no historical native acceptance", platform)
		}
		selected := steps[index]
		if selected.If != selection+" && !inputs.performance" || strings.Contains(selected.Run, "\n") {
			t.Fatalf("%s must select one native lifecycle command: %#v", platform, selected)
		}
		for _, argument := range []string{"mise exec --locked -- go run ./tools/release accept-native", "--peer=github", "--repository=", "--baseline-tag=", "--tag="} {
			if !strings.Contains(selected.Run, argument) {
				t.Fatalf("%s native lifecycle lost %q", platform, argument)
			}
		}
		clients := "--clients=" + map[string]string{"windows": "${{ inputs.windows_clients && inputs.diagnostic_client == '' }}", "darwin": "false", "linux": "false"}[platform]
		for _, input := range []struct {
			name    string
			matches bool
		}{
			{"clients", strings.Contains(selected.Run, clients)},
			{"diagnostic argument", strings.Contains(selected.Run, "--diagnostic-client=")},
			{"baseline", selected.Env["AIGW_BASELINE_TAG"] == "${{ inputs.baseline_tag }}"},
			{"candidate", selected.Env["AIGW_CANDIDATE_TAG"] == "${{ inputs.candidate_tag }}"},
			{"diagnostic", selected.Env["AIGW_NATIVE_DIAGNOSTIC_CLIENT"] == "${{ inputs.diagnostic_client }}"},
		} {
			if !input.matches {
				t.Fatalf("%s native %s input differs from its declaration", platform, input.name)
			}
		}
		for _, kind := range []string{"source", "artifact"} {
			trust := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Prepare native "+kind+" trust" })
			if trust < 0 || trust >= index || steps[trust].If != selection || strings.Contains(steps[trust].Run, "\n") ||
				!strings.Contains(steps[trust].Run, "./tools/ci trust-input") || strings.Contains(steps[trust].Run, "--artifact") != (kind == "artifact") {
				t.Fatalf("%s %s trust must precede acceptance through its original native owner", platform, kind)
			}
		}
		performance := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Measure historical release performance" })
		if performance < 0 || steps[performance].If != selection+" && inputs.performance" ||
			!strings.HasPrefix(steps[performance].Run, "mise run performance ") || strings.Contains(steps[performance].Run, "\n") ||
			!strings.Contains(steps[performance].Run, "--performance") || !strings.Contains(steps[performance].Run, clients) {
			t.Fatalf("%s performance must retain its separate native task and exact input scope", platform)
		}
	}
}

func TestManualPrebuiltPerformanceUsesItsExactToolClosure(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Env map[string]string `yaml:"env"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	const source = "${{ (github.event_name != 'workflow_dispatch' || inputs.full_quality || inputs.refresh_locks || inputs.windows_clients || inputs.candidate_tag == '') && '"
	for _, platform := range []string{"darwin", "linux", "windows"} {
		artifactTools := "go,gh,glab,github:goreleaser/goreleaser"
		if platform == "darwin" {
			artifactTools += ",github:indygreg/apple-platform-rs"
		}
		selected := "' || inputs.performance && '" + artifactTools + ",github:sharkdp/hyperfine' || '" + artifactTools + "' }}"
		tools := workflow.Jobs["native-"+platform].Env["MISE_ENABLE_TOOLS"]
		if !strings.HasPrefix(tools, source) || !strings.HasSuffix(tools, selected) {
			t.Errorf("%s must prepare selected performance tools without widening ordinary artifact acceptance: %s", platform, tools)
		}
	}
}

func TestPerformanceHostPreparesNativeMemoryTool(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				If  string `yaml:"if"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, step := range workflow.Jobs["native-linux"].Steps {
		if step.If == "github.event_name == 'workflow_dispatch' && inputs.performance" &&
			strings.Contains(step.Run, "sudo -n timeout --verbose --kill-after=5s 240s") &&
			strings.Contains(step.Run, "Acquire::http::Timeout=30") &&
			strings.Contains(step.Run, "install --no-install-recommends -y time") {
			return
		}
	}
	t.Fatal("Linux performance must prepare its native GNU time prerequisite")
}

func TestGitLabQualityCarriesProductProvenanceIdentity(t *testing.T) {
	content := `package ci
gitlab: {
	variables: {GIT_DEPTH: "0"}
	quality: {
		variables: {AIGW_RELEASE_AUTHOR_EMAIL: "team@example.invalid"}
	}
	"native-darwin": {}
	"native-linux": {}
	"native-windows": {}
}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	root := projectionRoot(t, content)

	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	gitlab := projections[0].Content
	if !strings.Contains(gitlab, "AIGW_RELEASE_AUTHOR_EMAIL") {
		t.Fatalf("GitLab projection lost product provenance identity:\n%s", gitlab)
	}
}

func TestLinuxArm64GitHubArtifactsDeclareProvenance(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	var pipeline struct {
		Quality struct {
			Variables map[string]string `yaml:"variables"`
		} `yaml:"quality"`
	}
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	enabled := strings.Split(pipeline.Quality.Variables["MISE_ENABLE_TOOLS"], ",")

	content, err := os.ReadFile(filepath.Join(root, "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock miseLock
	if err := toml.Unmarshal(content, &lock); err != nil {
		t.Fatal(err)
	}

	verified := 0
	for _, name := range enabled {
		entries := lock.Tools[name]
		for _, entry := range entries {
			provenance, githubAttestations := entry.LinuxARM64.Provenance.(string)
			if !githubAttestations || provenance != "github-attestations" {
				continue
			}
			verified++
		}
	}
	if verified == 0 {
		t.Fatal("mise.lock contains no Linux arm64 GitHub provenance declarations")
	}
}
