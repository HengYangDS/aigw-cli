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

func TestGitLabLinuxJobsInheritOneProjectLocalToolchainBootstrap(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		LinuxToolchain      gitLabJob `yaml:".linux-toolchain"`
		SourceToolchain     gitLabJob `yaml:".source-toolchain"`
		SourceAndGovernance gitLabJob `yaml:"source-and-governance"`
		NativeLinux         gitLabJob `yaml:"native-linux"`
		ReleaseReadiness    gitLabJob `yaml:"release-readiness"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	bootstrap := pipeline.LinuxToolchain.BeforeScript
	if len(bootstrap) != 1 {
		t.Fatalf("Linux toolchain bootstrap commands = %d, want 1: %q", len(bootstrap), bootstrap)
	}
	command := bootstrap[0]
	for _, required := range []string{
		`$1 == "min_version"`,
		`$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/ci-mise/$version`,
		`JOB-TOKEN: $CI_JOB_TOKEN`,
		`mise-linux-$arch.tar.gz`,
		`sha256sum --check`,
		`tar --extract --gzip`,
		`install -m 0755`,
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("Linux toolchain bootstrap lacks %q: %q", required, command)
		}
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
			return strings.HasPrefix(command, "mise exec --locked")
		})
		if lockedExecution < 0 || job.BeforeScript != nil || strings.Contains(strings.Join(job.Script, "\n"), "curl ") {
			t.Fatalf("%s does not cleanly inherit the toolchain bootstrap: %#v", name, job)
		}
	}
	if !slices.Equal(pipeline.SourceAndGovernance.Extends, []string{".source-toolchain"}) {
		t.Fatalf("source-and-governance extends = %q, want [.source-toolchain]", pipeline.SourceAndGovernance.Extends)
	}
	if len(pipeline.SourceToolchain.BeforeScript) != 3 {
		t.Fatalf("source toolchain bootstrap commands = %d, want 3: %q", len(pipeline.SourceToolchain.BeforeScript), pipeline.SourceToolchain.BeforeScript)
	}
}

func TestNativeJobsEnableTheirExactCommandToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		NativeDarwin  gitLabJob `yaml:"native-darwin"`
		NativeLinux   gitLabJob `yaml:"native-linux"`
		NativeWindows gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin":  pipeline.NativeDarwin,
		"native-linux":   pipeline.NativeLinux,
		"native-windows": pipeline.NativeWindows,
	} {
		if job.Variables["MISE_ENABLE_TOOLS"] != "go,cue" {
			t.Fatalf("%s enabled tools = %q, want go,cue", name, job.Variables["MISE_ENABLE_TOOLS"])
		}
	}
}

func TestGitLabSourceJobUsesItsExactToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		SourceToolchain     gitLabJob `yaml:".source-toolchain"`
		SourceAndGovernance gitLabJob `yaml:"source-and-governance"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	want := "go,node,npm:@fission-ai/openspec,cue,github:rhysd/actionlint,github:lycheeverse/lychee"
	if got := pipeline.SourceAndGovernance.Variables["MISE_ENABLE_TOOLS"]; got != want {
		t.Fatalf("source-and-governance enabled tools = %q, want %q", got, want)
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
	sourceSetup := pipeline.SourceToolchain.BeforeScript[1]
	if strings.Contains(sourceSetup, "curl ") || strings.Contains(sourceSetup, "tar --extract") {
		t.Fatalf("source tool setup reimplements mise installation: %q", sourceSetup)
	}
}

type gitLabJob struct {
	BeforeScript []string          `yaml:"before_script"`
	Extends      []string          `yaml:"extends"`
	Image        gitLabImage       `yaml:"image"`
	Script       []string          `yaml:"script"`
	Variables    map[string]string `yaml:"variables"`
}

type gitLabImage struct {
	Name       string   `yaml:"name"`
	Entrypoint []string `yaml:"entrypoint"`
}

func TestGitLabForgeContextIsScopedToSourceGovernance(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
gitlab: {
	variables: {GIT_DEPTH: "0"}
	"source-and-governance": {
		variables: AIGW_FORGE_PROVIDER: "gitlab"
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
	if got := strings.Count(gitlab, "AIGW_FORGE_PROVIDER: gitlab"); got != 1 {
		t.Fatalf("GitLab Forge context occurrences = %d, want 1:\n%s", got, gitlab)
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
