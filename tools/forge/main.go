// Command forge verifies product Git objects and publishes them unchanged to
// independently selected optional Forge peers.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() { os.Exit(execute(os.Args[1:], os.Stderr)) }

func execute(arguments []string, stderr *os.File) int {
	if err := run(arguments); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: forge <commits|tag|tags|project|publish-tag>")
	}
	switch arguments[0] {
	case "commits":
		return runCommitVerification(arguments[1:])
	case "tag":
		return runTagVerification(arguments[1:])
	case "tags":
		return runTagSetVerification(arguments[1:])
	case "project":
		return runProjection(arguments[1:])
	case "publish-tag":
		return runTagPublication(arguments[1:])
	default:
		return fmt.Errorf("unknown forge command: %s", arguments[0])
	}
}

func gitOutput(repository string, arguments ...string) (string, error) {
	output, err := gitBytes(repository, arguments...)
	return strings.TrimSpace(string(output)), err
}

func gitBytes(repository string, arguments ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, message)
	}
	return stdout.Bytes(), nil
}

func gitRun(repository string, arguments ...string) error {
	_, err := gitBytes(repository, arguments...)
	return err
}
