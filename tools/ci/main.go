// Command ci owns portable quality execution and CI projection reconciliation.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"aigw-cli/tools/ci/markdown"
	"aigw-cli/tools/ci/projection"
	"aigw-cli/tools/release/readiness"
)

var qualityCommands = repositoryQualityGraph.commands(false)

func main() {
	if err := run(os.Args[1:], os.Stdout, systemRunner); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, runner commandRunner) error {
	if len(args) == 0 {
		return errors.New("usage: ci <project|source|quality|openspec|links|check-format|check-go|check-source-size|check-spelling|check-toml|check-markdown|check-markdown-policy|check-mermaid|check-secrets|native|trust-input>")
	}
	checks := map[string]func(string, commandRunner) error{
		"links": checkLinks, "check-go": checkGo,
		"check-toml": checkTOML, "check-secrets": checkSecrets,
		"check-format":          checkFormat,
		"check-source-size":     checkSourceSize,
		"check-spelling":        checkSpelling,
		"check-mermaid":         checkMermaid,
		"check-markdown":        checkMarkdown,
		"check-markdown-policy": func(root string, _ commandRunner) error { return markdown.CheckPolicy(root) },
	}
	if check := checks[args[0]]; check != nil {
		if len(args) != 2 {
			return fmt.Errorf("usage: ci %s <root>", args[0])
		}
		root, err := filepath.Abs(args[1])
		if err != nil {
			return err
		}
		return check(root, runner)
	}
	switch args[0] {
	case "quality", "source":
		if len(args) != 1 {
			return fmt.Errorf("usage: ci %s", args[0])
		}
		configured := func() ([]command, error) { return configuredQualityCommands(".") }
		if args[0] == "source" {
			configured = func() ([]command, error) { return configuredSourceCommands(".") }
		}
		commands, err := configured()
		if err != nil {
			return err
		}
		return runCommands(commands, stdout, runner)
	case "openspec":
		if len(args) != 1 {
			return errors.New("usage: ci openspec")
		}
		return runOpenSpecValidation(stdout, systemOutputRunner)
	case "project":
		flags := flag.NewFlagSet("ci project", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		check := flags.Bool("check", false, "verify tracked projections without writing")
		root := flags.String("root", ".", "repository root")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
			return errors.New("usage: ci project [--check] [--root <path>]")
		}
		return projection.Reconcile(*root, *check)
	case "native":
		return runNative(args[1:], stdout, runner)
	case "trust-input":
		flags := flag.NewFlagSet("ci trust-input", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		output := flags.String("output", "", "allowed signers output")
		githubEnvironment := flags.String("github-env", "", "GitHub environment file")
		artifact := flags.Bool("artifact", false, "materialize artifact-signature trust instead of Git source trust")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *output == "" || *githubEnvironment == "" {
			return errors.New("usage: ci trust-input [--artifact] --output <path> --github-env <path>")
		}
		variable := "AIGW_RELEASE_ALLOWED_SIGNERS"
		if *artifact {
			variable = "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"
		}
		return writeTrustInput(*output, *githubEnvironment, variable)
	default:
		return fmt.Errorf("unknown ci command: %s", args[0])
	}
}

func runNative(args []string, stdout io.Writer, runner commandRunner) error {
	flags := flag.NewFlagSet("ci native", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	platform := flags.String("platform", runtime.GOOS, "darwin, linux, or windows")
	fullQuality := flags.Bool("full-quality", false, "Qualify every repository quality tool on this host before native acceptance")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !supportedNativePlatform(*platform) {
		return errors.New("usage: ci native [--platform <darwin|linux|windows>] [--full-quality]")
	}
	if *platform != runtime.GOOS {
		return fmt.Errorf("native acceptance requires %s host, running on %s", *platform, runtime.GOOS)
	}
	_, err := readiness.ReadProductVersion(".")
	if err != nil {
		return err
	}
	commands := nativeCommands(*platform)
	if *fullQuality {
		if err := validateRepositoryQualityGraph("."); err != nil {
			return err
		}
		commands = append(slices.Clone(qualityCommands), commands[1:]...)
	}
	return runCommands(commands, stdout, runner)
}

func configuredSourceCommands(root string) ([]command, error) {
	commands, err := configuredQualityCommands(root)
	if err != nil {
		return nil, err
	}
	for _, gate := range repositoryQualityGraph.Gates {
		if gate.SourceOnly {
			commands = append(commands, gate.Command)
		}
	}
	return commands, nil
}

func configuredQualityCommands(root string) ([]command, error) {
	if err := validateRepositoryQualityGraph(root); err != nil {
		return nil, err
	}
	commands := slices.Clone(qualityCommands)
	base := os.Getenv("AIGW_COMMIT_BASE")
	email := os.Getenv("AIGW_RELEASE_AUTHOR_EMAIL")
	signers := os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")
	if base == "" && email == "" && signers == "" {
		return commands, nil
	}
	if base == "" || email == "" || signers == "" {
		return nil, errors.New("product provenance verification requires commit base, author email, and allowed signers file")
	}
	provenance := command{Name: "go", Args: []string{"run", "./tools/forge", "commits", "--base", base, "--email", email, "--allowed-signers", signers}}
	commands = append([]command{provenance}, commands...)
	return commands, nil
}

func writeTrustInput(output, githubEnvironment, variable string) error {
	allowed := os.Getenv(variable)
	if allowed == "" {
		return fmt.Errorf("%s is required", variable)
	}
	if err := os.WriteFile(output, []byte(allowed+"\n"), 0o600); err != nil {
		return fmt.Errorf("write trust input: %w", err)
	}
	file, err := os.OpenFile(githubEnvironment, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub environment: %w", err)
	}
	defer func() { _ = file.Close() }()
	_, err = fmt.Fprintf(file, "%s_FILE=%s\n", variable, output)
	return err
}

func supportedNativePlatform(platform string) bool {
	return platform == "darwin" || platform == "linux" || platform == "windows"
}

func nativeCommands(platform string) []command {
	tests := command{Name: "go", Args: []string{"test", "-json", "./..."}}
	if platform != "windows" {
		profile := filepath.Join("build", "acceptance", "coverage-"+platform+".out")
		tests.Args = []string{"run", "./tools/coverage", "--race", "--profile-output", profile}
	}
	return []command{
		{Name: "go", Args: []string{"run", "./tools/ci", "check-go", "."}},
		tests,
		{Name: "go", Args: []string{"run", "./tools/release", "accept-native"}},
	}
}
