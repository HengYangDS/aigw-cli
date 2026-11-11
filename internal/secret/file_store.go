package secret

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
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
	// Windows protects a stored value, and an empty file holds nothing to
	// unprotect, so it is reported here on every platform.
	if len(value) == 0 {
		return "", ErrEmpty
	}
	plain, err := decodeCredential(value)
	if err != nil {
		return "", err
	}
	defer clear(plain)
	if len(plain) == 0 {
		return "", ErrEmpty
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
	// A failure names the Token file that the write replaces, as a full
	// path, never the staging file that it goes through.
	if err := writeSecureFile(root, slotName(kind, account), encoded); err != nil {
		return fmt.Errorf("write Token file: %w", tokenFileFailure(platform.AtTarget(filepath.Join(store.root, slotName(kind, account)), err)))
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
		return fmt.Errorf("delete Token file: %w", tokenFileFailure(platform.AtTarget(filepath.Join(store.root, name), err)))
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

// list names the regular files of the Token directory whose names are
// provider IDs, from the directory alone. A balance credential's slot and a
// staging file have names of another shape.
func (store *fileStore) list() ([]string, error) {
	root, err := openSecureRoot(store.root, false)
	if err != nil || root == nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open Token directory: %w", err)
	}
	defer func() { _ = directory.Close() }()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("list Token directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() && domain.ValidIdentifier(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// holdsNoValue reports, from the file's size and without reading it, that a
// present slot is empty.
func (store *fileStore) holdsNoValue(kind Kind, account string) (bool, error) {
	root, err := openSecureRoot(store.root, false)
	if err != nil || root == nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat(slotName(kind, account))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Token file: %w", err)
	}
	return info.Size() == 0, nil
}
