package secrets

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const backendChoiceName = "backend"

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
	value, err := readSecureFile(root, backendChoiceName)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read secret backend selection: %w", err)
	}
	backend := strings.TrimSpace(string(value))
	if !validPersistedBackend(backend) {
		return "", fmt.Errorf("invalid persisted secret backend %q", backend)
	}
	return backend, nil
}

func (choice backendChoice) Persist(backend string) (credentialFileSnapshot, bool, error) {
	if !validPersistedBackend(backend) {
		return credentialFileSnapshot{}, false, fmt.Errorf("invalid secret backend %q", backend)
	}
	root, err := openSecureRoot(choice.root, true)
	if err != nil {
		return credentialFileSnapshot{}, false, err
	}
	defer func() { _ = root.Close() }()
	preimage, err := captureOptionalSecureFile(root, backendChoiceName)
	if err != nil {
		return credentialFileSnapshot{}, false, err
	}
	if preimage.exists {
		defer clear(preimage.value)
		selected := strings.TrimSpace(string(preimage.value))
		if !validPersistedBackend(selected) {
			return credentialFileSnapshot{}, false, fmt.Errorf("invalid persisted secret backend %q", selected)
		}
		if selected != backend {
			return credentialFileSnapshot{}, false, errors.New("secret backend selection changed; retry the credential operation")
		}
		return credentialFileSnapshot{}, false, nil
	}
	postimage, err := writeSecureFileFromPreimage(root, backendChoiceName, preimage, []byte(backend+"\n"))
	if err != nil {
		return credentialFileSnapshot{}, false, fmt.Errorf("write secret backend selection: %w", err)
	}
	return postimage, true, nil
}

func (choice backendChoice) Rollback(postimage credentialFileSnapshot) error {
	if !postimage.exists || postimage.identity.info == nil {
		return errors.New("secret backend selection rollback requires an owned postimage")
	}
	root, err := openSecureRoot(choice.root, false)
	if err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("secret backend selection changed; refusing to remove newer state")
	}
	defer func() { _ = root.Close() }()
	if err := deleteSecureFileIf(root, backendChoiceName, postimage); err != nil {
		return fmt.Errorf("remove secret backend selection: %w", err)
	}
	return nil
}

func validPersistedBackend(backend string) bool {
	return backend == "keyring" || backend == "file"
}
