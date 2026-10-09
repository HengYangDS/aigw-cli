package projection

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitLabJobsUseEventScopedRunnerSelectors(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Parity         gitLabJob `yaml:"accepted-ref-parity"`
		Quality        gitLabJob `yaml:"quality"`
		NativeDarwin   gitLabJob `yaml:"native-darwin"`
		ReleaseVersion gitLabJob `yaml:"release-version"`
		ReleaseAssets  gitLabJob `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"accepted-ref-parity": pipeline.Parity,
		"quality":             pipeline.Quality,
		"native-darwin":       pipeline.NativeDarwin,
		"release-version":     pipeline.ReleaseVersion,
		"release-assets":      pipeline.ReleaseAssets,
	} {
		if want := []string{"$AIGW_CI_DARWIN_RUNNER_TAG"}; !slices.Equal(job.Tags, want) {
			t.Errorf("%s runner tags = %q, want %q", name, job.Tags, want)
		}
	}
	var workflow struct {
		Workflow struct {
			Rules []struct {
				If        string            `yaml:"if"`
				Variables map[string]string `yaml:"variables"`
			} `yaml:"rules"`
		} `yaml:"workflow"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, rule := range workflow.Workflow.Rules {
		if rule.If == "" {
			continue
		}
		review := strings.Contains(rule.If, "merge_request_event") || strings.Contains(rule.If, `$CI_COMMIT_REF_PROTECTED == "false"`)
		for variable, tags := range map[string][2]string{
			"AIGW_CI_DARWIN_RUNNER_TAG":  {"ci-macos-arm64-shell", "ci-macos-arm64-review"},
			"AIGW_CI_LINUX_RUNNER_TAG":   {"ci-linux-arm64-container-protected", "ci-linux-arm64-container"},
			"AIGW_CI_WINDOWS_RUNNER_TAG": {"ci-windows-arm64-shell", "ci-windows-arm64-review"},
		} {
			want := tags[0]
			if review {
				want = tags[1]
			}
			if got := rule.Variables[variable]; got != want {
				t.Errorf("%s selects %s = %q, want %q", rule.If, variable, got, want)
			}
		}
	}
}

func TestGitHubNativeJobsUseHostedRunners(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type        string    `yaml:"type"`
					Default     yaml.Node `yaml:"default"`
					Description string    `yaml:"description"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			RunsOn yaml.Node `yaml:"runs-on"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{"self_hosted_linux_arm64", "self_hosted_windows_arm64"} {
		if _, present := workflow.On.Dispatch.Inputs[input]; present {
			t.Errorf("GitHub Verify retains obsolete input %q", input)
		}
	}
	for name, runner := range map[string]string{
		"accepted-ref-parity": "ubuntu-latest",
		"quality":             "ubuntu-latest",
		"native-darwin":       "macos-26-intel",
		"native-linux":        "ubuntu-latest",
		"native-windows":      "windows-latest",
	} {
		if got := workflow.Jobs[name].RunsOn.Value; got != runner {
			t.Errorf("%s runner = %q, want %q", name, got, runner)
		}
	}
}

func TestEachForgeProjectsTheCompleteNativeMatrix(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeDarwin  *gitLabJob `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		NativeWindows *gitLabJob `yaml:"native-windows"`
		Assets        gitLabJob  `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(projections[0].Content, "allow_failure:") {
		t.Fatal("GitLab projection weakens a native job with allow_failure")
	}
	var github struct {
		Jobs map[string]any `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]*gitLabJob{
		"darwin": gitlab.NativeDarwin, "linux": gitlab.NativeLinux, "windows": gitlab.NativeWindows,
	} {
		name := "native-" + platform
		if job == nil {
			t.Errorf("GitLab lacks %s", name)
			continue
		}
		if _, present := github.Jobs[name]; !present {
			t.Errorf("GitHub lacks %s", name)
		}
		if !slices.ContainsFunc(job.Script, func(command string) bool {
			return strings.Contains(command, "go run ./tools/ci native --platform "+platform)
		}) {
			t.Errorf("GitLab %s lacks its native product journey: %q", name, job.Script)
		}
		if !slices.ContainsFunc(gitlab.Assets.Needs, func(need gitLabNeed) bool { return need.Job == name }) {
			t.Errorf("GitLab assets do not require %s", name)
		}
	}
}
