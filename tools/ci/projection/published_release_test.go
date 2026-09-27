package projection

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

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
	if !present || len(workflow.Jobs) != 1 || workflow.Permissions["contents"] != "read" || workflow.Permissions["actions"] != "read" || len(job.Permissions) != 0 {
		t.Fatal("published artifact verification must have read-only authority")
	}
	if len(workflow.On) != 1 || !workflow.On["workflow_dispatch"].Inputs["tag"].Required {
		t.Fatal("artifact verification requires explicit dispatch after complete publication")
	}

	const tag = "${{ inputs.tag }}"
	if job.Env["CI_COMMIT_TAG"] != tag || job.Steps[0].With["ref"] != "${{ github.event.pull_request.head.sha || github.sha }}" {
		t.Fatal("release artifact identity and verifier revision must remain separate")
	}
	if job.Env["AIGW_RELEASE_ARTIFACT_SIGNER"] != "${{ vars.AIGW_RELEASE_ARTIFACT_SIGNER }}" ||
		job.Env["MISE_ENABLE_TOOLS"] != "${{ inputs.native_lifecycle && (startsWith(inputs.runner, 'macos-') && 'go,gh,node,github:goreleaser/goreleaser,github:indygreg/apple-platform-rs' || 'go,gh,node,github:goreleaser/goreleaser') || 'go,gh' }}" {
		t.Fatal("release signer trust or lifecycle tool closure is absent")
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
		"mise exec --locked -- go run ./tools/release validate-version-tag",
		`mise exec --locked -- go run ./tools/ci trust-input --output "$env:RUNNER_TEMP/aigw-allowed-signers" --github-env "$env:GITHUB_ENV"`,
		`mise exec --locked -- go run ./tools/ci trust-input --artifact --output "$env:RUNNER_TEMP/aigw-artifact-signers" --github-env "$env:GITHUB_ENV"`,
		`mise exec --locked -- gh release download "$env:CI_COMMIT_TAG" --repo "$env:GITHUB_REPOSITORY" --dir dist`,
		"mise exec --locked -- go run ./tools/release verify-artifacts dist",
		"mise exec --locked -- go run ./tools/release accept-native --artifacts dist",
	}
	if len(commands) != len(want)+1 || !slices.Equal(commands[:3], want[:3]) || !slices.Equal(commands[4:], want[3:]) || !slices.Equal(trust, []string{"AIGW_RELEASE_ALLOWED_SIGNERS", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"}) {
		t.Fatalf("release verification order or trust differs: %q, %q", commands, trust)
	}
	evidence := commands[3]
	for _, required := range []string{
		"git rev-parse", "release-evidence", "--repository", "--workflow verify.yml",
		"--tag", "--sha", "--job 'Quality and governance'", "--job 'Native macOS acceptance'",
		"--job 'Native Linux acceptance'", "--job 'Native Windows acceptance'",
		"--job 'Release version'",
	} {
		if !strings.Contains(evidence, required) {
			t.Fatalf("release evidence command lacks %q: %q", required, evidence)
		}
	}
}

func TestPublishedNativeLifecycleUsesSelectedIsolatedRunner(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On map[string]struct {
			Inputs map[string]struct {
				Type    string
				Default string
				Options []string
			}
		} `yaml:"on"`
		Concurrency struct{ Group string }
		Jobs        map[string]struct {
			Runner   string `yaml:"runs-on"`
			Defaults struct{ Run struct{ Shell string } }
			Steps    []struct {
				If  string            `yaml:"if"`
				Env map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[2].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["release-assets"]
	inputs := workflow.On["workflow_dispatch"].Inputs
	runner := inputs["runner"]
	if runner.Type != "choice" || runner.Default != "ubuntu-24.04" ||
		!slices.Equal(runner.Options, []string{"ubuntu-24.04", "ubuntu-24.04-arm", "macos-26-intel", "macos-15-intel", "windows-2025", "windows-11-arm"}) {
		t.Fatalf("release verification runner choices = %#v", runner)
	}
	if inputs["native_lifecycle"].Type != "boolean" || inputs["native_lifecycle"].Default != "false" {
		t.Fatal("published native lifecycle must be an explicit optional verification")
	}
	if job.Runner != "${{ inputs.runner }}" || job.Defaults.Run.Shell != "pwsh" {
		t.Fatal("release verification must use the selected native runner and a portable shell")
	}
	if workflow.Concurrency.Group != "release-${{ github.repository }}-${{ inputs.tag }}-${{ inputs.runner }}" {
		t.Fatal("different native release targets must be able to run independently")
	}
	native := job.Steps[len(job.Steps)-1]
	if native.If != "inputs.native_lifecycle" || native.Env["AIGW_VERIFY_SYSTEM_KEYRING"] != "${{ runner.os == 'Windows' && '1' || '0' }}" || native.Env["AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"] != "ephemeral-host" {
		t.Fatal("native lifecycle must retain explicit isolated credential-store admission")
	}
}

func TestGitLabPublishedAssetsUsePeerLocalDownloadAndVerification(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Version gitLabJob `yaml:"release-version"`
		Assets  gitLabJob `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pipeline.Version.Script, []string{"env GODEBUG=http2client=0 mise install --locked", "mise exec --locked -- go run ./tools/release validate-version-tag"}) {
		t.Fatal("tag admission is missing")
	}
	want := []string{"env GODEBUG=http2client=0 mise install --locked", "mkdir dist", `mise exec --locked -- glab release download "$CI_COMMIT_TAG" --repo "$CI_PROJECT_URL" --asset-name 'aigw_*' --asset-name 'checksums.txt*' --dir dist`, "mise exec --locked -- go run ./tools/release verify-artifacts dist"}
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
	if !slices.Equal(needs, []string{"quality", "native-darwin", "native-linux", "native-windows", "linux-secret-service", "release-version"}) {
		t.Fatalf("release requirements = %q", needs)
	}
	if len(pipeline.Assets.Rules) != 2 || pipeline.Assets.Rules[0].If != `$CI_COMMIT_TAG && ($CI_PIPELINE_SOURCE == "api" || $CI_PIPELINE_SOURCE == "web")` || pipeline.Assets.Rules[1].When != "never" {
		t.Fatal("asset verification must follow explicit post-publication dispatch")
	}
}
