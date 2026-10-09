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
		"--job 'Native Linux acceptance'", "--job 'Native Windows acceptance'", "--job 'Linux Secret Service'",
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
		Version   gitLabJob `yaml:"release-version"`
		Assets    gitLabJob `yaml:"release-assets"`
		Toolchain struct {
			AfterScript []string `yaml:"after_script"`
		} `yaml:".linux-toolchain"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if len(pipeline.Version.Script) != 4 || len(pipeline.Version.AfterScript) != 1 ||
		!strings.Contains(pipeline.Version.Script[0], "MISE_NETRC_FILE") ||
		!slices.Equal(pipeline.Version.Script[1:3], []string{"env GODEBUG=http2client=0 mise install --locked", "mise exec --locked -- go run ./tools/release validate-version-tag"}) ||
		pipeline.Version.Script[3] != pipeline.Version.AfterScript[0] {
		t.Fatal("tag admission is missing")
	}
	want := []string{"mkdir dist", `mise exec --locked -- glab release download "$CI_COMMIT_TAG" --repo "$CI_PROJECT_URL" --asset-name 'aigw_*' --asset-name 'checksums.txt*' --dir dist`, "mise exec --locked -- go run ./tools/release verify-artifacts dist"}
	if len(pipeline.Toolchain.AfterScript) != 1 || !strings.Contains(pipeline.Toolchain.AfterScript[0], "Mise job-owned supply state retired.") {
		t.Fatal("published asset acceptance requires exact job-owned cleanup")
	}
	want = append(want, pipeline.Toolchain.AfterScript[0])
	if !slices.Equal(pipeline.Assets.Extends, []string{".linux-toolchain"}) || !slices.Equal(pipeline.Assets.Script, want) {
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
	if len(pipeline.Assets.Rules) != 2 || pipeline.Assets.Rules[0].If != `$CI_COMMIT_TAG && ($CI_PIPELINE_SOURCE == "api" || $CI_PIPELINE_SOURCE == "web") && $CI_COMMIT_REF_PROTECTED == "true"` || pipeline.Assets.Rules[1].When != "never" {
		t.Fatal("asset verification must follow explicit post-publication dispatch")
	}
}

func TestGitHubNativeInputReleaseProjectsOneOwnedTransport(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type     string
					Required bool
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			If    string
			Steps []struct {
				Name, Run, If string
				Env           map[string]string
			}
		}
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"input_release", "input_sha256", "candidate_source"} {
		input, exists := workflow.On.Dispatch.Inputs[key]
		if !exists || input.Type != "string" || input.Required {
			t.Fatalf("native input selector %s must be optional and typed", key)
		}
	}
	for _, name := range []string{"quality", "linux-secret-service"} {
		if !strings.Contains(workflow.Jobs[name].If, "inputs.input_release == ''") {
			t.Fatalf("%s repeats source-only gates for prebuilt inputs", name)
		}
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		steps := workflow.Jobs["native-"+platform].Steps
		index := slices.IndexFunc(steps, func(step struct {
			Name, Run, If string
			Env           map[string]string
		}) bool {
			return step.Name == "Run historical release acceptance"
		})
		if index < 0 {
			t.Fatalf("%s lacks the existing native release owner", platform)
		}
		step := steps[index]
		for key, input := range map[string]string{"AIGW_NATIVE_INPUT_RELEASE": "input_release", "AIGW_NATIVE_INPUT_SHA256": "input_sha256", "AIGW_CANDIDATE_SOURCE": "candidate_source"} {
			if step.Env[key] != "${{ inputs."+input+" }}" || strings.Contains(step.Run, "${{ inputs."+input+" }}") {
				t.Fatalf("%s input %s must flow through environment, not shell source", platform, input)
			}
		}
		for _, flag := range []string{"--input-release=", "--input-sha256=", "--candidate-source=", "--candidate=", "--baseline-tag="} {
			if !strings.Contains(step.Run, flag) {
				t.Fatalf("%s lost original native input flag %s", platform, flag)
			}
		}
		ordinary := slices.IndexFunc(steps, func(step struct {
			Name, Run, If string
			Env           map[string]string
		}) bool {
			return step.Name == "Run native macOS acceptance" || step.Name == "Run native Linux acceptance" || step.Name == "Run native Windows acceptance"
		})
		if ordinary < 0 || !strings.Contains(steps[ordinary].If, "inputs.input_release == ''") {
			t.Fatalf("%s repeats source acceptance for prebuilt native inputs", platform)
		}
		if platform == "darwin" && (step.Env["AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"] != "ephemeral-host" || !strings.Contains(step.Env["AIGW_VERIFY_SYSTEM_KEYRING"], "inputs.macos_keychain")) {
			t.Fatal("native macOS input selection lost its explicit disposable-host Keychain contract")
		}
		if !strings.Contains(step.If, "inputs.input_release") {
			t.Fatalf("%s native inputs cannot select the existing acceptance job", platform)
		}
	}
}
