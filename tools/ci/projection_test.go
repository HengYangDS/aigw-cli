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
	"source-and-governance": {image: #ToolchainImage}
	"native-linux": {image: #ToolchainImage}
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
	if got := strings.Count(gitlab, "entrypoint:\n      - \"\""); got != 2 {
		t.Fatalf("empty GitLab image entrypoints = %d, want 2:\n%s", got, gitlab)
	}
}

func TestGitLabContainerJobsBootstrapRepositoryMiseBeforeLockedExecution(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		SourceAndGovernance gitLabJob `yaml:"source-and-governance"`
		NativeLinux         gitLabJob `yaml:"native-linux"`
		ReleaseReadiness    gitLabJob `yaml:"release-readiness"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"source-and-governance": pipeline.SourceAndGovernance,
		"native-linux":          pipeline.NativeLinux,
		"release-readiness":     pipeline.ReleaseReadiness,
	} {
		lockedExecution := slices.IndexFunc(job.Script, func(command string) bool {
			return strings.HasPrefix(command, "mise exec --locked")
		})
		bootstrap := job.Script[0]
		if len(job.Script) < 2 ||
			!strings.Contains(bootstrap, `$1 == "min_version"`) ||
			!strings.Contains(bootstrap, `mise-v${version}-linux-${arch}.tar.gz`) ||
			!strings.Contains(bootstrap, `SHASUMS256.txt`) ||
			!strings.Contains(bootstrap, `sed -n`) ||
			!strings.Contains(bootstrap, `sha256sum --check`) ||
			!strings.Contains(bootstrap, `--connect-timeout 10`) ||
			!strings.Contains(bootstrap, `--max-time 120`) ||
			!strings.Contains(bootstrap, `--continue-at -`) ||
			!strings.Contains(bootstrap, `tar --extract --gzip`) ||
			!strings.Contains(bootstrap, `install -m 0755`) ||
			!strings.Contains(bootstrap, `--http1.1`) ||
			!strings.Contains(bootstrap, `--retry 4`) ||
			!strings.Contains(bootstrap, `--retry-all-errors`) ||
			strings.Contains(bootstrap, `install.sh`) ||
			strings.Contains(bootstrap, `MISE_CURL_OPTS`) ||
			lockedExecution < 1 {
			t.Fatalf("%s script does not checksum and install the exact repository mise asset before locked execution: %q", name, job.Script)
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

type gitLabJob struct {
	Script    []string          `yaml:"script"`
	Variables map[string]string `yaml:"variables"`
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
