package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestProjectionReconciliationWritesAndChecksExactContent(t *testing.T) {
	root := t.TempDir()
	projections := []projection{
		{Path: ".gitlab-ci.yml", Content: "stages: [verify]\n"},
		{Path: ".github/workflows/verify.yml", Content: "name: Verify\n"},
	}

	if err := reconcileProjections(root, projections, true); err != nil {
		t.Fatal(err)
	}
	if err := reconcileProjections(root, projections, false); err != nil {
		t.Fatalf("fresh projections drifted: %v", err)
	}

	drifted := filepath.Join(root, ".gitlab-ci.yml")
	if err := os.WriteFile(drifted, []byte("manual: edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileProjections(root, projections, false); err == nil || !strings.Contains(err.Error(), ".gitlab-ci.yml") {
		t.Fatalf("projection drift error = %v", err)
	}
}

func TestProjectionReconciliationRejectsInvalidManifestPaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"", "/tmp/workflow.yml", "../workflow.yml", ".github/../workflow.yml"} {
		err := reconcileProjections(root, []projection{{Path: path, Content: "content\n"}}, true)
		if err == nil {
			t.Fatalf("accepted projection path %q", path)
		}
	}
}

func TestProjectionPathUsesRepositorySeparators(t *testing.T) {
	root := t.TempDir()
	path, err := projectionPath(root, ".github/workflows/verify.yml")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".github", "workflows", "verify.yml")
	if path != want {
		t.Fatalf("projection path = %q, want %q", path, want)
	}
}

func TestProjectionPathRejectsBackslashTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := projectionPath(root, `..\workflow.yml`); err == nil {
		t.Fatal("accepted backslash traversal")
	}
}

func TestProjectCommandRendersAndChecksTheTrackedProjections(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
gitlab: {stages: ["verify"]}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"project", "--root", root}, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"project", "--check", "--root", root}, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("fresh projection check: %v", err)
	}
	for _, item := range projectionExpressions {
		path := filepath.Join(root, item.path)
		if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
			t.Fatalf("projection %s: data=%q error=%v", item.path, data, err)
		}
	}
}

func TestProjectionCommandUsesRepositoryRelativeModel(t *testing.T) {
	root := t.TempDir()
	process := projectionCommand(root, "gitlab")
	if process.Dir != root {
		t.Fatalf("projection command directory = %q, want %q", process.Dir, root)
	}
	want := []string{"cue", "export", filepath.FromSlash(".config/ci/pipeline.cue"), "--expression", "gitlab", "--out", "yaml"}
	if !reflect.DeepEqual(process.Args, want) {
		t.Fatalf("projection command = %#v, want %#v", process.Args, want)
	}
}

func TestVerificationRoutingCoversReviewAndMaintainerPaths(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab struct {
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
		{If: `$CI_PIPELINE_SOURCE == "merge_request_event" && $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "dev"`},
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
			t.Fatalf("GitLab verification route %d = %#v, want %#v", index, got, want)
		}
	}

	var github struct {
		On struct {
			Push struct {
				Branches []string `yaml:"branches"`
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
	if !slices.Equal(github.On.Push.Branches, []string{"dev", "main"}) {
		t.Fatalf("GitHub protected-branch routes = %q, want [dev main]", github.On.Push.Branches)
	}
	if !slices.Equal(github.On.PullRequest.Branches, []string{"dev"}) {
		t.Fatalf("GitHub review targets = %q, want [dev]", github.On.PullRequest.Branches)
	}
	if github.On.WorkflowDispatch == nil {
		t.Fatal("GitHub verification lacks the explicit maintainer dispatch route")
	}
}

func TestAcceptedPublicationChecksRefParityFromMain(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var gitlab struct {
		AcceptedRefParity gitLabJob `yaml:"accepted-ref-parity"`
		Source            gitLabJob `yaml:"source-and-governance"`
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
		"source-and-governance": gitlab.Source,
		"native-darwin":         gitlab.Darwin,
		"native-linux":          gitlab.Linux,
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

func TestGitLabToolchainImagesYieldToTheRunnerShell(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
#ToolchainImage: {
	name: "example.invalid/toolchain@sha256:fixture"
	entrypoint: [""]
}
gitlab: {
	".linux-toolchain": {image: #ToolchainImage}
	"source-and-governance": {extends: [".linux-toolchain"]}
	"native-linux": {extends: [".linux-toolchain"]}
}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	gitlab := projections[0].Content
	if got := strings.Count(gitlab, "entrypoint:\n      - \"\""); got != 1 {
		t.Fatalf("empty GitLab image entrypoints = %d, want 1:\n%s", got, gitlab)
	}
}

func TestGitLabLinuxJobsUseOneLockedToolchainImage(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Variables           map[string]string `yaml:"variables"`
		LinuxToolchain      gitLabJob         `yaml:".linux-toolchain"`
		SourceToolchain     gitLabJob         `yaml:".source-toolchain"`
		SourceAndGovernance gitLabJob         `yaml:"source-and-governance"`
		NativeLinux         gitLabJob         `yaml:"native-linux"`
		ReleaseReadiness    gitLabJob         `yaml:"release-readiness"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if got := pipeline.Variables["MISE_CONFIG_DIR"]; got != "$CI_PROJECT_DIR/.config/ci" {
		t.Fatalf("GitLab mise config directory = %q, want repository-owned config directory", got)
	}
	if got := pipeline.Variables["MISE_GLOBAL_CONFIG_FILE"]; got != "" {
		t.Fatalf("GitLab must not project a missing global config file: %q", got)
	}
	bootstrap := pipeline.LinuxToolchain.BeforeScript
	if len(bootstrap) != 0 {
		t.Fatalf("Linux jobs must use the exact locked mise image without a second bootstrap: %q", bootstrap)
	}
	if pipeline.LinuxToolchain.Image.Name == "" || pipeline.LinuxToolchain.Image.Entrypoint == nil {
		t.Fatalf("Linux toolchain image is incomplete: %#v", pipeline.LinuxToolchain.Image)
	}
	for name, job := range map[string]gitLabJob{
		"native-linux":      pipeline.NativeLinux,
		"release-readiness": pipeline.ReleaseReadiness,
	} {
		if !slices.Equal(job.Extends, []string{".linux-toolchain"}) {
			t.Fatalf("%s extends = %q, want [.linux-toolchain]", name, job.Extends)
		}
		lockedExecution := slices.IndexFunc(job.Script, func(command string) bool {
			return strings.Contains(command, "mise exec --locked")
		})
		if lockedExecution < 0 || job.BeforeScript != nil || strings.Contains(strings.Join(job.Script, "\n"), "curl ") {
			t.Fatalf("%s does not cleanly inherit the toolchain bootstrap: %#v", name, job)
		}
	}
	if !slices.Equal(pipeline.SourceAndGovernance.Extends, []string{".source-toolchain"}) {
		t.Fatalf("source-and-governance extends = %q, want [.source-toolchain]", pipeline.SourceAndGovernance.Extends)
	}
	if len(pipeline.SourceToolchain.BeforeScript) != 2 {
		t.Fatalf("source toolchain bootstrap commands = %d, want 2: %q", len(pipeline.SourceToolchain.BeforeScript), pipeline.SourceToolchain.BeforeScript)
	}
}

func TestGitHubWorkflowsUseOnlyRepositoryMiseConfiguration(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
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
		if got := workflow.Env["MISE_CONFIG_DIR"]; got != "${{ github.workspace }}/.config/ci" {
			t.Fatalf("GitHub projection %d mise config directory = %q, want repository-owned config directory", index, got)
		}
		if got := workflow.Env["MISE_GLOBAL_CONFIG_FILE"]; got != "" {
			t.Fatalf("GitHub projection %d must not project a global config file: %q", index, got)
		}
	}
}

func TestNativeJobsEnableTheirExactCommandToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		NativeDarwin gitLabJob `yaml:"native-darwin"`
		NativeLinux  gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin": pipeline.NativeDarwin,
		"native-linux":  pipeline.NativeLinux,
	} {
		if job.Variables["MISE_ENABLE_TOOLS"] != "go,cue" {
			t.Fatalf("%s enabled tools = %q, want go,cue", name, job.Variables["MISE_ENABLE_TOOLS"])
		}
	}
}

func TestGitHubLinuxSecretServiceInstallationKeepsBothPackageCommandsPrivileged(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
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
	steps := workflow.Jobs["native-linux"].Steps
	index := slices.IndexFunc(steps, func(step struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
	}) bool {
		return step.Name == "Install Secret Service"
	})
	if index < 0 {
		t.Fatal("GitHub native Linux job lacks Secret Service installation")
	}
	command := steps[index].Run
	if !strings.HasPrefix(command, "sudo -- sh -c '") {
		t.Fatalf("Secret Service installation does not elevate the complete transaction: %q", command)
	}
	for _, operation := range []string{"apt-get update", "DEBIAN_FRONTEND=noninteractive apt-get install"} {
		if !strings.Contains(command, operation) {
			t.Fatalf("Secret Service installation lacks %q: %q", operation, command)
		}
	}
}

func TestGitHubDarwinSystemCredentialJourneyRequiresAnEphemeralHost(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
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

func TestSourceJobsUseTheirExactToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	const sourceTools = "go,node,cue,github:editorconfig-checker/editorconfig-checker,github:gitleaks/gitleaks,github:rhysd/actionlint,github:lycheeverse/lychee"
	var pipeline struct {
		SourceToolchain     gitLabJob `yaml:".source-toolchain"`
		SourceAndGovernance gitLabJob `yaml:"source-and-governance"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if got := pipeline.SourceAndGovernance.Variables["MISE_ENABLE_TOOLS"]; got != sourceTools {
		t.Fatalf("GitLab source tools = %q, want %q", got, sourceTools)
	}
	if !slices.Contains(pipeline.SourceAndGovernance.Script, "npm ci --ignore-scripts") {
		t.Fatalf("GitLab source job lacks locked npm installation: %q", pipeline.SourceAndGovernance.Script)
	}
	bootstrap := strings.Join(pipeline.SourceToolchain.BeforeScript, "\n")
	for _, tool := range []string{
		"mise.lock",
		"ci-source-tools/$lock_sha/\\$4",
		"MISE_URL_REPLACEMENTS",
		"gitlab-ci-token:$CI_JOB_TOKEN",
	} {
		if !strings.Contains(bootstrap, tool) {
			t.Fatalf("source-and-governance bootstrap lacks %q: %q", tool, bootstrap)
		}
	}
	sourceMirror := pipeline.SourceToolchain.BeforeScript[0]
	if strings.Contains(sourceMirror, "curl ") || strings.Contains(sourceMirror, "tar --extract") {
		t.Fatalf("source tool mirror reimplements mise installation: %q", sourceMirror)
	}

	var github struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if got := github.Jobs["source-and-governance"].Env["MISE_ENABLE_TOOLS"]; got != sourceTools {
		t.Fatalf("GitHub source tools = %q, want %q", got, sourceTools)
	}
	steps := github.Jobs["source-and-governance"].Steps
	if !slices.ContainsFunc(steps, func(step struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
	}) bool {
		return step.Name == "Install locked npm tools" && step.Run == "npm ci --ignore-scripts"
	}) {
		t.Fatalf("GitHub source job lacks locked npm installation: %#v", steps)
	}
}

func TestGitHubVerificationChecksOutTheExactProductCommit(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
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
	root := filepath.Clean(filepath.Join("..", ".."))
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

func TestGitHubReleaseBuildUsesTheCanonicalTagInput(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
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

type gitLabJob struct {
	BeforeScript []string          `yaml:"before_script"`
	Extends      []string          `yaml:"extends"`
	Image        gitLabImage       `yaml:"image"`
	Needs        []gitLabNeed      `yaml:"needs"`
	Script       []string          `yaml:"script"`
	Variables    map[string]string `yaml:"variables"`
	Rules        []struct {
		If   string `yaml:"if"`
		When string `yaml:"when"`
	} `yaml:"rules"`
}

type gitLabNeed struct {
	Job string `yaml:"job"`
}

type gitLabImage struct {
	Name       string   `yaml:"name"`
	Entrypoint []string `yaml:"entrypoint"`
}

func TestForgeProjectionsFollowDeclaredNativeCapacity(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeWindows *gitLabJob `yaml:"native-windows"`
		Package       gitLabJob  `yaml:"package"`
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
	for _, need := range gitlab.Package.Needs {
		if need.Job == "native-windows" {
			t.Fatal("GitLab package duplicates product-level native Windows admission")
		}
	}

	for _, projectionIndex := range []int{1, 2} {
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

func TestGitLabSourceGovernanceCarriesProductProvenanceIdentity(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
gitlab: {
	variables: {GIT_DEPTH: "0"}
	"source-and-governance": {
		variables: {AIGW_RELEASE_AUTHOR_EMAIL: "team@example.invalid"}
	}
	"native-darwin": {}
	"native-linux": {}
	"native-windows": {}
}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	gitlab := projections[0].Content
	if !strings.Contains(gitlab, "AIGW_RELEASE_AUTHOR_EMAIL") {
		t.Fatalf("GitLab projection lost product provenance identity:\n%s", gitlab)
	}
}

func TestProjectionRenderingReportsModelAndExecutableFailures(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte("not valid CUE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderProjections(root); err == nil || !strings.Contains(err.Error(), "render .gitlab-ci.yml") {
		t.Fatalf("invalid model error = %v", err)
	}
	if err := run([]string{"project", "--root", root}, &bytes.Buffer{}, nil); err == nil || !strings.Contains(err.Error(), "render .gitlab-ci.yml") {
		t.Fatalf("project command model error = %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := renderProjections(root); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("missing CUE error = %v", err)
	}
}

func TestProjectionReconciliationReportsReadAndWriteFailures(t *testing.T) {
	root := t.TempDir()
	missing := []projection{{Path: ".gitlab-ci.yml", Content: "content\n"}}
	if err := reconcileProjections(root, missing, false); err == nil || !strings.Contains(err.Error(), "read projection") {
		t.Fatalf("missing projection error = %v", err)
	}

	blockedRoot := filepath.Join(root, "blocked")
	if err := os.WriteFile(blockedRoot, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileProjections(blockedRoot, missing, true); err == nil || !strings.Contains(err.Error(), "create projection directory") {
		t.Fatalf("directory creation error = %v", err)
	}

	directoryTarget := filepath.Join(root, "directory.yml")
	if err := os.Mkdir(directoryTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeProjection(directoryTarget, []byte("content\n")); err == nil || !strings.Contains(err.Error(), "write projection") {
		t.Fatalf("projection write error = %v", err)
	}
}
