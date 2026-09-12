package secrets

import (
	"errors"
	"fmt"
	"os"
)

// fileStore persists every credential purpose beneath one selected backend
// root. Platform leaves supply only storage protection and validation.
type fileStore struct{ root string }

func newFileStore(root string) Store { return scopedView{store: &fileStore{root: root}} }

func (store *fileStore) get(kind Kind, account string) (string, error) {
	root, err := openSecureRoot(store.root, false)
	if err != nil {
		return "", err
	}
	if root == nil {
		return "", ErrNotFound
	}
	defer func() { _ = root.Close() }()
	value, err := readSecureFile(root, slotName(kind, account))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Token file: %w", err)
	}
	defer clear(value)
	plain, err := decodeCredential(value)
	if err != nil {
		return "", err
	}
	defer clear(plain)
	if len(plain) == 0 {
		return "", ErrNotFound
	}
	return string(plain), nil
}

func (store *fileStore) set(kind Kind, account, value string) error {
	root, err := openSecureRoot(store.root, true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	encoded, err := encodeCredential([]byte(value))
	if err != nil {
		return err
	}
	defer clear(encoded)
	if err := validateOptionalOwnedFile(root, slotName(kind, account)); err != nil {
		return err
	}
	if err := writeSecureFile(root, slotName(kind, account), encoded); err != nil {
		return fmt.Errorf("write Token file: %w", err)
	}
	return nil
}

func (store *fileStore) delete(kind Kind, account string) error {
	root, err := openSecureRoot(store.root, false)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	defer func() { _ = root.Close() }()
	name := slotName(kind, account)
	if err := deleteSecureFile(root, name); err != nil {
		return fmt.Errorf("delete Token file: %w", err)
	}
	return nil
}

func (store *fileStore) exists(kind Kind, account string) (bool, error) {
	root, err := openSecureRoot(store.root, false)
	if err != nil {
		return false, err
	}
	if root == nil {
		return false, nil
	}
	defer func() { _ = root.Close() }()
	present, err := secureFileExists(root, slotName(kind, account))
	if err != nil {
		return false, err
	}
	return present, nil
}
