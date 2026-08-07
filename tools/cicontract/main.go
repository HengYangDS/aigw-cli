// Command cicontract validates repository-owned CI projections.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

var immutableAction = regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)

type gateConfig struct {
	Toolchain struct {
		GoMod        string `toml:"go_mod"`
		GitHubCache  bool   `toml:"github_setup_go_cache"`
		GitHubAction struct {
			Checkout string `toml:"checkout"`
			SetupGo  string `toml:"setup_go"`
		} `toml:"github_actions"`
	} `toml:"toolchain"`
	Common commandSet `toml:"common"`
	GitLab struct {
		RunnerTagVariable string           `toml:"runner_tag_variable"`
		GitDepth          string           `toml:"git_depth"`
		GoProxyFallback   string           `toml:"goproxy_fallback"`
		PrepareCache      string           `toml:"prepare_cache_script"`
		SuppressBranch    bool             `toml:"suppress_untagged_release_branch"`
		SuppressMR        bool             `toml:"suppress_release_branch_merge_request"`
		ForbidWindows     bool             `toml:"forbid_windows_native_acceptance_job"`
		ForbidMacOS       bool             `toml:"forbid_macos_native_acceptance_job"`
		ForbidMirror      bool             `toml:"forbid_github_mirror_job"`
		Commands          requiredCommands `toml:"commands"`
		Package           requiredCommands `toml:"package"`
		Release           requiredCommands `toml:"release"`
	} `toml:"gitlab"`
	GitHub struct {
		Verify struct {
			Runner            string           `toml:"runner"`
			Permissions       string           `toml:"permissions"`
			ForbidPullRequest bool             `toml:"forbid_pull_request_triggers"`
			ForbidGoProxy     bool             `toml:"forbid_goproxy_policy"`
			ForbidWrite       bool             `toml:"forbid_write_permissions"`
			ForbidFloating    bool             `toml:"forbid_floating_actions"`
			Commands          requiredCommands `toml:"commands"`
			Active            commandSet       `toml:"active_script_commands"`
		} `toml:"verify"`
		Release struct {
			Runner      string           `toml:"runner"`
			Permissions string           `toml:"permissions"`
			Name        string           `toml:"name"`
			TagFilter   string           `toml:"tag_filter"`
			Needs       string           `toml:"needs"`
			Commands    requiredCommands `toml:"commands"`
			Forbid      struct {
				Tokens []string `toml:"tokens"`
			} `toml:"forbid"`
		} `toml:"release"`
	} `toml:"github"`
	Native struct {
		Linux   requiredCommands `toml:"linux"`
		Windows requiredCommands `toml:"windows"`
		MacOS   struct {
			Required []string `toml:"staging_commands"`
		} `toml:"macos"`
	} `toml:"native"`
}

type commandSet struct {
	Commands []string `toml:"commands"`
}

type requiredCommands struct {
	Required []string `toml:"required"`
}

type workflow struct {
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]job    `yaml:"jobs"`
}

type job struct {
	RunsOn          any              `yaml:"runs-on"`
	If              any              `yaml:"if"`
	ContinueOnError any              `yaml:"continue-on-error"`
	Steps           []map[string]any `yaml:"steps"`
}

func main() { os.Exit(execute(os.Args[1:], os.Stderr)) }

func execute(args []string, stderr *os.File) int {
	if err := run(args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cicontract <proxy-policy|github-verify|github-release|pipeline> [root]")
	}
	root := "."
	if len(args) > 1 {
		root = args[1]
	}
	switch args[0] {
	case "proxy-policy":
		return proxyPolicy(root)
	case "github-verify":
		return githubWorkflowContract(root, false)
	case "github-release":
		return githubWorkflowContract(root, true)
	case "pipeline":
		return pipelineContract(root)
	default:
		return fmt.Errorf("unknown cicontract command: %s", args[0])
	}
}

func read(root, relative string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", relative, err)
	}
	return string(data), nil
}

func proxyPolicy(root string) error {
	text, err := read(root, ".gitlab-ci.yml")
	if err != nil {
		return err
	}
	const expected = "${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}"
	if !strings.Contains(text, expected) {
		return errors.New("GitLab GOPROXY default must use a pipe-separated resilient fallback chain")
	}
	if strings.Contains(text, "${AIGW_GOPROXY:-https://goproxy.cn,direct}") {
		return errors.New("GitLab GOPROXY must not leave a comma-separated timeout-terminal fallback")
	}
	return nil
}

func loadGates(root string) (gateConfig, error) {
	data, err := os.ReadFile(filepath.Join(root, ".config/ci/verify-gates.toml"))
	if err != nil {
		return gateConfig{}, err
	}
	var gates gateConfig
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&gates); err != nil {
		return gateConfig{}, err
	}
	for name, action := range map[string]string{"checkout": gates.Toolchain.GitHubAction.Checkout, "setup-go": gates.Toolchain.GitHubAction.SetupGo} {
		if !immutableAction.MatchString(action) {
			return gateConfig{}, fmt.Errorf("GitHub Action %s must use an immutable commit", name)
		}
	}
	return gates, nil
}

func githubWorkflowContract(root string, release bool) error {
	gates, err := loadGates(root)
	if err != nil {
		return err
	}
	relative := ".github/workflows/verify.yml"
	runner := gates.GitHub.Verify.Runner
	required := append([]string{}, gates.Common.Commands...)
	permissions := gates.GitHub.Verify.Permissions
	if release {
		relative = ".github/workflows/release.yml"
		runner = gates.GitHub.Release.Runner
		required = append(required[:0], gates.GitHub.Release.Commands.Required...)
		permissions = gates.GitHub.Release.Permissions
	} else {
		required = append(required, gates.GitHub.Verify.Commands.Required...)
	}
	text, err := read(root, relative)
	if err != nil {
		return err
	}
	var document workflow
	if err := yaml.Unmarshal([]byte(text), &document); err != nil {
		return fmt.Errorf("parse %s: %w", relative, err)
	}
	if document.Permissions["contents"] != strings.TrimPrefix(permissions, "contents: ") {
		return fmt.Errorf("%s has incorrect repository permissions", relative)
	}
	matchedRunner := false
	for name, current := range document.Jobs {
		if fmt.Sprint(current.RunsOn) == runner {
			matchedRunner = true
		}
		if !release && name == "verify" {
			if current.If != nil || current.ContinueOnError != nil {
				return errors.New("GitHub verify job must remain blocking")
			}
			if err := activeCommands(current.Steps, gates.GitHub.Verify.Active.Commands); err != nil {
				return err
			}
		}
	}
	if !matchedRunner {
		return fmt.Errorf("%s does not use the configured runner", relative)
	}
	for _, token := range append(required, gates.Toolchain.GitHubAction.Checkout, gates.Toolchain.GitHubAction.SetupGo, "go-version-file: go.mod", "cache: false") {
		if !strings.Contains(text, token) {
			return fmt.Errorf("%s is missing %q", relative, token)
		}
	}
	for _, token := range []string{"@main", "@master", "runs-on: [self-hosted"} {
		if strings.Contains(text, token) {
			return fmt.Errorf("%s contains forbidden %q", relative, token)
		}
	}
	return nil
}

func activeCommands(steps []map[string]any, required []string) error {
	for _, step := range steps {
		if step["name"] != "Run source and policy gates" {
			continue
		}
		if step["if"] != nil || step["continue-on-error"] != nil {
			return errors.New("GitHub source gate step must remain blocking")
		}
		run, _ := step["run"].(string)
		for _, command := range required {
			if !containsLine(run, command) {
				return fmt.Errorf("GitHub source gates are missing active command: %s", command)
			}
		}
		return nil
	}
	return errors.New("GitHub source gate step is missing")
}

func containsLine(block, command string) bool {
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == command {
			return true
		}
	}
	return false
}

func pipelineContract(root string) error {
	gates, err := loadGates(root)
	if err != nil {
		return err
	}
	gitlab, err := read(root, ".gitlab-ci.yml")
	if err != nil {
		return err
	}
	verify, err := read(root, ".github/workflows/verify.yml")
	if err != nil {
		return err
	}
	release, err := read(root, ".github/workflows/release.yml")
	if err != nil {
		return err
	}
	if _, err := read(root, gates.Toolchain.GoMod); err != nil {
		return err
	}
	for _, token := range append(append([]string{}, gates.Common.Commands...), gates.GitLab.Commands.Required...) {
		if !strings.Contains(gitlab, token) {
			return fmt.Errorf("GitLab verification is missing %q", token)
		}
	}
	for _, token := range gates.GitLab.Package.Required {
		if !strings.Contains(gitlab, token) {
			return fmt.Errorf("GitLab package plane is missing %q", token)
		}
	}
	for _, token := range gates.GitLab.Release.Required {
		if !strings.Contains(gitlab, token) {
			return fmt.Errorf("GitLab release plane is missing %q", token)
		}
	}
	if gates.GitLab.SuppressBranch && !hasNeverRule(gitlab, `CI_COMMIT_BRANCH =~ /^release\//`) {
		return errors.New("GitLab must suppress untagged release branches")
	}
	if gates.GitLab.SuppressMR && !hasNeverRule(gitlab, `CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\//`) {
		return errors.New("GitLab must suppress release branch merge requests")
	}
	for _, forbidden := range []string{"allow_failure: true", "mirror-github:", "AIGW_GITHUB_MIRROR", "windows-native-acceptance:", "macos-native-acceptance:"} {
		if strings.Contains(gitlab, forbidden) {
			return fmt.Errorf("GitLab contains forbidden %q", forbidden)
		}
	}
	if err := githubWorkflowContract(root, false); err != nil {
		return err
	}
	if err := githubWorkflowContract(root, true); err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(release), "gitlab-ci") || containsLine(release, "sh scripts/release/publish/publish-gitlab-release.sh") {
		return errors.New("GitHub release plane depends on GitLab")
	}
	if strings.Contains(verify, "AIGW_GOPROXY") || strings.Contains(verify, "goproxy.cn") {
		return errors.New("GitHub verify inherited GitLab dependency policy")
	}
	return nil
}

func hasNeverRule(text, expression string) bool {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if strings.Contains(line, expression) && index+1 < len(lines) && strings.TrimSpace(lines[index+1]) == "when: never" {
			return true
		}
	}
	return false
}
