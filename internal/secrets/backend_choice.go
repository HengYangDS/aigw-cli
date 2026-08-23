//go:build !windows

package secrets

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type backendChoice struct{ root string }

func newBackendChoice(root string) backendChoice { return backendChoice{root: root} }

func (choice backendChoice) Read() (string, error) {
	root, err := openSecureRoot(choice.root, false)
	if err != nil {
		return "", err
	}
	if root == nil {
		return "", ErrNotFound
	}
	defer func() { _ = root.Close() }()
	value, err := readSecureFile(root, "backend")
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read secret backend selection: %w", err)
	}
	backend := strings.TrimSpace(string(value))
	if backend != "keyring" && backend != "file" {
		return "", fmt.Errorf("invalid persisted secret backend %q", backend)
	}
	return backend, nil
}

func (choice backendChoice) Write(backend string) error {
	if backend != "keyring" && backend != "file" {
		return fmt.Errorf("invalid secret backend %q", backend)
	}
	root, err := openSecureRoot(choice.root, true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := writeSecureFile(root, "backend", []byte(backend+"\n")); err != nil {
		return fmt.Errorf("write secret backend selection: %w", err)
	}
	return nil
}
