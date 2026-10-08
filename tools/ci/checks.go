package main

import (
	nativeprocess "aigw-cli/internal/process"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"aigw-cli/tools/ci/markdown"
)

func currentRepositoryFiles(root, kind string, patterns ...string) ([]string, error) {
	arguments := []string{
		"-C", root, "--git-dir", ".git", "--work-tree", ".",
		"ls-files", "-z", "--cached", "--others",
		"--exclude-standard", "--deduplicate", "--",
	}
	process := exec.Command(
		"git", append(arguments, patterns...)...,
	)
	// The explicit checkout, not an inherited alternate index, owns this scope.
	process.Env = slices.DeleteFunc(process.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(name, "GIT_INDEX_FILE")
	})
	output, err := process.Output()
	if err != nil {
		return nil, fmt.Errorf("list repository %s: %w", kind, err)
	}
	fields := bytes.Split(bytes.TrimSuffix(output, []byte{0}), []byte{0})
	files := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) == 0 {
			continue
		}
		filename := filepath.Join(root, string(field))
		info, statErr := os.Stat(filename)
		switch {
		case statErr == nil && !info.IsDir():
			files = append(files, filename)
		case errors.Is(statErr, os.ErrNotExist):
			continue
		case statErr != nil:
			return nil, fmt.Errorf("inspect repository %s %s: %w", kind, field, statErr)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("repository contains no current %s", kind)
	}
	slices.Sort(files)
	return files, nil
}

func checkGo(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "Go source", "*.go")
	if err != nil {
		return err
	}
	config := filepath.Join(root, ".config", "checks", "go", "policy.yml")
	if err := runner(command{Name: "golangci-lint", Dir: root, Args: []string{
		"fmt", "--diff", "--config", config,
	}}); err != nil {
		return err
	}
	packages := make(map[string]struct{})
	for _, file := range files {
		if !goToolSource(root, file) {
			continue
		}
		packages[filepath.Dir(file)] = struct{}{}
	}
	if len(packages) == 0 {
		return errors.New("repository contains no current Go package source")
	}
	return runner(command{Name: "golangci-lint", Dir: root, Args: append(
		[]string{"run", "--config", config, "--"}, slices.Sorted(maps.Keys(packages))...,
	)})
}

func goToolSource(root, file string) bool {
	relative, err := filepath.Rel(root, file)
	if err != nil {
		return false
	}
	for component := range strings.SplitSeq(filepath.ToSlash(relative), "/") {
		if strings.HasPrefix(component, ".") || strings.HasPrefix(component, "_") {
			return false
		}
	}
	return true
}

func checkLinks(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "Markdown", "*.md")
	if err != nil {
		return err
	}
	if err := requireTrackedLinkTargets(root, files); err != nil {
		return err
	}
	return runner(command{Name: "lychee", Dir: root, Args: []string{
		"--offline", "--include-fragments=anchor-only", "--no-progress", "--cache=false", "--files-from", "-",
	}, Input: strings.Join(files, "\n") + "\n"})
}

func requireTrackedLinkTargets(root string, files []string) error {
	localRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	process := exec.Command("git", "-C", root, "--git-dir", ".git", "--work-tree", ".", "ls-files", "--cached", "-z")
	process.Env = slices.DeleteFunc(process.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(name, "GIT_INDEX_FILE")
	})
	index, err := process.Output()
	if err != nil {
		return fmt.Errorf("read link target index: %w", err)
	}
	tracked := map[string]bool{".": true}
	for file := range strings.SplitSeq(string(index), "\x00") {
		if file == "" {
			continue
		}
		for name := filepath.FromSlash(file); name != "."; name = filepath.Dir(name) {
			tracked[name] = true
		}
	}
	process = exec.Command("lychee", "--dump", "--offline", "--no-progress", "--scheme", "file", "--files-from", "-")
	process.Dir = root
	process.Stdin = strings.NewReader(strings.Join(files, "\n") + "\n")
	links, err := process.Output()
	if err != nil {
		return fmt.Errorf("extract repository links: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for link := range strings.SplitSeq(strings.TrimSpace(string(links)), "\n") {
		if link == "" {
			continue
		}
		target, err := url.Parse(link)
		if err != nil {
			return fmt.Errorf("parse extracted link: %w", err)
		}
		file := filepath.FromSlash(target.Path)
		if strings.HasPrefix(file, string(filepath.Separator)) && filepath.VolumeName(file[1:]) != "" {
			file = file[1:]
		}
		relative, err := filepath.Rel(localRoot, file)
		if err != nil || target.Host != "" || !tracked[relative] {
			return fmt.Errorf("link target is not Git-tracked repository content: %s; link to committed content or a published artifact", link)
		}
		resolved, err := filepath.EvalSymlinks(file)
		if err != nil {
			return fmt.Errorf("resolve link target %s: %w", link, err)
		}
		relative, err = filepath.Rel(root, resolved)
		if err != nil || !tracked[relative] {
			return fmt.Errorf("link target is not Git-tracked repository content: %s; link to committed content or a published artifact", link)
		}
	}
	return nil
}

func formatSource(root string, runner commandRunner, write bool) error {
	files, err := currentRepositoryFiles(root, "authored files")
	if err != nil {
		return err
	}
	for index, path := range files {
		files[index], err = filepath.Rel(root, path)
		if err != nil {
			return err
		}
	}
	input, err := json.Marshal(files)
	if err != nil {
		return err
	}
	checker := filepath.Join(root, "tools", "ci", "format.mjs")
	arguments := []string{checker}
	if write {
		arguments = append(arguments, "--write")
	}
	return runner(command{Name: "node", Dir: root, Args: arguments, Input: string(input)})
}

func checkMarkdown(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "Markdown", "*.md", "*.markdown", ":(exclude)openspec/changes/archive/**")
	if err != nil {
		return err
	}
	if err := markdown.CheckPolicy(root); err != nil {
		return err
	}
	input, err := json.Marshal(files)
	if err != nil {
		return err
	}
	checker := filepath.Join(root, "tools", "ci", "markdown", "lint.mjs")
	return runner(command{Name: "node", Dir: root, Args: []string{checker}, Input: string(input)})
}

func checkMermaid(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "diagram sources", "*.md", "*.mdx", "*.markdown", "*.mmd")
	if err != nil {
		return err
	}
	input, err := json.Marshal(files)
	if err != nil {
		return err
	}
	checker := filepath.Join(root, "tools", "ci", "markdown", "diagrams.mjs")
	return runner(command{Name: "node", Dir: root, Args: []string{checker}, Input: string(input)})
}

func checkSpelling(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "authored files")
	if err != nil {
		return err
	}
	for index, file := range files {
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return fmt.Errorf("resolve spelling input relative to checkout: %w", err)
		}
		files[index] = filepath.ToSlash(relative)
	}
	return runner(command{
		Name:  "typos",
		Dir:   root,
		Args:  []string{"--config", ".config/checks/spelling/policy.toml", "--force-exclude", "--file-list", "-"},
		Input: strings.Join(files, "\n") + "\n",
	})
}

func checkTOML(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "TOML", "*.toml", "mise.lock")
	if err != nil {
		return err
	}
	for index, path := range files {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve TOML input relative to checkout: %w", err)
		}
		files[index] = relative
	}
	config := filepath.Join(root, ".config", "checks", "toml", "policy.toml")
	for _, arguments := range [][]string{
		{"lint", "--config", config, "--no-schema", "--"},
		{"format", "--config", config, "--check", "--diff", "--"},
	} {
		if err := runner(command{Name: "taplo", Dir: root, Args: append(arguments, files...), Env: []string{"RUST_LOG=warn"}}); err != nil {
			return err
		}
	}
	return nil
}

func checkSecrets(root string, runner commandRunner) (err error) {
	files, err := currentRepositoryFiles(root, "authored files")
	if err != nil {
		return err
	}
	source, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	parent := filepath.Join(root, "build", "tmp")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	projection, err := os.MkdirTemp(parent, ".secret-scan-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(projection)) }()
	for _, file := range files {
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		info, err := source.Lstat(relative)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		content, err := source.ReadFile(relative)
		if err != nil {
			return err
		}
		destination := filepath.Join(projection, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(destination, content, 0o600); err != nil {
			return err
		}
	}
	return runner(command{Name: "gitleaks", Dir: projection, Args: []string{
		"dir", "--config", ".config/checks/secrets/policy.toml",
		"--redact", "--no-banner", "--log-level", "warn", ".",
	}})
}

func checkWorkflows(root string, runner commandRunner) error {
	files, err := currentRepositoryFiles(root, "GitHub workflows", ".github/workflows/*.yml", ".github/workflows/*.yaml")
	if err != nil {
		return err
	}
	if err := runner(command{Name: "actionlint", Args: append([]string{"-shellcheck="}, files...), Dir: root}); err != nil {
		return err
	}
	parent := filepath.Join(root, "build", "verification", "workflows")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	evidence, err := os.MkdirTemp(parent, "check-")
	if err != nil {
		return err
	}
	records := make([]workflowScript, 0)
	for _, file := range files {
		scripts, err := workflowScripts(file)
		if err != nil {
			return err
		}
		records = append(records, scripts...)
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidence, "scripts.json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	for index, script := range records {
		if script.Shell != "bash" && script.Shell != "sh" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		stdout, stderr, runErr := (nativeprocess.Runner{}).RunCaptureStreams(ctx, nativeprocess.Plan{Executable: "shellcheck", Directory: root, Env: os.Environ(), Args: []string{"--norc", "-f", "json", "-x", "--shell", script.Shell, "-e", "SC1091,SC2194,SC2050,SC2153,SC2154,SC2157,SC2043", "-"}, Stdin: script.Input})
		cancel()
		stem := filepath.Join(evidence, fmt.Sprintf("script-%d", index))
		if err := os.WriteFile(stem+".json", stdout, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(stem+".stderr", stderr, 0o600); err != nil {
			return err
		}
		var findings []json.RawMessage
		if err := json.Unmarshal(stdout, &findings); err != nil {
			return fmt.Errorf("native shell validation for %s: %w", script.File, err)
		}
		if runErr != nil || findings == nil || len(findings) > 0 || nativeprocess.DiagnosticFailure(stderr) {
			return fmt.Errorf("native ShellCheck refused %s job %s step %d; evidence %s: %w", script.File, script.Job, script.Step, stem, errors.Join(runErr, errors.New("shell validation is incomplete or has findings")))
		}
	}
	return nil
}

type workflowScript struct {
	File   string `json:"file"`
	Job    string `json:"job"`
	Shell  string `json:"shell"`
	SHA256 string `json:"sha256"`
	Step   int    `json:"step"`
	Bytes  int    `json:"bytes"`
	Input  string `json:"-"`
}

func workflowScripts(path string) ([]workflowScript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var workflow struct {
		Defaults struct {
			Run struct {
				Shell string `yaml:"shell"`
			} `yaml:"run"`
		} `yaml:"defaults"`
		Jobs map[string]struct {
			Defaults struct {
				Run struct {
					Shell string `yaml:"shell"`
				} `yaml:"run"`
			} `yaml:"defaults"`
			Steps []struct{ Run, Shell string } `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("parse workflow %s: %w", path, err)
	}
	jobs := slices.Sorted(maps.Keys(workflow.Jobs))
	var scripts []workflowScript
	for _, name := range jobs {
		job := workflow.Jobs[name]
		for index, step := range job.Steps {
			if step.Run == "" {
				continue
			}
			shell := step.Shell
			if shell == "" {
				shell = job.Defaults.Run.Shell
			}
			if shell == "" {
				shell = workflow.Defaults.Run.Shell
			}
			fields := strings.Fields(shell)
			if len(fields) == 0 || strings.Contains(shell, "${{") || !slices.Contains([]string{"bash", "sh", "pwsh", "powershell"}, fields[0]) {
				return nil, fmt.Errorf("workflow %s job %s step %d requires an explicit supported shell", path, name, index)
			}
			input, err := workflowShellInput(step.Run, fields[0])
			if err != nil {
				return nil, err
			}
			scripts = append(scripts, workflowScript{File: path, Job: name, Step: index, Shell: fields[0], SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(step.Run))), Bytes: len(step.Run), Input: input})
		}
	}
	return scripts, nil
}

func workflowShellInput(script, shell string) (string, error) {
	var input strings.Builder
	for {
		start := strings.Index(script, "${{")
		if start < 0 {
			input.WriteString(script)
			break
		}
		input.WriteString(script[:start])
		end := strings.Index(script[start:], "}}")
		if end < 0 {
			return "", errors.New("workflow shell expression is unterminated")
		}
		end += start + 2
		input.WriteString(strings.Repeat("_", end-start))
		script = script[end:]
	}
	setup := "set -e"
	if shell == "bash" {
		setup = "set -eo pipefail"
	}
	return setup + "\n" + input.String() + "\n", nil
}
