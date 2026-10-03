package projection

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

func TestGitLabUnixLockedToolsUseJobScopedMirrorWithoutChangingGitHub(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		Variables      map[string]string `yaml:"variables"`
		LinuxToolchain struct {
			BeforeScript []string `yaml:"before_script"`
			AfterScript  []string `yaml:"after_script"`
		} `yaml:".linux-toolchain"`
		NativeDarwin struct {
			Script      []string `yaml:"script"`
			AfterScript []string `yaml:"after_script"`
		} `yaml:"native-darwin"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.Variables["AIGW_TOOL_SOURCE"] != "upstream" {
		t.Fatal("GitLab must default to explicit locked upstream acquisition")
	}
	for name, commands := range map[string][]string{
		"Linux": gitlab.LinuxToolchain.BeforeScript,
		"macOS": gitlab.NativeDarwin.Script,
	} {
		mirror := slices.IndexFunc(commands, func(command string) bool {
			return strings.Contains(command, "MISE_URL_REPLACEMENTS")
		})
		install := slices.IndexFunc(commands, func(command string) bool {
			return strings.Contains(command, "mise install --locked")
		})
		if mirror < 0 || install <= mirror {
			t.Errorf("GitLab %s must configure the mirror before locked installation: %v", name, commands)
			continue
		}
		prelude := commands[mirror]
		if !strings.Contains(prelude, "AIGW_TOOL_SOURCE") {
			t.Errorf("GitLab %s has no explicit mirror selection", name)
		}
		for _, required := range []string{"CI_API_V4_URL", "CI_PROJECT_ID", "CI_SERVER_HOST", "CI_JOB_TOKEN", "MISE_NETRC_FILE", "$mirror_dir/mise-data", "$mirror_dir/mise-cache", "github.com/", "api.github.com/", "mise-github/v1/", "CI_JOB_ID"} {
			if !strings.Contains(prelude, required) {
				t.Errorf("GitLab %s mirror prelude omits %q", name, required)
			}
		}
		metadata := strings.Index(prelude, "regex:^https://api[.]github[.]com/repos/")
		fallback := strings.Index(prelude, `"https://api.github.com/"`)
		if metadata < 0 || fallback <= metadata || !strings.Contains(prelude, "release-$1-$2-$3.json") {
			t.Errorf("GitLab %s must resolve mirrored release metadata before the API fallback", name)
		}
		for _, forbidden := range []string{"192.168.64.101", "projects/456", "$HOME/.netrc"} {
			if strings.Contains(prelude, forbidden) {
				t.Errorf("GitLab %s mirror prelude hard-codes %q", name, forbidden)
			}
		}
	}
	for name, commands := range map[string][]string{
		"Linux": gitlab.LinuxToolchain.AfterScript,
		"macOS": gitlab.NativeDarwin.AfterScript,
	} {
		if !slices.ContainsFunc(commands, func(command string) bool {
			return strings.Contains(command, "CI_JOB_ID") && strings.Contains(command, "mise-mirror")
		}) {
			t.Errorf("GitLab %s has no exact job-owned mirror credential cleanup: %v", name, commands)
		}
	}
	if strings.Contains(projections[1].Content, "mise-github/v1/") {
		t.Fatal("GitHub projection unexpectedly depends on the GitLab tool mirror")
	}
}

func TestGitHubToolSourceSelectsItsReciprocalMirrorExplicitly(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	command := projectionCommand(root, "miseMirror.githubEnvironment")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var declared map[string]string
	if err := yaml.Unmarshal(output, &declared); err != nil {
		t.Fatal(err)
	}
	for _, item := range projections[1:] {
		t.Run(item.Path, func(t *testing.T) {
			var workflow struct {
				On struct {
					Dispatch struct {
						Inputs struct {
							ToolSource struct {
								Default string   `yaml:"default"`
								Options []string `yaml:"options"`
							} `yaml:"tool_source"`
						} `yaml:"inputs"`
					} `yaml:"workflow_dispatch"`
				} `yaml:"on"`
				Jobs map[string]struct {
					Steps []struct {
						Name string            `yaml:"name"`
						Env  map[string]string `yaml:"env"`
					} `yaml:"steps"`
				} `yaml:"jobs"`
			}
			if err := yaml.Unmarshal([]byte(item.Content), &workflow); err != nil {
				t.Fatal(err)
			}
			if choice := workflow.On.Dispatch.Inputs.ToolSource; choice.Default != "upstream" || !slices.Equal(choice.Options, []string{"upstream", "peer"}) {
				t.Fatal("tool source must default to locked upstream with an explicit peer choice")
			}
			found := false
			for name, job := range workflow.Jobs {
				for _, step := range job.Steps {
					if step.Name != "Install the locked toolchain" {
						continue
					}
					found = true
					selected := step.Env["MISE_URL_REPLACEMENTS"]
					if selected == declared["MISE_URL_REPLACEMENTS"] || strings.Count(selected, "${{") != 1 ||
						!strings.Contains(selected, "format(") || !strings.Contains(selected, "inputs.tool_source == 'peer'") ||
						!strings.Contains(selected, "github.server_url") || !strings.Contains(selected, "github.repository") ||
						!strings.Contains(selected, "|| ''") {
						t.Errorf("%s must resolve peer URLs inside one outer GitHub expression", name)
					}
					for _, forbidden := range []string{"MISE_GITLAB_TOKEN", "MISE_NETRC_FILE", "MISE_NETRC"} {
						if step.Env[forbidden] != "" {
							t.Errorf("%s reciprocal public mirror inherits %s", name, forbidden)
						}
					}
				}
			}
			if !found {
				t.Fatal("workflow lost its locked toolchain consumer")
			}
		})
	}
}

func TestGitHubMirrorRewritesEveryLockedGlabEndpoint(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	output, err := projectionCommand(root, "miseMirror.githubEnvironment").Output()
	if err != nil {
		t.Fatal(err)
	}
	var environment map[string]string
	if err := yaml.Unmarshal(output, &environment); err != nil {
		t.Fatal(err)
	}
	serverURL, repository := "https://github.example", "owner/project"
	template := strings.NewReplacer("{{", "{", "}}", "}", "{0}", serverURL, "{1}", repository).
		Replace(environment["MISE_URL_REPLACEMENTS"])
	var replacements map[string]string
	if err := json.Unmarshal([]byte(template), &replacements); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock miseLock
	if err := toml.Unmarshal(content, &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Tools["glab"]) != 1 || len(replacements) != 2 {
		t.Fatal("one locked glab must have exactly browser and API mirror contracts")
	}
	entry := lock.Tools["glab"][0]
	for _, platform := range []misePlatformLock{entry.LinuxARM64, entry.LinuxX64, entry.MacOSARM64, entry.MacOSX64, entry.WindowsARM64, entry.WindowsX64} {
		for _, source := range []string{platform.URL, platform.URLAPI} {
			t.Run(source, func(t *testing.T) {
				matched := 0
				for pattern, target := range replacements {
					expression, err := regexp.Compile(strings.TrimPrefix(pattern, "regex:"))
					if err != nil {
						t.Fatal(err)
					}
					if !expression.MatchString(source) {
						continue
					}
					matched++
					resolved, err := url.PathUnescape(expression.ReplaceAllString(source, target))
					filename, filenameErr := url.PathUnescape(path.Base(source))
					if err != nil || filenameErr != nil || !strings.HasPrefix(resolved, serverURL+"/"+repository+"/releases/download/") || strings.Contains(resolved, "gitlab") || !strings.HasSuffix(resolved, "/"+filename) {
						t.Errorf("locked asset rewritten outside the selected peer or renamed: %q, %v, %v", resolved, err, filenameErr)
					}
				}
				if matched != 1 {
					t.Errorf("locked endpoint has %d mirror owners, want one: %s", matched, source)
				}
			})
		}
	}
}

func TestGitLabMiseMirrorUsesCheckoutOwnedPath(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Review struct {
			Script      []string `yaml:"script"`
			AfterScript []string `yaml:"after_script"`
		} `yaml:"native-darwin-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if len(pipeline.Review.Script) == 0 || len(pipeline.Review.AfterScript) != 1 {
		t.Fatalf("macOS review mirror lifecycle is incomplete: %+v", pipeline.Review)
	}
	prepare, cleanup := pipeline.Review.Script[0], pipeline.Review.AfterScript[0]
	for _, script := range []string{prepare, cleanup} {
		if strings.Contains(script, "CI_BUILDS_DIR") || !strings.Contains(script, "$(pwd -P)/build/tmp/.aigw-mise-mirror-$CI_JOB_ID") {
			t.Fatalf("mirror path must be rooted at the checked-out working directory: %q", script)
		}
	}
	if runtime.GOOS == "windows" {
		return // Windows runners do not provide a POSIX shell.
	}
	project := t.TempDir()
	neighbor := filepath.Join(project, "build", "tmp", "keep")
	for _, choice := range []string{"upstream", "invalid"} {
		command := exec.Command("sh", "-c", prepare)
		command.Dir = project
		command.Env = append(os.Environ(), "AIGW_TOOL_SOURCE="+choice)
		output, err := command.CombinedOutput()
		if (err == nil) != (choice == "upstream") {
			t.Fatalf("tool source %s: %v\n%s", choice, err, output)
		}
		if choice == "invalid" && !strings.Contains(string(output), "AIGW_TOOL_SOURCE must be upstream or peer") {
			t.Fatalf("invalid source has no precise refusal: %s", output)
		}
		if entries, err := os.ReadDir(project); err != nil || len(entries) != 0 {
			t.Fatalf("unselected mirror wrote private state: %v, %v", entries, err)
		}
	}
	env := append(os.Environ(),
		"AIGW_TOOL_SOURCE=peer",
		"CI_PROJECT_DIR="+project,
		"CI_BUILDS_DIR=builds",
		"CI_API_V4_URL=https://gitlab.example.invalid/api/v4",
		"CI_PROJECT_ID=456",
		"CI_SERVER_HOST=gitlab.example.invalid",
		"CI_JOB_ID=123",
		"CI_JOB_TOKEN=fixture-only",
	)
	for _, script := range []string{prepare, cleanup} {
		command := exec.Command("sh", "-c", script)
		command.Dir, command.Env = project, env
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("mirror lifecycle: %v\n%s", err, output)
		}
		mirror := filepath.Join(project, "build", "tmp", ".aigw-mise-mirror-123")
		if script == prepare {
			if _, err := os.Stat(filepath.Join(mirror, "netrc")); err != nil {
				t.Fatalf("mirror was not prepared: %v", err)
			}
			if err := os.WriteFile(neighbor, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if _, err := os.Stat(mirror); !os.IsNotExist(err) {
			t.Fatalf("mirror remains after cleanup: %v", err)
		}
		if _, err := os.Stat(neighbor); err != nil {
			t.Fatalf("mirror cleanup removed neighboring state: %v", err)
		}
	}
	t.Run("relative checkout survives installer directory changes", func(t *testing.T) {
		project := t.TempDir()
		env := append(os.Environ(),
			"AIGW_TOOL_SOURCE=peer",
			"CI_PROJECT_DIR=builds/runner/0/group/repo",
			"CI_API_V4_URL=https://gitlab.example.invalid/api/v4",
			"CI_PROJECT_ID=456",
			"CI_SERVER_HOST=gitlab.example.invalid",
			"CI_JOB_ID=456",
			"CI_JOB_TOKEN=fixture-only",
		)
		probe := `mkdir -p "$MISE_DATA_DIR/installs/go/probe/bin"
fake_go="$MISE_DATA_DIR/installs/go/probe/bin/go"
printf '%s\n' '#!/bin/sh' 'printf "go-probe=pass\\n"' > "$fake_go"
chmod 700 "$fake_go"
mkdir -p tool-cwd
cd tool-cwd
"$fake_go" version
for path in "$MISE_DATA_DIR" "$MISE_CACHE_DIR" "$MISE_NETRC_FILE"; do
	case "$path" in /*) ;; *) printf 'non-absolute mirror path: %s\\n' "$path" >&2; exit 1 ;; esac
done`
		command := exec.Command("sh", "-eu", "-c", prepare+"\n"+probe)
		command.Dir, command.Env = project, env
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("relative mirror paths must survive installer directory changes: %v\n%s", err, output)
		}
		command = exec.Command("sh", "-eu", "-c", cleanup)
		command.Dir, command.Env = project, env
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("relative mirror cleanup: %v\n%s", err, output)
		}
		mirror := filepath.Join(project, "build", "tmp", ".aigw-mise-mirror-456")
		if _, err := os.Stat(mirror); !os.IsNotExist(err) {
			t.Fatalf("relative mirror remains after cleanup: %v", err)
		}
	})
}

func TestGitLabWindowsLockedToolsUseJobScopedMirror(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	type windowsJob struct {
		Script      []string `yaml:"script"`
		AfterScript []string `yaml:"after_script"`
	}
	var gitlab struct {
		NativeWindows       windowsJob `yaml:"native-windows"`
		NativeWindowsReview windowsJob `yaml:"native-windows-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	windows := gitlab.NativeWindows
	if len(windows.Script) < 2 {
		t.Fatalf("Windows job lacks the Mise preflight and locked install: %+v", windows)
	}
	if !strings.Contains(windows.Script[1], "mise install --locked") {
		t.Fatalf("Windows job does not install locked tools: %+v", windows)
	}
	const jobDirectory = `Join-Path (Split-Path -Parent $env:CI_PROJECT_DIR) "aigw-ci-mise-$env:CI_JOB_ID"`
	for name, job := range map[string]windowsJob{"protected": windows, "review": gitlab.NativeWindowsReview} {
		requireWindowsMiseJobStorage(t, name, job.Script, job.AfterScript, jobDirectory)
		selected := strings.Index(job.Script[0], "if ($env:AIGW_TOOL_SOURCE -eq 'peer')")
		credentials := strings.Index(job.Script[0], "$netrc = ")
		if selected < 0 || credentials <= selected {
			t.Errorf("%s Windows job acquires mirror credentials before explicit selection", name)
		}
		if !strings.Contains(job.Script[0], `= "${mirrorBase}" + 'release-$1-$2-$3.json'`) {
			t.Errorf("%s Windows job expands Mise regex captures before Mise receives them", name)
		}
	}
	if len(gitlab.NativeWindowsReview.Script) == 0 || windows.Script[0] != gitlab.NativeWindowsReview.Script[0] ||
		!reflect.DeepEqual(windows.AfterScript, gitlab.NativeWindowsReview.AfterScript) {
		t.Fatal("protected and review Windows jobs use different Mise admission or cleanup")
	}
	if _, err := os.Stat(filepath.Join(root, "tools", "ci", "bootstrap", "mise-windows.ps1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete Windows bootstrap remains: %v", err)
	}
	for _, required := range []string{"MISE_URL_REPLACEMENTS", "MISE_NETRC_FILE", "CI_SERVER_HOST", "CI_JOB_TOKEN", "packages/generic/mise-github/v1/", "release-$1-$2-$3.json", "https://github.com/", "https://api.github.com/", "icacls"} {
		if !strings.Contains(windows.Script[0], required) {
			t.Errorf("Windows job omits scoped mirror control %q", required)
		}
	}
	probe := strings.Index(windows.Script[0], "$reported = & $mise --version")
	if probe < 0 {
		t.Fatal("Windows job never probes the runner-owned Mise executable")
	}
	for _, name := range []string{"MISE_CONFIG_DIR", "MISE_CACHE_DIR", "MISE_STATE_DIR", "MISE_DATA_DIR"} {
		assignment := strings.Index(windows.Script[0], "'"+name+"'")
		if assignment < 0 || assignment > probe {
			t.Fatalf("%s is not owned by the job before Mise loads configuration", name)
		}
	}
	if assignment := strings.Index(windows.Script[0], "[Environment]::SetEnvironmentVariable($name, $path)"); assignment < 0 || assignment > probe ||
		!strings.Contains(windows.Script[0], "$path = Join-Path $jobDirectory $name") {
		t.Fatal("Windows job does not confine Mise state before its first invocation")
	}
	if trust := strings.Index(windows.Script[0], "$env:MISE_TRUSTED_CONFIG_PATHS = $env:CI_PROJECT_DIR"); trust < 0 || trust > probe {
		t.Fatal("Windows job does not trust its exact checkout before Mise walks config ancestors")
	}
	if !strings.Contains(windows.Script[0], "throw 'Runner-owned Mise failed to start under the job identity.'") {
		t.Fatal("Windows job conflates a Mise startup failure with a version mismatch")
	}
}
