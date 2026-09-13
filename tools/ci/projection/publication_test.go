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
		"release-assets": gitlab.ReleaseAssets,
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
		if job.If != "github.ref_type == 'tag' || github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || github.ref_name == 'dev' || github.ref_name == 'main'" {
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
	for _, name := range []string{"baseline_tag", "candidate_tag"} {
		input, ok := workflow.On.Dispatch.Inputs[name]
		if !ok || input.Required || input.Type != "string" {
			t.Fatalf("%s must be an optional explicit release tag: %#v", name, workflow.On.Dispatch.Inputs)
		}
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
		if step.If != "github.event_name == 'workflow_dispatch' && (inputs.baseline_tag != '' || inputs.candidate_tag != '' || inputs.windows_clients)" || step.Shell != "pwsh" || step.Env["AIGW_BASELINE_TAG"] != "${{ inputs.baseline_tag }}" {
			t.Fatalf("%s historical acceptance selection = %#v", platform, step)
		}
		if step.Env["AIGW_CANDIDATE_TAG"] != "${{ inputs.candidate_tag }}" || !strings.Contains(step.Run, "$acceptance += @('--artifacts', $candidate)") {
			t.Fatalf("%s cannot consume the published candidate", platform)
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

func TestPublishedArtifactVerificationUsesExactTagAndPublicTrust(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On map[string]struct {
			Inputs map[string]struct{ Required bool }
		} `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Permissions map[string]string `yaml:"permissions"`
			Env         map[string]string `yaml:"env"`
			Steps       []struct {
				Name string            `yaml:"name"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[2].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	job, present := workflow.Jobs["release-assets"]
	if !present || len(workflow.Jobs) != 1 || workflow.Permissions["contents"] != "read" || len(job.Permissions) != 0 {
		t.Fatal("published artifact verification must have read-only authority")
	}
	if len(workflow.On) != 1 || !workflow.On["workflow_dispatch"].Inputs["tag"].Required {
		t.Fatal("artifact verification requires explicit dispatch after complete publication")
	}
	const tag = "${{ inputs.tag }}"
	if job.Env["CI_COMMIT_TAG"] != tag || job.Steps[0].With["ref"] != tag {
		t.Fatal("release verification changed the selected tag")
	}
	if job.Env["AIGW_RELEASE_ARTIFACT_SIGNER"] != "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}" {
		t.Fatal("release signer trust is absent")
	}
	var commands []string
	var trust []string
	for _, step := range job.Steps {
		if step.Run != "" {
			commands = append(commands, step.Run)
		}
		for _, key := range []string{"AIGW_RELEASE_ALLOWED_SIGNERS", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"} {
			if step.Env[key] == "${{ vars."+key+" }}" {
				trust = append(trust, key)
			}
		}
	}
	want := []string{
		"mise exec --locked -- go run ./tools/release validate-readiness-tag",
		`mise exec --locked -- go run ./tools/ci trust-input --output "$RUNNER_TEMP/aigw-allowed-signers" --github-env "$GITHUB_ENV"`,
		`mise exec --locked -- go run ./tools/ci trust-input --artifact --output "$RUNNER_TEMP/aigw-artifact-signers" --github-env "$GITHUB_ENV"`,
		`mise exec --locked -- gh release download "$CI_COMMIT_TAG" --repo "$GITHUB_REPOSITORY" --dir dist`,
		"mise exec --locked -- go run ./tools/release verify-artifacts dist",
	}
	if !slices.Equal(commands, want) || !slices.Equal(trust, []string{"AIGW_RELEASE_ALLOWED_SIGNERS", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"}) {
		t.Fatalf("release verification order or trust differs: %q, %q", commands, trust)
	}
}

func TestGitLabPublishedAssetsUsePeerLocalDownloadAndVerification(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Readiness gitLabJob `yaml:"release-readiness"`
		Assets    gitLabJob `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pipeline.Readiness.Script, []string{"mise exec --locked -- go run ./tools/release validate-readiness-tag"}) {
		t.Fatal("tag admission is missing")
	}
	want := []string{"mkdir dist", `mise exec --locked -- glab release download "$CI_COMMIT_TAG" --repo "$CI_PROJECT_URL" --asset-name 'aigw_*' --asset-name 'checksums.txt*' --dir dist`, "mise exec --locked -- go run ./tools/release verify-artifacts dist"}
	if !slices.Equal(pipeline.Assets.Script, want) {
		t.Fatalf("GitLab asset verification = %q", pipeline.Assets.Script)
	}
	variables := pipeline.Assets.Variables
	if variables["AIGW_RELEASE_ALLOWED_SIGNERS_FILE"] != "$AIGW_RELEASE_ALLOWED_SIGNERS" || variables["AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"] != "$AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS" || variables["GLAB_ENABLE_CI_AUTOLOGIN"] != "true" {
		t.Fatal("GitLab asset verification lacks native job authentication or public trust")
	}
	var needs []string
	for _, need := range pipeline.Assets.Needs {
		needs = append(needs, need.Job)
	}
	if !slices.Equal(needs, []string{"quality", "native-darwin", "native-linux", "release-readiness"}) {
		t.Fatalf("release requirements = %q", needs)
	}
	if len(pipeline.Assets.Rules) != 2 || pipeline.Assets.Rules[0].If != `$CI_COMMIT_TAG && ($CI_PIPELINE_SOURCE == "api" || $CI_PIPELINE_SOURCE == "web")` || pipeline.Assets.Rules[1].When != "never" {
		t.Fatal("asset verification must follow explicit post-publication dispatch")
	}
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
