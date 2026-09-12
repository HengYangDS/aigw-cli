//go:build darwin

package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

const keychainItemNotFoundExitCode = 44

func observeKeyringItem(service, slot string) (bool, error) {
	output, err := keychainMetadataCommand(service, slot).CombinedOutput()
	return classifyKeychainObservation(output, err)
}

func keychainMetadataCommand(service, slot string) *exec.Cmd {
	return exec.Command(
		"/usr/bin/security",
		"find-generic-password",
		"-s", service,
		"-a", slot,
	)
}

func classifyKeychainObservation(output []byte, err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == keychainItemNotFoundExitCode {
		return false, nil
	}
	return false, fmt.Errorf("query Keychain item metadata: %w: %s", err, bytes.TrimSpace(output))
}
