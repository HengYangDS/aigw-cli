// Command ci owns CI projection contracts and portable source acceptance.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"
)

type command struct {
	Name string
	Args []string
}

type commandRunner func(command) error

var sourceCommands = []command{
	{Name: "go", Args: []string{"run", "./tools/ci", "toolchain", "."}},
	{Name: "go", Args: []string{"run", "./tools/release", "validate-toolchain", "go.mod"}},
	{Name: "go", Args: []string{"run", "./tools/release", "validate-release-sources"}},
	{Name: "go", Args: []string{"run", "./tools/architecture", "--root", "."}},
	{Name: "go", Args: []string{"test", "./tools/architecture"}},
	{Name: "go", Args: []string{"run", "./tools/coverage", "--race"}},
	{Name: "go", Args: []string{"vet", "./..."}},
	{Name: "go", Args: []string{"tool", "staticcheck", "-checks=all,-ST1000,-ST1005", "./..."}},
	{Name: "go", Args: []string{"tool", "errcheck", "./..."}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "go-format"}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "product-surface"}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "english-text"}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "credentials"}},
	{Name: "go", Args: []string{"test", "./tools/repository"}},
	{Name: "go", Args: []string{"run", "./tools/repository", "--root", ".", "governance"}},
	{Name: "go", Args: []string{"test", "./internal/upgrade", "./tools/release"}},
	{Name: "go", Args: []string{"run", "./tools/ci", "pipeline", "."}},
	{Name: "go", Args: []string{"run", "./tools/ci", "github-verify", "."}},
	{Name: "go", Args: []string{"run", "./tools/ci", "github-release", "."}},
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
		return errors.New("usage: ci <source|static|native|trust-input|fetch-tags|toolchain|proxy-policy|github-verify|github-release|pipeline>")
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
	case "toolchain", "proxy-policy", "github-verify", "github-release", "pipeline":
		return runContract(args)
	default:
		return fmt.Errorf("unknown ci command: %s", args[0])
	}
}

func configuredSourceCommands() ([]command, error) {
	commands := append([]command(nil), sourceCommands...)
	provider := os.Getenv("AIGW_FORGE_PROVIDER")
	if provider == "" {
		return commands, nil
	}
	if provider != "gitlab" && provider != "github" {
		return nil, errors.New("AIGW_FORGE_PROVIDER must be gitlab or github")
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
	if platform == "windows" {
		binary += ".exe"
		installed += ".exe"
	}
	return []command{
		{Name: "go", Args: []string{"run", "./tools/coverage", "--race"}},
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
