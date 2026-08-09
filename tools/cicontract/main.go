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
var immutableMiseImage = regexp.MustCompile(`^ghcr\.io/jdx/mise@sha256:[0-9a-f]{64}$`)

type gateConfig struct {
	Toolchain struct {
		GoMod        string `toml:"go_mod"`
		MiseConfig   string `toml:"mise_config"`
		MiseLock     string `toml:"mise_lock"`
		GitHubAction struct {
			Checkout string `toml:"checkout"`
			Mise     string `toml:"mise"`
		} `toml:"github_actions"`
	} `toml:"toolchain"`
	Common commandSet `toml:"common"`
	GitLab struct {
		BootstrapImage    string           `toml:"bootstrap_image"`
		RunnerTagVariable string           `toml:"runner_tag_variable"`
		GitDepth          string           `toml:"git_depth"`
		GoProxyFallback   string           `toml:"goproxy_fallback"`
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
		Darwin  requiredCommands `toml:"darwin"`
		Linux   requiredCommands `toml:"linux"`
		Windows requiredCommands `toml:"windows"`
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
		return errors.New("usage: cicontract <toolchain|proxy-policy|github-verify|github-release|pipeline> [root]")
	}
	root := "."
	if len(args) > 1 {
		root = args[1]
	}
	switch args[0] {
	case "toolchain":
		gates, err := loadGates(root)
		if err != nil {
			return err
		}
		return validateMiseProjection(root, gates)
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
	for name, action := range map[string]string{"checkout": gates.Toolchain.GitHubAction.Checkout, "mise": gates.Toolchain.GitHubAction.Mise} {
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
	native := append(append([]string{}, gates.Native.Darwin.Required...), gates.Native.Linux.Required...)
	native = append(native, gates.Native.Windows.Required...)
	if release {
		relative = ".github/workflows/release.yml"
		runner = gates.GitHub.Release.Runner
		required = append(required[:0], gates.GitHub.Release.Commands.Required...)
		permissions = gates.GitHub.Release.Permissions
	} else {
		required = append(required, gates.GitHub.Verify.Commands.Required...)
	}
	required = append(required, native...)
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
		for _, step := range current.Steps {
			if _, explicit := step["shell"]; explicit {
				return fmt.Errorf("%s must not select a workflow shell", relative)
			}
			if run, ok := step["run"].(string); ok && strings.Contains(run, "\n") {
				return fmt.Errorf("%s must invoke one portable owner per run step", relative)
			}
		}
		if err := validateMiseBootstrapOrder(current.Steps, gates.Toolchain.GitHubAction.Mise); err != nil {
			return fmt.Errorf("%s job %s: %w", relative, name, err)
		}
	}
	if !matchedRunner {
		return fmt.Errorf("%s does not use the configured runner", relative)
	}
	for _, token := range append(required, gates.Toolchain.GitHubAction.Checkout, gates.Toolchain.GitHubAction.Mise, "install: true", "cache: false") {
		if !strings.Contains(text, token) {
			return fmt.Errorf("%s is missing %q", relative, token)
		}
	}
	for _, token := range []string{
		`GIT_CONFIG_COUNT: "1"`,
		"GIT_CONFIG_KEY_0: init.defaultBranch",
		"GIT_CONFIG_VALUE_0: main",
	} {
		if !strings.Contains(text, token) {
			return fmt.Errorf("%s must configure Git's default branch without global state", relative)
		}
	}
	for _, token := range []string{"@main", "@master", "runs-on: [self-hosted"} {
		if strings.Contains(text, token) {
			return fmt.Errorf("%s contains forbidden %q", relative, token)
		}
	}
	return nil
}

func validateMiseBootstrapOrder(steps []map[string]any, action string) error {
	bootstrapped := false
	for _, step := range steps {
		if uses, _ := step["uses"].(string); uses == action {
			bootstrapped = true
			continue
		}
		run, _ := step["run"].(string)
		if strings.Contains(run, "mise exec") && !bootstrapped {
			return errors.New("owned command invokes mise before mise bootstrap")
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
		if step["shell"] != nil {
			return errors.New("GitHub source gate step must not select a shell")
		}
		run, _ := step["run"].(string)
		if strings.Contains(run, "\n") {
			return errors.New("GitHub source gate step must invoke one portable owner")
		}
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
	if err := validateMiseProjection(root, gates); err != nil {
		return err
	}
	if err := validateGitLabCommandProjection(root); err != nil {
		return err
	}
	if !immutableMiseImage.MatchString(gates.GitLab.BootstrapImage) {
		return errors.New("GitLab mise image must be the official immutable digest-pinned image")
	}
	if !hasGitLabDefaultImage(gitlab, gates.GitLab.BootstrapImage) {
		return errors.New("GitLab mise image does not match the toolchain SSOT")
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
	if strings.Contains(strings.ToLower(release), "gitlab-ci") || containsLine(release, "publish-gitlab") {
		return errors.New("GitHub release plane depends on GitLab")
	}
	if strings.Contains(verify, "AIGW_GOPROXY") || strings.Contains(verify, "goproxy.cn") {
		return errors.New("GitHub verify inherited GitLab dependency policy")
	}
	return nil
}

func validateMiseProjection(root string, gates gateConfig) error {
	if gates.Toolchain.MiseConfig == "" || gates.Toolchain.MiseLock == "" {
		return errors.New("toolchain SSOT must declare mise config and lock paths")
	}
	config, err := read(root, gates.Toolchain.MiseConfig)
	if err != nil {
		return err
	}
	lock, err := read(root, gates.Toolchain.MiseLock)
	if err != nil {
		return err
	}
	if !strings.Contains(config, "[tools]") {
		return errors.New("mise configuration must declare repository tools")
	}
	if !strings.Contains(lock, "[[tools.go]]") {
		return errors.New("mise lock must resolve the Go toolchain")
	}
	for _, relative := range []string{".gitlab-ci.yml", ".github/workflows/verify.yml", ".github/workflows/release.yml"} {
		text, err := read(root, relative)
		if err != nil {
			return err
		}
		lower := strings.ToLower(text)
		for _, token := range []string{"brew install", "apt-get install", "curl |", "curl -", "setup-go@"} {
			if strings.Contains(lower, token) {
				return fmt.Errorf("%s contains ad hoc tool bootstrap %q", relative, token)
			}
		}
		for _, line := range strings.Split(text, "\n") {
			if err := validateMiseCommand(strings.TrimSpace(strings.TrimPrefix(line, "-"))); err != nil {
				return fmt.Errorf("%s: %w", relative, err)
			}
		}
	}
	return nil
}

func validateMiseCommand(command string) error {
	fields := strings.Fields(command)
	if len(fields) < 2 || fields[0] != "mise" || (fields[1] != "exec" && fields[1] != "x") {
		return nil
	}
	for _, field := range fields[2:] {
		if field == "--locked" {
			return nil
		}
		if field == "--" {
			break
		}
	}
	return errors.New("repository command must use locked mise execution")
}

func hasGitLabDefaultImage(text, expected string) bool {
	var document map[string]any
	if err := yaml.Unmarshal([]byte(text), &document); err != nil {
		return false
	}
	defaults, ok := document["default"].(map[string]any)
	if !ok {
		return false
	}
	return fmt.Sprint(defaults["image"]) == expected
}

func validateGitLabCommandProjection(root string) error {
	data, err := os.ReadFile(filepath.Join(root, ".gitlab-ci.yml"))
	if err != nil {
		return err
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse .gitlab-ci.yml: %w", err)
	}
	if _, present := document["before_script"]; present {
		return errors.New("GitLab pipeline must not own a top-level before_script")
	}
	if defaults, ok := document["default"].(map[string]any); ok {
		if _, present := defaults["before_script"]; present {
			return errors.New("GitLab default must not own a before_script")
		}
	}
	for name, raw := range document {
		job, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, present := job["before_script"]; present {
			return fmt.Errorf("GitLab job %s must not own a before_script", name)
		}
		rawScript, present := job["script"]
		if !present {
			continue
		}
		var script []any
		switch value := rawScript.(type) {
		case []any:
			script = value
		case []string:
			for _, item := range value {
				script = append(script, item)
			}
		default:
			return fmt.Errorf("GitLab job %s script must be a command list", name)
		}
		for _, rawCommand := range script {
			command := fmt.Sprint(rawCommand)
			if strings.Contains(command, "\n") {
				return fmt.Errorf("GitLab job %s must invoke one portable owner per script item", name)
			}
		}
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
