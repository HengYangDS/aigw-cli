//go:build !windows

package secrets

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
)

type syncWriter interface {
	Write([]byte) (int, error)
	Sync() error
}

type syncer interface {
	Sync() error
}

type readRoot interface {
	Lstat(string) (os.FileInfo, error)
	Open(string) (*os.File, error)
}

type writeRoot interface {
	Lstat(string) (os.FileInfo, error)
	OpenFile(string, int, os.FileMode) (*os.File, error)
	Remove(string) error
	Rename(string, string) error
	Open(string) (*os.File, error)
}

type credentialFileSnapshot struct {
	value  []byte
	exists bool
}

func openSecureRoot(path string, create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, fmt.Errorf("create Token directory: %w", err)
		}
	}
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && !create {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect Token directory: %w", err)
	}
	if err := validateSecureRoot(before); err != nil {
		return nil, err
	}
	return openVerifiedRoot(path, before)
}

func validateSecureRoot(info os.FileInfo) error {
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return errors.New("Token directory must be an owner-only directory")
	}
	return validateOwner(info)
}

func openVerifiedRoot(path string, before os.FileInfo) (*os.Root, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open Token directory: %w", err)
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = root.Close()
		return nil, errors.New("Token directory changed while opening")
	}
	return root, nil
}

func readSecureFile(root readRoot, name string) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if err := validateOwnedFile(before); err != nil {
		return nil, err
	}
	file, err := openVerifiedFile(root, name, before)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

func openVerifiedFile(root readRoot, name string, before os.FileInfo) (*os.File, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		_ = file.Close()
		return nil, errors.New("Token file changed while opening")
	}
	return file, nil
}

func writeSecureFile(root writeRoot, name string, value []byte) error {
	preimage, err := captureOptionalSecureFile(root, name)
	if err != nil {
		return err
	}
	defer clear(preimage.value)
	committed, err := replaceSecureFile(root, name, value)
	if err == nil || !committed {
		return err
	}
	return restoreSecureFile(root, name, preimage, credentialFileSnapshot{value: value, exists: true}, err)
}

func replaceSecureFile(root writeRoot, name string, value []byte) (bool, error) {
	temporaryName := ".token-" + rand.Text()
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
	}()
	defer func() { _ = root.Remove(temporaryName) }()
	if err := writeAndSync(temporary, value); err != nil {
		return false, err
	}
	if err := temporary.Close(); err != nil {
		return false, err
	}
	closed = true
	if err := root.Rename(temporaryName, name); err != nil {
		return false, err
	}
	if err := syncRoot(root); err != nil {
		return true, err
	}
	return true, nil
}

func deleteSecureFile(root writeRoot, name string) error {
	preimage, err := captureOptionalSecureFile(root, name)
	if err != nil {
		return err
	}
	defer clear(preimage.value)
	if !preimage.exists {
		return nil
	}
	if err := root.Remove(name); err != nil {
		return err
	}
	if err := syncRoot(root); err != nil {
		return restoreSecureFile(root, name, preimage, credentialFileSnapshot{}, err)
	}
	return nil
}

func captureOptionalSecureFile(root writeRoot, name string) (credentialFileSnapshot, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return credentialFileSnapshot{}, nil
	}
	if err != nil {
		return credentialFileSnapshot{}, err
	}
	if err := validateOwnedFile(info); err != nil {
		return credentialFileSnapshot{}, err
	}
	value, err := readSecureFile(root, name)
	if err != nil {
		return credentialFileSnapshot{}, err
	}
	return credentialFileSnapshot{value: value, exists: true}, nil
}

func restoreSecureFile(root writeRoot, name string, preimage, postimage credentialFileSnapshot, cause error) error {
	current, err := captureOptionalSecureFile(root, name)
	if err != nil {
		return fmt.Errorf("%w; inspect Token postimage before compensation: %v", cause, err)
	}
	defer clear(current.value)
	if !sameCredentialFileSnapshot(current, postimage) {
		return fmt.Errorf("%w; Token postimage changed; refusing to overwrite newer state", cause)
	}
	if !preimage.exists {
		if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w; remove uncommitted Token: %v", cause, err)
		}
		if err := syncRoot(root); err != nil {
			return fmt.Errorf("%w; sync restored Token state: %v", cause, err)
		}
		return cause
	}
	if _, err := replaceSecureFile(root, name, preimage.value); err != nil {
		return fmt.Errorf("%w; restore previous Token: %v", cause, err)
	}
	return cause
}

func sameCredentialFileSnapshot(left, right credentialFileSnapshot) bool {
	return left.exists == right.exists &&
		(!left.exists || subtle.ConstantTimeCompare(left.value, right.value) == 1)
}

func writeAndSync(target syncWriter, value []byte) error {
	if _, err := target.Write(value); err != nil {
		return err
	}
	return target.Sync()
}

func syncRoot(root interface {
	Open(string) (*os.File, error)
}) error {
	directory, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("open Token directory: %w", err)
	}
	defer func() { _ = directory.Close() }()
	return syncDirectory(directory)
}

func syncDirectory(directory syncer) error { return directory.Sync() }
