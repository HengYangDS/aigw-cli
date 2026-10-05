package projection

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

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
		NativeDarwin  gitLabJob `yaml:"native-darwin"`
		NativeLinux   gitLabJob `yaml:"native-linux"`
		NativeWindows gitLabJob `yaml:"native-windows"`
		Quality       gitLabJob `yaml:"quality"`
		SecretService gitLabJob `yaml:"linux-secret-service"`
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
		for _, required := range []string{"CI_API_V4_URL", "CI_PROJECT_ID", "CI_SERVER_HOST", "CI_JOB_TOKEN", "MISE_NETRC_FILE", "$mirror_dir/mise-data", "$MISE_DATA_DIR/cache", "github.com/", "api.github.com/", "packages/generic/mise-github/", "mise.lock", "lock_digest", "CI_JOB_ID"} {
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
	for name, job := range map[string]gitLabJob{
		"macOS":          gitlab.NativeDarwin,
		"Linux":          gitlab.NativeLinux,
		"Windows":        gitlab.NativeWindows,
		"Quality":        gitlab.Quality,
		"Secret Service": gitlab.SecretService,
	} {
		cleanup := job.AfterScript
		if len(cleanup) == 0 {
			cleanup = gitlab.LinuxToolchain.AfterScript
		}
		if len(job.Script) == 0 || len(cleanup) != 1 {
			t.Errorf("GitLab %s must retain one primary and fallback cleanup owner", name)
			continue
		}
		if job.Script[len(job.Script)-1] != cleanup[0] {
			t.Errorf("GitLab %s cleanup must gate the job before after_script", name)
		}
		if !strings.Contains(cleanup[0], "Mise job-owned supply state retired.") {
			t.Errorf("GitLab %s cleanup must attest exact absence after removal", name)
		}
	}
	if strings.Contains(projections[1].Content, "packages/generic/mise-github/") {
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
	prepare, cleanup := unixMiseMirrorCommands(t)
	for _, script := range []string{prepare, cleanup} {
		if strings.Contains(script, "CI_BUILDS_DIR") || !strings.Contains(script, "$(pwd -P)/build/tmp/.aigw-mise-mirror-$CI_JOB_ID") {
			t.Fatalf("mirror path must be rooted at the checked-out working directory: %q", script)
		}
	}
	if !strings.Contains(prepare, `export MISE_DATA_DIR="$mirror_dir/mise-data"`) || !strings.Contains(prepare, `export MISE_CACHE_DIR="$MISE_DATA_DIR/cache"`) {
		t.Fatal("peer selection must own cold native Mise data and cache directories")
	}
	if runtime.GOOS == "windows" {
		return // Windows runners do not provide a POSIX shell.
	}
	project := t.TempDir()
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
	t.Run("cold peer state retires without changing restored cache", func(t *testing.T) {
		root, _ := nativeMiseMirrorInputs(t, filepath.Clean(filepath.Join("..", "..", "..")))
		cache := filepath.Join(root, "build", "runtime", "tool-cache", ".mise", "peer")
		neighbor := filepath.Join(root, "keep")
		if err := os.WriteFile(neighbor, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(cache, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, "preserved"), []byte("restored"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), "sh", "-eu", "-c", prepare+`
test "$MISE_DATA_DIR" = "$mirror_dir/mise-data"
test "$MISE_DATA_DIR" != "$AIGW_MISE_DATA_ROOT"
test "$MISE_CACHE_DIR" = "$MISE_DATA_DIR/cache"
test ! -e "$MISE_DATA_DIR"
mkdir -p "$MISE_DATA_DIR/installs" "$MISE_CACHE_DIR"
printf '%s' completed > "$MISE_DATA_DIR/installs/fixture"
test -f "$MISE_NETRC_FILE"
`+cleanup+`
test ! -e "$mirror_dir"
test ! -e "$MISE_DATA_DIR"
test "$(cat "$AIGW_MISE_DATA_ROOT/preserved")" = restored
`)
		command.Dir = root
		command.Env = append(os.Environ(), "AIGW_TOOL_SOURCE=peer", "AIGW_MISE_DATA_ROOT="+cache, "CI_JOB_ID=cache-fixture", "CI_PROJECT_ID=example", "CI_SERVER_HOST=gitlab.example", "CI_API_V4_URL=https://gitlab.example/api/v4", "CI_JOB_TOKEN=synthetic-job-token")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("cold peer state and exact credential cleanup diverged: %s, %v", output, err)
		}
		if _, err := os.Stat(neighbor); err != nil {
			t.Fatalf("mirror cleanup removed neighboring state: %v", err)
		}
	})
	t.Run("relative checkout survives installer directory changes", func(t *testing.T) {
		project, _ := nativeMiseMirrorInputs(t, filepath.Clean(filepath.Join("..", "..", "..")))
		t.Setenv("AIGW_MISE_DATA_ROOT", t.TempDir())
		env := append(os.Environ(),
			"AIGW_TOOL_SOURCE=peer",
			"AIGW_MISE_DATA_ROOT=",
			"CI_PROJECT_DIR=builds/runner/0/group/repo",
			"CI_API_V4_URL=https://gitlab.example.invalid/api/v4",
			"CI_PROJECT_ID=456",
			"CI_SERVER_HOST=gitlab.example.invalid",
			"CI_JOB_ID=456",
			"CI_JOB_TOKEN=fixture-only",
		)
		probe := `test "$MISE_DATA_DIR" = "$mirror_dir/mise-data"
mkdir -p "$MISE_DATA_DIR/installs/go/probe/bin"
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
	t.Run("cleanup rejects a successful removal that leaves owned state", func(t *testing.T) {
		project := t.TempDir()
		mirror := filepath.Join(project, "build", "tmp", ".aigw-mise-mirror-retained")
		if err := os.MkdirAll(mirror, 0o700); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), "sh", "-eu", "-c", "rm() { return 0; }\n"+cleanup)
		command.Dir = project
		command.Env = append(os.Environ(), "CI_JOB_ID=retained")
		output, err := command.CombinedOutput()
		if err == nil || strings.Contains(string(output), "Mise job-owned supply state retired.") {
			t.Fatalf("retained mirror must refuse cleanup acceptance: %v, %s", err, output)
		}
		if _, err := os.Stat(mirror); err != nil {
			t.Fatalf("failed cleanup must preserve its retained fixture: %v", err)
		}
	})
}

func TestGitLabUnixMiseMirrorFailureStaysOnSelectedPeer(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // The Unix projection requires a POSIX shell.
	}
	repository := filepath.Clean(filepath.Join("..", "..", ".."))
	prepare, cleanup := unixMiseMirrorCommands(t)
	project, selected := nativeMiseMirrorInputs(t, repository)
	mirrorPath := "/api/v4/projects/456/" + lockedMiseMirrorPath(t, project)
	t.Setenv("AIGW_MISE_DATA_ROOT", t.TempDir())
	var requests []string
	var mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.Method+" "+r.Host+r.URL.Path)
		mutex.Unlock()
		http.NotFound(w, r)
	}))
	defer server.Close()
	environment := slices.DeleteFunc(os.Environ(), func(value string) bool {
		name, _, _ := strings.Cut(value, "=")
		return name == "AIGW_MISE_DATA_ROOT" || strings.HasPrefix(name, "MISE_") || strings.HasPrefix(name, "__MISE_") ||
			strings.HasPrefix(name, "GH_") || strings.HasPrefix(name, "GITHUB_") || strings.HasPrefix(name, "GITLAB_")
	})
	environment = append(environment,
		"AIGW_MISE_DATA_ROOT="+t.TempDir(),
		"HOME="+project, "XDG_CONFIG_HOME="+filepath.Join(project, ".config"),
		"MISE_TRUSTED_CONFIG_PATHS="+project, "MISE_CONFIG_DIR="+filepath.Join(project, ".config", "mise"),
		"MISE_CEILING_PATHS="+filepath.Dir(project),
		"MISE_GLOBAL_CONFIG_FILE="+filepath.Join(project, ".config", "mise", "config.toml"), "MISE_SYSTEM_CONFIG_DIR="+filepath.Join(project, "system"),
		"MISE_GITHUB_CREDENTIAL_COMMAND=", "MISE_GITLAB_CREDENTIAL_COMMAND=",
		"MISE_GITHUB_GH_CLI_TOKENS=false", "MISE_GITLAB_GLAB_CLI_TOKENS=false",
		"MISE_HTTP_RETRIES=0", "MISE_HTTP_TIMEOUT=2s", "MISE_USE_VERSIONS_HOST=false",
		"AIGW_TOOL_SOURCE=peer", "CI_API_V4_URL="+server.URL+"/api/v4", "CI_PROJECT_ID=456",
		"CI_SERVER_HOST="+strings.TrimPrefix(server.URL, "http://"), "CI_JOB_ID=123", "CI_JOB_TOKEN=fixture-only",
		"HTTP_PROXY="+server.URL, "HTTPS_PROXY="+server.URL, "ALL_PROXY="+server.URL,
		"NO_PROXY=127.0.0.1", "http_proxy="+server.URL, "https_proxy="+server.URL, "all_proxy="+server.URL, "no_proxy=127.0.0.1",
	)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "sh", "-eu", "-c", cleanup)
		command.Dir, command.Env = project, environment
		if output, err := command.CombinedOutput(); err != nil {
			t.Errorf("native mirror cleanup: %v\n%s", err, output)
		}
		if _, err := os.Stat(filepath.Join(project, "build", "tmp", ".aigw-mise-mirror-123")); !os.IsNotExist(err) {
			t.Errorf("owned mirror remains: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "sh", "-eu", "-c", prepare+"\ntest \"$MISE_DATA_DIR\" = \"$mirror_dir/mise-data\"\nexec mise install --locked github:anchore/syft --jobs=1")
	command.Dir, command.Env, command.WaitDelay = project, environment, time.Second
	output, err := command.CombinedOutput()
	if err == nil || ctx.Err() != nil {
		t.Errorf("missing mirror must fail without expiry: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "404 Not Found") || strings.Contains(string(output), "mise WARN") {
		t.Errorf("missing mirror must report 404 without warnings: %s", output)
	}
	mutex.Lock()
	defer mutex.Unlock()
	for _, request := range requests {
		method, target, _ := strings.Cut(request, " ")
		if (method != http.MethodGet && method != http.MethodHead) || !strings.HasPrefix(target, strings.TrimPrefix(server.URL, "http://")+mirrorPath) {
			t.Errorf("native acquisition escaped selected peer: %s", request)
		}
	}
	for _, upstream := range []string{selected.URL, selected.URLAPI} {
		parsed, err := url.Parse(upstream)
		if err != nil {
			t.Fatalf("unexpected native locked URL: %q, %v", upstream, err)
		}
		want := strings.TrimPrefix(server.URL, "http://") + strings.TrimSuffix(mirrorPath, "/") + parsed.Path
		if !slices.ContainsFunc(requests, func(request string) bool { _, target, _ := strings.Cut(request, " "); return target == want }) {
			t.Errorf("native locked request was not observed: %s in %v", want, requests)
		}
	}
}

func unixMiseMirrorCommands(t *testing.T) (string, string) {
	t.Helper()
	output, err := projectionCommand(filepath.Clean(filepath.Join("..", "..", "..")), "{prepare: miseMirror.unixPrepare, cleanup: miseMirror.unixCleanup}").Output()
	if err != nil {
		t.Fatal(err)
	}
	var commands struct {
		Prepare string `yaml:"prepare"`
		Cleanup string `yaml:"cleanup"`
	}
	if err := yaml.Unmarshal(output, &commands); err != nil {
		t.Fatal(err)
	}
	return commands.Prepare, commands.Cleanup
}

func nativeMiseMirrorInputs(t *testing.T, repository string) (string, misePlatformLock) {
	t.Helper()
	project := t.TempDir()
	var lock miseLock
	for _, name := range []string{"mise.toml", "mise.lock"} {
		content, err := os.ReadFile(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "mise.lock" {
			if err := toml.Unmarshal(content, &lock); err != nil {
				t.Fatal(err)
			}
		}
		target := filepath.Join(project, name)
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	versions := lock.Tools["github:anchore/syft"]
	selected := map[string]misePlatformLock{
		"darwin-arm64": versions[0].MacOSARM64, "darwin-amd64": versions[0].MacOSX64,
		"linux-arm64": versions[0].LinuxARM64, "linux-amd64": versions[0].LinuxX64,
	}[runtime.GOOS+"-"+runtime.GOARCH]
	return project, selected
}

func lockedMiseMirrorPath(t *testing.T, repository string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repository, "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("packages/generic/mise-github/%x/", sha256.Sum256(content))
}
