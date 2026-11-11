package projection

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

var (
	ipv4Literal        = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	numericProjectPath = regexp.MustCompile(`projects/[0-9]+`)
)

// assertPreludeNamesNoInstance fails when a mirror prelude names its GitLab
// instance or project other than through CI variables, or reads $HOME/.netrc.
func assertPreludeNamesNoInstance(t *testing.T, name, prelude string) {
	t.Helper()
	for _, literal := range []*regexp.Regexp{ipv4Literal, numericProjectPath} {
		if found := literal.FindString(prelude); found != "" {
			t.Errorf("GitLab %s mirror prelude hard-codes %q", name, found)
		}
	}
	if strings.Contains(prelude, "$HOME/.netrc") {
		t.Errorf("GitLab %s mirror prelude reads $HOME/.netrc", name)
	}
}

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
		for _, required := range []string{"CI_API_V4_URL", "CI_PROJECT_ID", "CI_SERVER_HOST", "CI_JOB_TOKEN", "MISE_NETRC_FILE", "$mirror_dir/mise-data", "$MISE_DATA_DIR/cache", "github.com/", "api.github.com/", "packages/generic/mise-github/", ".config/mise/mise.lock", "lock_digest", "CI_JOB_ID"} {
			if !strings.Contains(prelude, required) {
				t.Errorf("GitLab %s mirror prelude omits %q", name, required)
			}
		}
		metadata := strings.Index(prelude, "regex:^https://api[.]github[.]com/repos/")
		fallback := strings.Index(prelude, `"https://api.github.com/"`)
		if metadata < 0 || fallback <= metadata || !strings.Contains(prelude, "release-$1-$2-$3.json") {
			t.Errorf("GitLab %s must resolve mirrored release metadata before the API fallback", name)
		}
		assertPreludeNamesNoInstance(t, name, prelude)
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
	for _, projection := range projections[1:] {
		for _, forbidden := range []string{"packages/generic/mise-github/", "MISE_URL_REPLACEMENTS"} {
			if strings.Contains(projection.Content, forbidden) {
				t.Fatalf("GitHub projection %s depends on a peer tool copy: %s", projection.Path, forbidden)
			}
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
			"CI_PROJECT_ID=101",
			"CI_SERVER_HOST=gitlab.example.invalid",
			"CI_JOB_ID=202",
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
		mirror := filepath.Join(project, "build", "tmp", ".aigw-mise-mirror-202")
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
	for _, name := range []string{".config/mise/config.toml", ".config/mise/mise.lock"} {
		content, err := os.ReadFile(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == ".config/mise/mise.lock" {
			if err := toml.Unmarshal(content, &lock); err != nil {
				t.Fatal(err)
			}
		}
		target := filepath.Join(project, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
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
	content, err := os.ReadFile(filepath.Join(repository, ".config", "mise", "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("packages/generic/mise-github/%x/", sha256.Sum256(content))
}
