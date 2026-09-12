package projection

import (
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type miseLock struct {
	Tools map[string][]struct {
		LinuxARM64 misePlatformLock `toml:"platforms.linux-arm64"`
	} `toml:"tools"`
}

type misePlatformLock struct {
	Provenance         any  `toml:"provenance"`
	ProvenanceVerified bool `toml:"provenance_verified"`
}

func TestToolchainCacheStaysOutsideGoPackageDiscovery(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Linux struct {
			Variables map[string]string `yaml:"variables"`
		} `yaml:".linux-toolchain"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cache := strings.ReplaceAll(pipeline.Linux.Variables["MISE_DATA_DIR"], "$CI_PROJECT_DIR", root)
	for path, source := range map[string]string{
		filepath.Join(root, "go.mod"):                              "module fixture\n",
		filepath.Join(root, "product.go"):                          "package fixture\n",
		filepath.Join(cache, "installs", "tool", "src", "tool.go"): "package tool\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "list", "./...")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "fixture" {
		t.Fatalf("tool cache became a product package: %s, %v", output, err)
	}
}

func TestToolchainCachesPreserveLockAndExecutionBoundaries(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Linux struct {
			Variables    map[string]string `yaml:"variables"`
			BeforeScript []string          `yaml:"before_script"`
			Cache        struct {
				Key struct {
					Files  []string `yaml:"files"`
					Prefix string   `yaml:"prefix"`
				} `yaml:"key"`
				Paths  []string `yaml:"paths"`
				Policy string   `yaml:"policy"`
				When   string   `yaml:"when"`
			} `yaml:"cache"`
		} `yaml:".linux-toolchain"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	linux := pipeline.Linux
	if !slices.Equal(linux.Cache.Key.Files, []string{"mise.toml", "mise.lock"}) || linux.Cache.Policy != "pull-push" {
		t.Fatalf("tool cache must be bound to native manifests and locks: %#v", linux.Cache)
	}
	if linux.Cache.When != "always" {
		t.Fatal("completed tool installations must survive a later job failure")
	}
	for _, input := range []string{"linux", "$CI_RUNNER_ID", "$CI_JOB_NAME", "$CI_COMMIT_REF_SLUG"} {
		if !strings.Contains(linux.Cache.Key.Prefix, input) {
			t.Fatalf("GitLab cache key lacks %s: %s", input, linux.Cache.Key.Prefix)
		}
	}
	if len(linux.Cache.Paths) != 2 || linux.Variables["MISE_DATA_DIR"]+"/installs/" != "$CI_PROJECT_DIR/"+linux.Cache.Paths[0] || linux.Variables["MISE_CACHE_DIR"]+"/" != "$CI_PROJECT_DIR/"+linux.Cache.Paths[1] {
		t.Fatalf("cache must retain job-local installs with their native completion metadata: %#v", linux)
	}
	layout := strings.ReplaceAll(strings.TrimPrefix(linux.Variables["MISE_DATA_DIR"], "$CI_PROJECT_DIR/"), "/", "-")
	directories := make([]string, 0, len(linux.Cache.Paths))
	for _, path := range linux.Cache.Paths {
		directories = append(directories, filepath.Base(filepath.Clean(path)))
	}
	if !strings.Contains(linux.Cache.Key.Prefix, layout+"-"+strings.Join(directories, "-")) {
		t.Fatal("cache key must invalidate archives from a different installation or metadata layout")
	}
	if !slices.Contains(linux.BeforeScript, "mise install --locked") {
		t.Fatal("cache hit must not replace the locked installation command")
	}
	for _, projection := range projections[1:] {
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string `yaml:"uses"`
					With struct {
						Cache          bool   `yaml:"cache"`
						Install        bool   `yaml:"install"`
						InstallArgs    string `yaml:"install_args"`
						CacheKeyPrefix string `yaml:"cache_key_prefix"`
					} `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
			t.Fatal(err)
		}
		for name, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if !strings.HasPrefix(step.Uses, "jdx/mise-action@") {
					continue
				}
				if !step.With.Cache || !step.With.Install || step.With.InstallArgs != "--locked" || step.With.CacheKeyPrefix != "mise-${{ github.job }}" {
					t.Fatalf("%s/%s does not use a scoped native tool cache with locked installation: %#v", projection.Path, name, step.With)
				}
			}
		}
	}
}

func TestGitLabToolchainImagesYieldToTheRunnerShell(t *testing.T) {
	content := `package ci
#ToolchainImage: {
	name: "example.invalid/toolchain@sha256:fixture"
	entrypoint: [""]
}
gitlab: {
	".linux-toolchain": {image: #ToolchainImage}
	quality: {extends: [".linux-toolchain"]}
	"native-linux": {extends: [".linux-toolchain"]}
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
	if got := strings.Count(gitlab, "entrypoint:\n      - \"\""); got != 1 {
		t.Fatalf("empty GitLab image entrypoints = %d, want 1:\n%s", got, gitlab)
	}
}

func TestGitLabLinuxJobsUseOneLockedToolchainImage(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Variables        map[string]string `yaml:"variables"`
		LinuxToolchain   gitLabJob         `yaml:".linux-toolchain"`
		Quality          gitLabJob         `yaml:"quality"`
		NativeLinux      gitLabJob         `yaml:"native-linux"`
		ReleaseReadiness gitLabJob         `yaml:"release-readiness"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pipeline.LinuxToolchain.BeforeScript, []string{"mise install --locked"}) {
		t.Fatalf("Linux bootstrap must install the repository lock directly: %q", pipeline.LinuxToolchain.BeforeScript)
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
	if !slices.Equal(pipeline.Quality.Extends, []string{".linux-toolchain"}) {
		t.Fatalf("quality extends = %q, want [.linux-toolchain]", pipeline.Quality.Extends)
	}
}

func TestForgeBootstrapUsesOneMiseRelease(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		Toolchain gitLabJob `yaml:".linux-toolchain"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	image, digest, pinned := strings.Cut(gitlab.Toolchain.Image.Name, "@sha256:")
	_, version, tagged := strings.Cut(image, ":")
	if !pinned || len(digest) != 64 || !tagged || version == "" {
		t.Fatalf("mise image must identify one version and digest: %q", gitlab.Toolchain.Image.Name)
	}
	for _, projection := range projections[1:] {
		var github struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string            `yaml:"uses"`
					With map[string]string `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projection.Content), &github); err != nil {
			t.Fatal(err)
		}
		observed := 0
		for name, job := range github.Jobs {
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "jdx/mise-action@") {
					observed++
					if got := step.With["version"]; got != version {
						t.Errorf("%s bootstrap version = %q, want image version %q", name, got, version)
					}
				}
			}
		}
		if observed == 0 {
			t.Fatal("workflow did not exercise a mise bootstrap step")
		}
	}
}

func TestForgeToolchainsIgnoreForeignMiseConfiguration(t *testing.T) {
	repository := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(repository)
	if err != nil {
		t.Fatal(err)
	}
	earlyConfig, err := os.ReadFile(filepath.Join(repository, ".config", "miserc.toml"))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	root := filepath.Join(workspace, "checkout with spaces")
	inputs := map[string]string{
		filepath.Join(workspace, "mise.toml"):             "[env]\nAIGW_PARENT_PROBE = 'foreign'\n",
		filepath.Join(workspace, "global", "config.toml"): "[env]\nAIGW_GLOBAL_PROBE = 'foreign'\n",
		filepath.Join(workspace, "system", "config.toml"): "[env]\nAIGW_SYSTEM_PROBE = 'foreign'\n",
		filepath.Join(root, "mise.toml"):                  "[env]\nAIGW_PROJECT_PROBE = 'owned'\n",
		filepath.Join(root, ".config", "miserc.toml"):     string(earlyConfig),
		filepath.Join(root, ".config", "ci", ".keep"):     "",
		filepath.Join(root, "nested", ".keep"):            "",
	}
	for path, content := range inputs {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	environment := slices.DeleteFunc(os.Environ(), func(value string) bool {
		name, _, _ := strings.Cut(value, "=")
		return strings.HasPrefix(name, "MISE_") || strings.HasPrefix(name, "__MISE_")
	})
	environment = append(environment,
		"MISE_CONFIG_DIR="+filepath.Join(workspace, "global"),
		"MISE_GLOBAL_CONFIG_FILE="+filepath.Join(workspace, "global", "config.toml"),
		"MISE_SYSTEM_CONFIG_DIR="+filepath.Join(workspace, "system"),
		"MISE_TRUSTED_CONFIG_PATHS="+workspace,
	)
	for _, projection := range projections {
		t.Run(projection.Path, func(t *testing.T) {
			var config struct {
				Env       map[string]string `yaml:"env"`
				Variables map[string]string `yaml:"variables"`
			}
			if err := yaml.Unmarshal([]byte(projection.Content), &config); err != nil {
				t.Fatal(err)
			}
			projected := make(map[string]string)
			maps.Copy(projected, config.Env)
			maps.Copy(projected, config.Variables)
			projectedEnvironment := slices.Clone(environment)
			for name, value := range projected {
				if strings.HasPrefix(name, "MISE_") {
					value = strings.NewReplacer("$CI_PROJECT_DIR", root, "${{ github.workspace }}", root).Replace(value)
					projectedEnvironment = append(projectedEnvironment, name+"="+value)
				}
			}
			for _, directory := range []string{root, filepath.Join(root, "nested")} {
				command := exec.Command("mise", "env", "--json")
				command.Dir = directory
				command.Env = projectedEnvironment
				output, err := command.Output()
				if err != nil {
					t.Fatalf("native mise configuration discovery: %v", err)
				}
				var observed map[string]string
				if err := json.Unmarshal(output, &observed); err != nil {
					t.Fatal(err)
				}
				for name, want := range map[string]string{
					"AIGW_PROJECT_PROBE": "owned", "AIGW_PARENT_PROBE": "",
					"AIGW_GLOBAL_PROBE": "", "AIGW_SYSTEM_PROBE": "",
				} {
					if observed[name] != want {
						t.Errorf("%s from %s = %q, want %q", name, directory, observed[name], want)
					}
				}
			}
		})
	}
	for path, want := range inputs {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Errorf("configuration discovery modified %s: %v", path, err)
		}
	}
}

func TestNativeJobsEnableTheirExactCommandToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Quality      gitLabJob `yaml:"quality"`
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
		for tool := range strings.SplitSeq(pipeline.Quality.Variables["MISE_ENABLE_TOOLS"], ",") {
			if !slices.Contains(strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ","), tool) {
				t.Errorf("GitLab %s cannot run repository conformance tests: missing %s", name, tool)
			}
		}
		for _, tool := range []string{"glab", "github:goreleaser/goreleaser", "github:anchore/syft"} {
			if !slices.Contains(strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ","), tool) {
				t.Errorf("GitLab %s lacks native release conformance tool %s", name, tool)
			}
		}
		bootstrap := slices.Index(job.Script, "mise run bootstrap")
		if bootstrap < 0 || bootstrap >= len(job.Script)-1 {
			t.Errorf("GitLab %s must prepare locked dependencies before native acceptance", name)
		}
	}

	var workflow struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	for _, projection := range projections[1:] {
		if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"native-darwin", "native-linux", "native-windows"} {
			job := workflow.Jobs[name]
			if got := job.Env["MISE_ENABLE_TOOLS"]; got != pipeline.NativeDarwin.Variables["MISE_ENABLE_TOOLS"] {
				t.Errorf("%s %s tool closure = %q, want the same native closure as GitLab", projection.Path, name, got)
			}
			bootstrap := false
			for _, step := range job.Steps {
				if strings.Contains(step.Run, "./tools/ci native") && !bootstrap {
					t.Errorf("%s %s runs native acceptance before locked dependency preparation", projection.Path, name)
				}
				bootstrap = bootstrap || step.Run == "mise run bootstrap"
			}
		}
	}
}

func TestLockRefreshIsExplicitAndUsesOneNativeTask(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type     string `yaml:"type"`
					Required bool   `yaml:"required"`
					Default  bool   `yaml:"default"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				If   string            `yaml:"if"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	input, present := workflow.On.Dispatch.Inputs["refresh_locks"]
	if !present || input.Type != "boolean" || input.Required || input.Default {
		t.Fatal("lock refresh must be an explicit optional manual input")
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		found := false
		for _, step := range workflow.Jobs["native-"+platform].Steps {
			if step.Name != "Verify native lock resolution" {
				continue
			}
			found = true
			if step.If != "github.event_name == 'workflow_dispatch' && inputs.refresh_locks" || step.Run != "mise run dependencies:resolve" || step.Env["MISE_GITHUB_TOKEN"] != "${{ github.token }}" {
				t.Fatalf("%s lock refresh must use scoped native authority: %#v", platform, step)
			}
		}
		if !found {
			t.Fatalf("%s lacks manual lock resolution", platform)
		}
	}
	var gitlab struct {
		Darwin gitLabJob `yaml:"native-darwin"`
		Linux  gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]gitLabJob{"darwin": gitlab.Darwin, "linux": gitlab.Linux} {
		if !slices.Contains(job.Script, `if [ "${AIGW_REFRESH_LOCKS:-false}" = true ]; then mise run dependencies:resolve; fi`) {
			t.Fatalf("GitLab %s lacks the same opt-in lock task", platform)
		}
	}
}

func TestQualityJobsUseTheirExactToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		LinuxToolchain gitLabJob `yaml:".linux-toolchain"`
		Quality        gitLabJob `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	qualityTools := pipeline.Quality.Variables["MISE_ENABLE_TOOLS"]
	if qualityTools == "" {
		t.Fatal("GitLab quality job must declare its native toolchain")
	}
	if !slices.Contains(pipeline.Quality.Script, "mise run bootstrap") {
		t.Fatalf("GitLab quality job lacks locked dependency preparation: %q", pipeline.Quality.Script)
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
	if got := github.Jobs["quality"].Env["MISE_ENABLE_TOOLS"]; got != qualityTools {
		t.Fatalf("GitHub quality tools = %q, want %q", got, qualityTools)
	}
	steps := github.Jobs["quality"].Steps
	if !slices.ContainsFunc(steps, func(step struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
	}) bool {
		return step.Name == "Prepare locked dependencies" && step.Run == "mise run bootstrap"
	}) {
		t.Fatalf("GitHub quality job lacks locked dependency preparation: %#v", steps)
	}
}
