package projection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestReconcileOwnsRenderingAndReadOnlyVerification(t *testing.T) {
	content := `package ci
gitlab: {stages: ["verify"]}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	root := projectionRoot(t, content)
	if err := Reconcile(root, true); err == nil {
		t.Fatal("check reported missing projections as current")
	}
	if _, err := os.Stat(filepath.Join(root, ".gitlab-ci.yml")); !os.IsNotExist(err) {
		t.Fatalf("check created a projection: %v", err)
	}
	if err := Reconcile(root, false); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(root, true); err != nil {
		t.Fatalf("fresh projections: %v", err)
	}
	path := filepath.Join(root, ".gitlab-ci.yml")
	edited := []byte("manual: edit\n")
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(root, true); err == nil || !strings.Contains(err.Error(), ".gitlab-ci.yml") {
		t.Fatalf("drift observation: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(edited) {
		t.Fatalf("check changed the edited projection: %q, %v", data, err)
	}
}

func TestProjectionRenderingReportsModelAndExecutableFailures(t *testing.T) {
	root := projectionRoot(t, "not valid CUE")
	if _, err := renderProjections(root); err == nil || !strings.Contains(err.Error(), "render .gitlab-ci.yml") {
		t.Fatalf("invalid model error = %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := renderProjections(root); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("missing CUE error = %v", err)
	}
}

func TestReconcileKeepsExistingFilesWhenRenderingFails(t *testing.T) {
	root := projectionRoot(t, "package ci\ngitlab: {name: \"New\"}\n")
	path := filepath.Join(root, ".gitlab-ci.yml")
	if err := os.WriteFile(path, []byte("name: Existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(root, false); err == nil || !strings.Contains(err.Error(), "github/workflows/verify.yml") {
		t.Fatalf("incomplete model: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "name: Existing\n" {
		t.Fatalf("render failure changed existing projection: %q, %v", data, err)
	}
}

func TestReconcileReportsOwnedTargetWriteFailures(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		path   string
		folder bool
		want   string
	}{
		{name: "file target is directory", path: ".gitlab-ci.yml", folder: true, want: "write projection"},
		{name: "parent is file", path: ".github", want: "create projection directory"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			content := "package ci\ngitlab: {}\ngithubVerify: {}\ngithubRelease: {}\n"
			root := projectionRoot(t, content)
			path := filepath.Join(root, testCase.path)
			var err error
			if testCase.folder {
				err = os.Mkdir(path, 0o700)
			} else {
				err = os.WriteFile(path, []byte("existing file"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := Reconcile(root, false); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("write failure: %v, want %q", err, testCase.want)
			}
		})
	}
}

func projectionRoot(t *testing.T, model string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		".config/ci/pipeline.cue": model,
		".ethos/workspace.toml":   "[branch_roles]\naccepted_branch = 'integration'\nrelease_branch = 'release'\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestProjectionUsesCheckoutBranchRoles(t *testing.T) {
	model, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "ci", "pipeline.cue"))
	if err != nil {
		t.Fatal(err)
	}
	root := projectionRoot(t, string(model))
	items, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		Parity  gitLabJob `yaml:"accepted-ref-parity"`
		Quality gitLabJob `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(items[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if got := gitlab.Parity.Rules[0].If; got != `$CI_PIPELINE_SOURCE == "push" && $CI_COMMIT_BRANCH == "release"` {
		t.Fatalf("GitLab release role = %q", got)
	}
	if got := gitlab.Quality.Rules[1].If; got != `$CI_PIPELINE_SOURCE == "merge_request_event" && ($CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "integration" || $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "release")` {
		t.Errorf("GitLab review roles = %q", got)
	}
	var github struct {
		On map[string]struct {
			Branches []string `yaml:"branches"`
		} `yaml:"on"`
	}
	if err := yaml.Unmarshal([]byte(items[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"push", "pull_request"} {
		want := "integration,release"
		if got := strings.Join(github.On[event].Branches, ","); got != want {
			t.Fatalf("GitHub %s roles = %q, want %q", event, got, want)
		}
	}
	if err := os.Remove(filepath.Join(root, ".ethos", "workspace.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := renderProjections(root); err == nil {
		t.Fatal("missing branch-role authority produced projections")
	}
}
