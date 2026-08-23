//go:build !windows

package secrets

import (
	"crypto/rand"
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
	OpenFile(string, int, os.FileMode) (*os.File, error)
	Remove(string) error
	Rename(string, string) error
	Open(string) (*os.File, error)
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
	temporaryName := ".token-" + rand.Text()
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = temporary.Close() }()
	committed := false
	defer func() {
		if !committed {
			_ = root.Remove(temporaryName)
		}
	}()
	if err := writeAndSync(temporary, value); err != nil {
		return err
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return err
	}
	committed = true
	return syncRoot(root)
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
