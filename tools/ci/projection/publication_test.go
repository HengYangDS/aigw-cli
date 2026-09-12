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

func TestQualityJobsProjectTheIntegrationCommitBase(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab struct {
		Quality     gitLabJob `yaml:"quality"`
		NativeLinux gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	wantGitLabBases := []string{
		"$CI_COMMIT_SHA^",
		"$CI_MERGE_REQUEST_DIFF_BASE_SHA",
		"$CI_COMMIT_BEFORE_SHA",
	}
	for index, want := range wantGitLabBases {
		if got := gitlab.Quality.Rules[index].Variables["AIGW_COMMIT_BASE"]; got != want {
			t.Fatalf("GitLab commit base rule %d = %q, want %q", index, got, want)
		}
	}
	if _, guessed := gitlab.Quality.Rules[3].Variables["AIGW_COMMIT_BASE"]; guessed {
		t.Fatal("GitLab manual verification must require an explicit AIGW_COMMIT_BASE")
	}
	for index, rule := range gitlab.NativeLinux.Rules {
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

	var release struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[2].Content), &release); err != nil {
		t.Fatal(err)
	}
	releaseSteps := release.Jobs["package-and-publish"].Steps
	releaseIndex := slices.IndexFunc(releaseSteps, func(step struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Verify the signed tag and source"
	})
	if releaseIndex < 0 {
		t.Fatal("GitHub release job lacks signed source verification")
	}
	wantReleaseBase := "${{ format('{0}^', inputs.tag || github.ref_name) }}"
	if got := releaseSteps[releaseIndex].Env["AIGW_COMMIT_BASE"]; got != wantReleaseBase {
		t.Fatalf("GitHub release commit base = %q, want %q", got, wantReleaseBase)
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
	if input, ok := manual.On.WorkflowDispatch.Inputs["commit_base"]; !ok || !input.Required {
		t.Fatalf("GitHub manual verification must require commit_base: %#v", manual.On.WorkflowDispatch.Inputs)
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
		Linux             gitLabJob `yaml:"native-linux"`
		Package           gitLabJob `yaml:"package"`
		Publish           gitLabJob `yaml:"publish"`
		Release           gitLabJob `yaml:"release"`
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
		"native-linux":  gitlab.Linux,
	} {
		protectedPushRuleSeen := false
		for _, rule := range job.Rules {
			if rule.If == `$CI_PIPELINE_SOURCE == "push" && ($CI_COMMIT_BRANCH == "dev" || $CI_COMMIT_BRANCH == "main")` {
				protectedPushRuleSeen = true
			}
		}
		if !protectedPushRuleSeen {
			t.Fatalf("GitLab %s does not admit a maintainer dev push", name)
		}
	}
	for name, job := range map[string]gitLabJob{
		"package": gitlab.Package,
		"publish": gitlab.Publish,
		"release": gitlab.Release,
	} {
		for _, rule := range job.Rules {
			if rule.If == `$CI_PIPELINE_SOURCE == "push" && $CI_COMMIT_BRANCH == "dev"` {
				t.Fatalf("GitLab %s must remain tag-only, got dev rule", name)
			}
		}
	}

	var github struct {
		Jobs map[string]struct {
			If string `yaml:"if"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if got := github.Jobs["accepted-ref-parity"].If; got != "github.event_name == 'push' && github.ref_name == 'main'" {
		t.Fatalf("GitHub accepted parity condition = %q", got)
	}
	for name, job := range github.Jobs {
		if name == "accepted-ref-parity" {
			continue
		}
		if job.If != "github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || github.ref_name == 'dev' || github.ref_name == 'main'" {
			t.Fatalf("GitHub %s does not positively admit the full verification lifecycle: %q", name, job.If)
		}
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
	input, ok := workflow.On.Dispatch.Inputs["baseline_tag"]
	if !ok || input.Required || input.Type != "string" {
		t.Fatalf("historical baseline must be an optional explicit tag: %#v", workflow.On.Dispatch.Inputs)
	}
	clients, ok := workflow.On.Dispatch.Inputs["windows_clients"]
	if !ok || clients.Required || clients.Type != "boolean" {
		t.Fatalf("Windows client qualification requires an explicit opt-in: %#v", clients)
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
		if step.Env["AIGW_QUALIFY_WINDOWS_CLIENTS"] != "${{ inputs.windows_clients }}" {
			t.Fatalf("%s lost the client qualification selection", platform)
		}
		if step.If != "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.windows_clients)" || step.Shell != "pwsh" || step.Env["AIGW_BASELINE_TAG"] != "${{ inputs.baseline_tag }}" {
			t.Fatalf("%s historical acceptance selection = %#v", platform, step)
		}
		if platform != "linux" && step.Env["AIGW_VERIFY_SYSTEM_KEYRING"] != "1" {
			t.Fatalf("%s historical acceptance must exercise its native credential store", platform)
		}
		if platform == "darwin" && step.Env["AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"] != "ephemeral-host" {
			t.Fatal("historical macOS credentials require an ephemeral host")
		}
		if !strings.Contains(step.Run, "$acceptance = @('accept-native')") || !strings.Contains(step.Run, "mise exec --locked -- go run ./tools/release @acceptance") {
			t.Fatalf("%s historical acceptance does not consume the existing package owner", platform)
		}
	}
}

func TestGitHubReleaseBuildUsesTheCanonicalTagInput(t *testing.T) {
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
	if err := yaml.Unmarshal([]byte(projections[2].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["package-and-publish"].Steps
	index := slices.IndexFunc(steps, func(step struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Build the complete release matrix"
	})
	if index < 0 {
		t.Fatal("GitHub release build step is missing")
	}
	want := "${{ inputs.tag || github.ref_name }}"
	if got := steps[index].Env["CI_COMMIT_TAG"]; got != want {
		t.Fatalf("GitHub release build tag = %q, want %q", got, want)
	}
	if _, duplicated := steps[index].Env["SELECTED_TAG"]; duplicated {
		t.Fatal("GitHub release build retains a parallel tag input")
	}
}

func TestReleaseSigningHasExplicitInputsBeforeConstruction(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Environment string            `yaml:"environment"`
			Env         map[string]string `yaml:"env"`
			Permissions map[string]string `yaml:"permissions"`
			Steps       []struct {
				Name string `yaml:"name"`
				Uses string `yaml:"uses"`
				Run  string `yaml:"run"`
				With struct {
					PrivateKey string `yaml:"ssh-private-key"`
				} `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[2].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["package-and-publish"]
	if job.Environment != "release" || job.Env["SSH_ASKPASS_REQUIRE"] != "never" {
		t.Fatal("release signing must use an explicit release environment without password prompts")
	}
	if workflow.Permissions["contents"] != "read" || job.Permissions["contents"] != "write" {
		t.Fatal("release write authority must be scoped to the publication job")
	}
	loaded, tested := false, false
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "webfactory/ssh-agent@") {
			loaded = step.With.PrivateKey == "${{ secrets.AIGW_RELEASE_SIGNING_PRIVATE_KEY }}"
		}
		if strings.Contains(step.Run, "ssh-add -T") {
			tested = loaded
		}
		if step.Name == "Build the complete release matrix" && !tested {
			t.Fatal("artifact construction preceded explicit signing capability verification")
		}
	}
	if !loaded || !tested {
		t.Fatal("release signing capability was not declared and verified")
	}
	var gitlab struct {
		Package gitLabJob `yaml:"package"`
		Publish gitLabJob `yaml:"publish"`
		Release gitLabJob `yaml:"release"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{"package": gitlab.Package, "publish": gitlab.Publish, "release": gitlab.Release} {
		variables := job.Variables
		if variables["AIGW_RELEASE_ALLOWED_SIGNERS_FILE"] != "$AIGW_RELEASE_ALLOWED_SIGNERS" || variables["AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"] != "$AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS" || variables["SSH_ASKPASS_REQUIRE"] != "never" {
			t.Fatalf("GitLab %s omits release trust projection: %v", name, variables)
		}
	}
}

func TestReleaseAdmissionPrecedesConstructionAcrossForges(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	const admission = "mise exec --locked -- go run ./tools/release validate-readiness-tag"
	t.Run("GitLab", func(t *testing.T) {
		var pipeline struct {
			Readiness gitLabJob `yaml:"release-readiness"`
			Package   struct {
				Needs []struct {
					Job      string `yaml:"job"`
					Optional bool   `yaml:"optional"`
				} `yaml:"needs"`
			} `yaml:"package"`
		}
		if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(pipeline.Readiness.Script, []string{admission}) {
			t.Fatalf("release admission command=%q", pipeline.Readiness.Script)
		}
		rules := pipeline.Readiness.Rules
		if len(rules) != 2 || rules[0].If != "$CI_COMMIT_TAG" || rules[0].When != "" || rules[1].When != "never" {
			t.Errorf("release admission must evaluate every tag: %#v", rules)
		}
		for _, need := range pipeline.Package.Needs {
			if need.Job == "release-readiness" && !need.Optional {
				return
			}
		}
		t.Fatal("package must require release admission")
	})
	t.Run("GitHub", func(t *testing.T) {
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Run             string            `yaml:"run"`
					If              string            `yaml:"if"`
					ContinueOnError bool              `yaml:"continue-on-error"`
					Env             map[string]string `yaml:"env"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projections[2].Content), &workflow); err != nil {
			t.Fatal(err)
		}
		admitted := false
		for _, step := range workflow.Jobs["package-and-publish"].Steps {
			if step.Run == admission {
				admitted = step.If == "" && !step.ContinueOnError && step.Env["CI_COMMIT_TAG"] == "${{ inputs.tag || github.ref_name }}"
			}
			if step.Run == "mise exec --locked -- go run ./tools/release build-ci build/release dist" {
				if !admitted {
					t.Fatal("GitHub construction must follow required admission for the selected tag")
				}
				return
			}
		}
		t.Fatal("GitHub release construction is missing")
	})
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

func TestLinuxArm64GitHubArtifactsHaveOfflineProvenanceTrust(t *testing.T) {
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
			if !entry.LinuxARM64.ProvenanceVerified {
				t.Errorf("%s linux-arm64 provenance was discovered but not verified", name)
			}
		}
	}
	if verified == 0 {
		t.Fatal("mise.lock contains no verified Linux arm64 GitHub artifacts")
	}
}
