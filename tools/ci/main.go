// Command ci owns portable quality execution and CI projection reconciliation.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

type command struct {
	Name string
	Args []string
}

type commandRunner func(command) error

var sourceCommands = []command{
	{Name: "go", Args: []string{"run", "./tools/ci", "project", "--check"}},
	{Name: "openspec", Args: []string{"validate", "--all", "--strict", "--no-interactive"}},
	{Name: "go", Args: []string{"run", "./tools/ci", "links", "."}},
	{Name: "gitleaks", Args: []string{"git", "--redact", "--no-banner", "."}},
	{Name: "go", Args: []string{"run", "./tools/release", "validate-toolchain", "go.mod"}},
	{Name: "go", Args: []string{"run", "./tools/release", "validate-release-sources"}},
	{Name: "go", Args: []string{"run", "./tools/architecture", "--root", "."}},
	{Name: "go", Args: []string{"test", "./tools/architecture"}},
	{Name: "go", Args: []string{"run", "./tools/coverage", "--race"}},
	{Name: "go", Args: []string{"vet", "./..."}},
	{Name: "go", Args: []string{"tool", "staticcheck", "-checks=SA*,S1*", "./..."}},
	{Name: "go", Args: []string{"tool", "errcheck", "./..."}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "go-format"}},
	{Name: "go", Args: []string{"test", "./tools/repository"}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "protected-lifecycle"}},
	{Name: "go", Args: []string{"test", "./internal/upgrade", "./tools/release"}},
	{Name: "actionlint"},
	{Name: "go", Args: []string{"test", "./tools/forge"}},
}

func configuredStaticCommands() []command {
	commands := make([]command, 0, len(sourceCommands)-1)
	for _, call := range sourceCommands {
		if call.Name == "go" && slices.Equal(call.Args, []string{"run", "./tools/coverage", "--race"}) {
			continue
		}
		commands = append(commands, call)
	}
	return commands
}

func main() {
	if err := run(os.Args[1:], os.Stdout, systemRunner); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, runner commandRunner) error {
	if len(args) == 0 {
		return errors.New("usage: ci <project|source|static|links|native|trust-input|fetch-tags>")
	}
	switch args[0] {
	case "static":
		if len(args) != 1 {
			return errors.New("usage: ci static")
		}
		return runCommands(configuredStaticCommands(), stdout, runner)
	case "source":
		if len(args) != 1 {
			return errors.New("usage: ci source")
		}
		commands, err := configuredSourceCommands()
		if err != nil {
			return err
		}
		return runCommands(commands, stdout, runner)
	case "project":
		flags := flag.NewFlagSet("ci project", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		check := flags.Bool("check", false, "verify tracked projections without writing")
		root := flags.String("root", ".", "repository root")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
			return errors.New("usage: ci project [--check] [--root <path>]")
		}
		projections, err := renderProjections(*root)
		if err != nil {
			return err
		}
		return reconcileProjections(*root, projections, !*check)
	case "links":
		if len(args) != 2 {
			return errors.New("usage: ci links <root>")
		}
		markdown, err := trackedMarkdown(args[1])
		if err != nil {
			return err
		}
		call := command{Name: "lychee", Args: append(
			[]string{"--offline", "--no-progress", "--cache=false", "--"}, markdown...,
		)}
		return runner(call)
	case "native":
		flags := flag.NewFlagSet("ci native", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		platform := flags.String("platform", "", "darwin, linux, or windows")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || !supportedNativePlatform(*platform) {
			return errors.New("usage: ci native --platform <darwin|linux|windows>")
		}
		if *platform != runtime.GOOS {
			return fmt.Errorf("native acceptance requires %s host, running on %s", *platform, runtime.GOOS)
		}
		return runCommands(nativeCommands(*platform), stdout, runner)
	case "trust-input":
		flags := flag.NewFlagSet("ci trust-input", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		output := flags.String("output", "", "allowed signers output")
		githubEnvironment := flags.String("github-env", "", "GitHub environment file")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *output == "" || *githubEnvironment == "" {
			return errors.New("usage: ci trust-input --output <path> --github-env <path>")
		}
		return writeTrustInput(*output, *githubEnvironment)
	case "fetch-tags":
		if len(args) != 1 {
			return errors.New("usage: ci fetch-tags")
		}
		return fetchTags(runner)
	default:
		return fmt.Errorf("unknown ci command: %s", args[0])
	}
}

func trackedMarkdown(root string) ([]string, error) {
	output, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.md").Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked Markdown: %w", err)
	}
	fields := bytes.Split(bytes.TrimSuffix(output, []byte{0}), []byte{0})
	markdown := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) != 0 {
			markdown = append(markdown, filepath.Join(root, string(field)))
		}
	}
	if len(markdown) == 0 {
		return nil, errors.New("repository contains no tracked Markdown")
	}
	return markdown, nil
}

type projection struct {
	Path    string
	Content string
}

var projectionExpressions = []struct {
	path       string
	expression string
}{
	{path: ".gitlab-ci.yml", expression: "gitlab"},
	{path: ".github/workflows/verify.yml", expression: "githubVerify"},
	{path: ".github/workflows/release.yml", expression: "githubRelease"},
}

func renderProjections(root string) ([]projection, error) {
	projections := make([]projection, 0, len(projectionExpressions))
	for _, item := range projectionExpressions {
		process := projectionCommand(root, item.expression)
		output, err := process.Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return nil, fmt.Errorf("render %s: %s", item.path, bytes.TrimSpace(exit.Stderr))
			}
			return nil, fmt.Errorf("render %s: %w", item.path, err)
		}
		projections = append(projections, projection{Path: item.path, Content: string(output)})
	}
	return projections, nil
}

func projectionCommand(root, expression string) *exec.Cmd {
	process := exec.Command(
		"cue",
		"export",
		filepath.FromSlash(".config/ci/pipeline.cue"),
		"--expression",
		expression,
		"--out",
		"yaml",
	)
	process.Dir = root
	return process
}

func reconcileProjections(root string, projections []projection, write bool) error {
	for _, item := range projections {
		path, err := projectionPath(root, item.Path)
		if err != nil {
			return err
		}
		if write {
			if err := writeProjection(path, []byte(item.Content)); err != nil {
				return err
			}
			continue
		}
		tracked, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read projection %s: %w", item.Path, err)
		}
		if !bytes.Equal(tracked, []byte(item.Content)) {
			return fmt.Errorf("projection drift: %s; run `mise exec --locked -- go run ./tools/ci project`", item.Path)
		}
	}
	return nil
}

func projectionPath(root, relative string) (string, error) {
	if relative == "" || strings.Contains(relative, `\`) || path.IsAbs(relative) || path.Clean(relative) != relative || relative == ".." || strings.HasPrefix(relative, "../") || hasWindowsVolume(relative) {
		return "", fmt.Errorf("invalid projection path %q", relative)
	}
	return filepath.Join(root, filepath.FromSlash(relative)), nil
}

func hasWindowsVolume(relative string) bool {
	return len(relative) >= 2 && relative[1] == ':' && ((relative[0] >= 'A' && relative[0] <= 'Z') || (relative[0] >= 'a' && relative[0] <= 'z'))
}

func writeProjection(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create projection directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write projection: %w", err)
	}
	return nil
}

func configuredSourceCommands() ([]command, error) {
	commands := append([]command(nil), sourceCommands...)
	provider := os.Getenv("AIGW_FORGE_PROVIDER")
	platform := provider
	if platform == "" {
		platform = "gitlab"
	}
	if platform != "gitlab" && platform != "github" {
		return nil, errors.New("AIGW_FORGE_PROVIDER must be gitlab or github")
	}
	for index, call := range commands {
		if call.Name == "gitleaks" {
			args := append([]string(nil), call.Args...)
			args = append(args[:len(args)-1], "--platform", platform, args[len(args)-1])
			commands[index] = command{Name: call.Name, Args: args}
		}
	}
	if provider == "" {
		return commands, nil
	}
	email := os.Getenv("AIGW_RELEASE_AUTHOR_EMAIL")
	signers := os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")
	if email == "" || signers == "" {
		return nil, errors.New("Forge verification requires author email and allowed signers file")
	}
	provenance := command{Name: "go", Args: []string{"run", "./tools/forge", "commits", "--provider", provider, "--email", email, "--allowed-signers", signers}}
	commands = append([]command{provenance}, commands...)
	return commands, nil
}

func writeTrustInput(output, githubEnvironment string) error {
	allowed := os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS")
	if allowed == "" {
		return errors.New("AIGW_RELEASE_ALLOWED_SIGNERS is required")
	}
	if err := os.WriteFile(output, []byte(allowed+"\n"), 0o600); err != nil {
		return fmt.Errorf("write trust input: %w", err)
	}
	file, err := os.OpenFile(githubEnvironment, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub environment: %w", err)
	}
	defer func() { _ = file.Close() }()
	_, err = fmt.Fprintf(file, "AIGW_GITHUB_ALLOWED_SIGNERS=%s\n", output)
	return err
}

func fetchTags(runner commandRunner) error {
	call := command{Name: "git", Args: []string{"fetch", "--force", "--tags", "origin"}}
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = runner(call); err == nil {
			return nil
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Millisecond)
		}
	}
	return fmt.Errorf("fetch annotated release tags: %w", err)
}

func runCommands(commands []command, stdout io.Writer, runner commandRunner) error {
	for _, call := range commands {
		_, _ = fmt.Fprintf(stdout, "==> %s\n", call.Name)
		if err := runner(call); err != nil {
			return fmt.Errorf("%s: %w", call.Name, err)
		}
	}
	return nil
}

func supportedNativePlatform(platform string) bool {
	return platform == "darwin" || platform == "linux" || platform == "windows"
}

func nativeCommands(platform string) []command {
	binary := filepath.Join("build", "acceptance", "aigw")
	installed := filepath.Join("build", "acceptance", "installed", "aigw")
	profile := filepath.Join("build", "acceptance", "coverage-"+platform+".out")
	if platform == "windows" {
		binary += ".exe"
		installed += ".exe"
	}
	return []command{
		{Name: "go", Args: []string{"run", "./tools/coverage", "--race", "--profile-output", profile}},
		{Name: "go", Args: []string{"vet", "./..."}},
		{Name: "go", Args: []string{"build", "-o", binary, "./cmd/aigw"}},
		{Name: binary, Args: []string{"--version"}},
		{Name: binary, Args: []string{"install", "--target", installed}},
		{Name: installed, Args: []string{"--version"}},
		{Name: binary, Args: []string{"uninstall", "--target", installed}},
	}
}

func systemRunner(call command) error {
	if err := os.MkdirAll(filepath.Join("build", "acceptance"), 0o755); err != nil {
		return err
	}
	process := exec.Command(call.Name, call.Args...)
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	return process.Run()
}
