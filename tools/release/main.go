// Command release orchestrates deterministic artifact construction and publication.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func main() { os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr)) }

func execute(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: release <build|build-ci|validate-release-sources|validate-toolchain|validate-readiness|validate-readiness-tag|validate-readiness-doc|validate-artifacts|compare-artifacts|upload-gitlab|publish-github|publish-gitlab>")
	}
	for _, commands := range []commandSet{buildCommands(), policyCommands(), artifactCommands(), publicationCommands()} {
		if command, ok := commands[args[0]]; ok {
			return command(args[1:], stdout)
		}
	}
	return fmt.Errorf("unknown release command: %s", args[0])
}

type command func([]string, io.Writer) error
type commandSet map[string]command

func requireArguments(args []string, count int, usage string) error {
	if len(args) != count {
		return errors.New(usage)
	}
	return nil
}
