//go:build darwin

package native

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

const keychainItemNotFoundExitCode = 44

func observeCredential(service, account string) (bool, error) {
	output, err := keychainMetadataCommand(service, account).CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == keychainItemNotFoundExitCode {
		return false, nil
	}
	return false, fmt.Errorf("query Keychain item metadata: %w: %s", err, bytes.TrimSpace(output))
}

func keychainMetadataCommand(service, account string) *exec.Cmd {
	return exec.Command("/usr/bin/security", "find-generic-password", "-s", service, "-a", account)
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME")
}
