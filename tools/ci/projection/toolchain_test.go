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
	Provenance any `toml:"provenance"`
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
	for input, required := range map[string]bool{"linux": true, "$CI_RUNNER_ID": true, "$CI_JOB_NAME": true, "$CI_COMMIT_REF_SLUG": false} {
		if strings.Contains(linux.Cache.Key.Prefix, input) != required {
			t.Fatalf("GitLab cache key membership for %s = %t, want %t: %s", input, !required, required, linux.Cache.Key.Prefix)
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
	for _, projection := range projections[1:] {
		var workflow struct {
			Jobs map[string]struct {
				Env   map[string]string `yaml:"env"`
				Steps []struct {
					Uses string            `yaml:"uses"`
					Env  map[string]string `yaml:"env"`
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
			if job.Env["GODEBUG"] != "" {
				t.Fatalf("%s/%s leaks installation transport into product verification", projection.Path, name)
			}
			for _, step := range job.Steps {
				transport := ""
				if strings.HasPrefix(step.Uses, "jdx/mise-action@") {
					transport = "http2client=0"
					if !step.With.Cache || !step.With.Install || step.With.InstallArgs != "--locked" || step.With.CacheKeyPrefix != "mise-${{ github.job }}" {
						t.Fatalf("%s/%s does not use a scoped native tool cache with locked installation: %#v", projection.Path, name, step.With)
					}
				}
				if step.Env["GODEBUG"] != transport {
					t.Fatalf("%s/%s transport = %q, want %q", projection.Path, name, step.Env["GODEBUG"], transport)
				}
			}
		}
	}
}

func TestGitLabToolchainUsesOfficialRunnableMiseImage(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Toolchain gitLabJob `yaml:".linux-toolchain"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	image, digest, pinned := strings.Cut(pipeline.Toolchain.Image, "@sha256:")
	if !pinned || len(digest) != 64 || !strings.HasPrefix(image, "ghcr.io/jdx/mise:") || !strings.HasSuffix(image, "-debian") {
		t.Fatalf("GitLab must use one runnable official mise image pinned by digest: %q", pipeline.Toolchain.Image)
	}
	if strings.Contains(projections[0].Content, "entrypoint:") {
		t.Fatal("GitLab projection overrides the official runnable image entrypoint")
	}
}

func TestGitLabLinuxToolchainBacksQualifiedNativeCapacity(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		LinuxToolchain gitLabJob  `yaml:".linux-toolchain"`
		NativeLinux    *gitLabJob `yaml:"native-linux"`
		Quality        gitLabJob  `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	wantBootstrap := []string{
		"apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y gcc libatomic1 libc6-dev openssh-client procps",
		"env GODEBUG=http2client=0 mise install --locked",
	}
	if got := pipeline.LinuxToolchain.BeforeScript; !slices.Equal(got, wantBootstrap) {
		t.Fatalf("Linux bootstrap = %q, want %q", got, wantBootstrap)
	}
	if pipeline.NativeLinux == nil {
		t.Fatal("GitLab projection lacks qualified native Linux capacity")
	}
	if !slices.Equal(pipeline.NativeLinux.Extends, []string{".linux-toolchain"}) {
		t.Fatalf("GitLab native Linux toolchain = %q", pipeline.NativeLinux.Extends)
	}
	if !slices.Equal(pipeline.NativeLinux.Tags, []string{"$AIGW_GITLAB_LINUX_RUNNER_TAG"}) {
		t.Fatalf("GitLab native Linux runner tags = %q", pipeline.NativeLinux.Tags)
	}
	if len(pipeline.Quality.Extends) != 0 || pipeline.Quality.Variables["CGO_ENABLED"] != "1" {
		t.Fatalf("GitLab quality must use the selected control executor directly: %#v", pipeline.Quality)
	}
	if !slices.Equal(pipeline.Quality.Tags, []string{"$AIGW_GITLAB_DARWIN_RUNNER_TAG"}) {
		t.Fatalf("GitLab quality runner tags = %q", pipeline.Quality.Tags)
	}
	if len(pipeline.Quality.Script) < 2 || pipeline.Quality.Script[0] != "env GODEBUG=http2client=0 mise install --locked" || pipeline.Quality.Script[1] != "mise run bootstrap" {
		t.Fatalf("GitLab quality bootstrap = %q", pipeline.Quality.Script)
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
	image, digest, pinned := strings.Cut(gitlab.Toolchain.Image, "@sha256:")
	_, version, tagged := strings.Cut(image, ":")
	version, runnable := strings.CutSuffix(version, "-debian")
	if !pinned || len(digest) != 64 || !tagged || !runnable || version == "" {
		t.Fatalf("mise image must identify one runnable version and digest: %q", gitlab.Toolchain.Image)
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
	projections = append(projections, projection{Path: "local", Content: "{}"})
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

func TestLockRefreshIsExplicitAndUsesOneNativeTask(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type     string    `yaml:"type"`
					Required bool      `yaml:"required"`
					Default  yaml.Node `yaml:"default"`
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
	if !present || input.Type != "boolean" || input.Required || input.Default.Value != "false" {
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
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]gitLabJob{"darwin": gitlab.Darwin} {
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
		Quality gitLabJob `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	qualityTools := pipeline.Quality.Variables["MISE_ENABLE_TOOLS"]
	if qualityTools == "" {
		t.Fatal("GitLab quality job must declare its native toolchain")
	}
	if !slices.Equal(pipeline.Quality.Tags, []string{"$AIGW_GITLAB_DARWIN_RUNNER_TAG"}) {
		t.Fatalf("GitLab quality runner tags = %q", pipeline.Quality.Tags)
	}
	if len(pipeline.Quality.Script) < 2 || pipeline.Quality.Script[0] != "env GODEBUG=http2client=0 mise install --locked" || pipeline.Quality.Script[1] != "mise run bootstrap" {
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
