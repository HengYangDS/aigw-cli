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
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Required bool   `yaml:"required"`
					Type     string `yaml:"type"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Name  string            `yaml:"name"`
				If    string            `yaml:"if"`
				Shell string            `yaml:"shell"`
				Env   map[string]string `yaml:"env"`
				Run   string            `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"baseline_tag", "candidate_tag"} {
		input, ok := workflow.On.Dispatch.Inputs[name]
		if !ok || input.Required || input.Type != "string" {
			t.Fatalf("%s must be an optional explicit release tag: %#v", name, workflow.On.Dispatch.Inputs)
		}
	}
	for _, name := range []string{"windows_clients", "performance"} {
		input, ok := workflow.On.Dispatch.Inputs[name]
		if !ok || input.Required || input.Type != "boolean" {
			t.Fatalf("%s qualification requires an explicit opt-in: %#v", name, input)
		}
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		job := workflow.Jobs["native-"+platform]
		index := slices.IndexFunc(job.Steps, func(step struct {
			Name  string            `yaml:"name"`
			If    string            `yaml:"if"`
			Shell string            `yaml:"shell"`
			Env   map[string]string `yaml:"env"`
			Run   string            `yaml:"run"`
		}) bool {
			return step.Name == "Run historical release acceptance"
		})
		if index < 0 {
			t.Fatalf("%s has no historical native acceptance", platform)
		}
		step := job.Steps[index]
		for key, value := range map[string]string{
			"AIGW_QUALIFY_WINDOWS_CLIENTS": "${{ inputs.windows_clients }}",
			"AIGW_MEASURE_PERFORMANCE":     "${{ inputs.performance }}",
			"AIGW_BASELINE_TAG":            "${{ inputs.baseline_tag }}",
			"AIGW_CANDIDATE_TAG":           "${{ inputs.candidate_tag }}",
		} {
			if step.Env[key] != value {
				t.Fatalf("%s lost selection %s", platform, key)
			}
		}
		if step.If != "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.candidate_tag != '' || inputs.windows_clients || inputs.macos_keychain || inputs.performance)" || step.Shell != "pwsh" {
			t.Fatalf("%s historical acceptance selection = %#v", platform, step)
		}
		if !strings.Contains(step.Run, "$acceptance += @('--artifacts', $candidate)") {
			t.Fatalf("%s cannot consume the published candidate", platform)
		}
		wantKeyring := map[string]string{
			"darwin":  "${{ github.event_name == 'workflow_dispatch' && inputs.macos_keychain && '1' || '0' }}",
			"windows": "1",
		}[platform]
		if step.Env["AIGW_VERIFY_SYSTEM_KEYRING"] != wantKeyring {
			t.Fatalf("%s historical acceptance does not match its credential qualification boundary", platform)
		}
		if platform == "darwin" && step.Env["AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"] != "ephemeral-host" {
			t.Fatal("historical macOS credentials require an ephemeral host")
		}
		if !strings.Contains(step.Run, "TestNativePublishedPredecessorJourney/published_keychain") {
			t.Fatal("macOS Keychain qualification must select the published predecessor journey")
		}
		if !strings.Contains(step.Run, "$acceptance = @('accept-native')") || !strings.Contains(step.Run, "mise exec --locked -- go run ./tools/release @acceptance") {
			t.Fatalf("%s historical acceptance does not consume the existing package owner", platform)
		}
		if !strings.Contains(step.Run, "mise run performance @performance") {
			t.Fatalf("%s performance must use the task-specific locked tool", platform)
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
