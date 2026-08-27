//go:build !windows

package secrets

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// fileStore persists one Token per Account beneath an owner-only directory.
type fileStore struct{ root string }

func newFileStore(root string) Store { return &fileStore{root: root} }

func (store *fileStore) Get(profile string) (string, error) {
	return store.get(APIToken, profile)
}

func (store *fileStore) get(kind Kind, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	name := slotName(kind, profile)
	root, err := openSecureRoot(store.root, false)
	if err != nil {
		return "", err
	}
	if root == nil {
		return "", ErrNotFound
	}
	defer func() { _ = root.Close() }()
	value, err := readSecureFile(root, name)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Token file: %w", err)
	}
	if len(value) == 0 {
		return "", ErrNotFound
	}
	return string(value), nil
}

func (store *fileStore) Set(profile, value string) error {
	return store.set(APIToken, profile, value)
}

func (store *fileStore) set(kind Kind, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	root, err := openSecureRoot(store.root, true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	name := slotName(kind, profile)
	if err := validateOptionalOwnedFile(root, name); err != nil {
		return err
	}
	return writeSecureFile(root, name, []byte(value))
}

func (store *fileStore) Delete(profile string) error {
	return store.delete(APIToken, profile)
}

func (store *fileStore) delete(kind Kind, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	root, err := openSecureRoot(store.root, false)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	defer func() { _ = root.Close() }()
	name := slotName(kind, profile)
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Token file: %w", err)
	}
	if err := validateOwnedFile(info); err != nil {
		return err
	}
	if err := root.Remove(name); err != nil {
		return fmt.Errorf("delete Token file: %w", err)
	}
	return syncRoot(root)
}

func (store *fileStore) Has(profile string) bool {
	_, err := store.Get(profile)
	return err == nil
}

func validateOptionalOwnedFile(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Token file: %w", err)
	}
	return validateOwnedFile(info)
}

func validateOwnedFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.New("Token path must be an owner-only regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || stat.Nlink != 1 {
		return errors.New("Token file ownership is ambiguous")
	}
	return validateOwner(info)
}

func validateOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || int(stat.Uid) != os.Geteuid() {
		return errors.New("Token storage is not owned by the current user")
	}
	return nil
}
