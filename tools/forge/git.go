package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func remoteReference(repository, remote, ref string) (string, error) {
	output, err := gitOutput(repository, "ls-remote", remote, ref)
	if err != nil {
		return "", err
	}
	if output == "" {
		return "", nil
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || fields[1] != ref {
		return "", fmt.Errorf("remote reference observation is malformed: %s", ref)
	}
	return fields[0], nil
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
