package secrets

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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
	value    []byte
	identity fileIdentity
	exists   bool
}

type fileIdentity struct {
	info     os.FileInfo
	size     int64
	mode     os.FileMode
	modified int64
}

func openSecureRoot(path string, create bool) (*os.Root, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("file secret backend requires an AIGW storage root")
	}
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
	postimage, err := writeSecureFileWithPostimage(root, name, value)
	clear(postimage.value)
	return err
}

func writeSecureFileWithPostimage(root writeRoot, name string, value []byte) (credentialFileSnapshot, error) {
	preimage, err := captureOptionalSecureFile(root, name)
	if err != nil {
		return credentialFileSnapshot{}, err
	}
	return writeSecureFileFromPreimage(root, name, preimage, value)
}

func writeSecureFileFromPreimage(root writeRoot, name string, preimage credentialFileSnapshot, value []byte) (credentialFileSnapshot, error) {
	defer clear(preimage.value)
	postimage, committed, err := replaceSecureFile(root, name, value)
	if err == nil {
		return postimage, err
	}
	if !committed {
		clear(postimage.value)
		return credentialFileSnapshot{}, err
	}
	rollbackErr := restoreSecureFile(root, name, preimage, postimage, err)
	clear(postimage.value)
	return credentialFileSnapshot{}, rollbackErr
}

func replaceSecureFile(root writeRoot, name string, value []byte) (credentialFileSnapshot, bool, error) {
	temporaryName := ".token-" + rand.Text()
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return credentialFileSnapshot{}, false, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
	}()
	defer func() { _ = root.Remove(temporaryName) }()
	if err := writeAndSync(temporary, value); err != nil {
		return credentialFileSnapshot{}, false, err
	}
	info, err := temporary.Stat()
	if err != nil {
		return credentialFileSnapshot{}, false, err
	}
	if err := temporary.Close(); err != nil {
		return credentialFileSnapshot{}, false, err
	}
	closed = true
	if err := root.Rename(temporaryName, name); err != nil {
		return credentialFileSnapshot{}, false, err
	}
	postimage := credentialFileSnapshot{value: append([]byte(nil), value...), identity: identifyFile(info), exists: true}
	if err := syncRoot(root); err != nil {
		return postimage, true, err
	}
	return postimage, true, nil
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

func deleteSecureFileIf(root writeRoot, name string, expected credentialFileSnapshot) error {
	current, err := captureOptionalSecureFile(root, name)
	if err != nil {
		return err
	}
	defer clear(current.value)
	if !sameCredentialFileSnapshot(current, expected) {
		return errors.New("secret backend selection changed; refusing to remove newer state")
	}
	if err := root.Remove(name); err != nil {
		return err
	}
	if err := syncRoot(root); err != nil {
		return restoreSecureFile(root, name, current, credentialFileSnapshot{}, err)
	}
	return nil
}

func secureFileExists(root writeRoot, name string) (bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Token file: %w", err)
	}
	if err := validateOwnedFile(info); err != nil {
		return false, err
	}
	return true, nil
}

func validateOptionalOwnedFile(root writeRoot, name string) error {
	_, err := secureFileExists(root, name)
	return err
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
	return credentialFileSnapshot{value: value, identity: identifyFile(info), exists: true}, nil
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
	restored, _, err := replaceSecureFile(root, name, preimage.value)
	clear(restored.value)
	if err != nil {
		return fmt.Errorf("%w; restore previous Token: %v", cause, err)
	}
	return cause
}

func sameCredentialFileSnapshot(left, right credentialFileSnapshot) bool {
	if left.exists != right.exists {
		return false
	}
	if !left.exists {
		return true
	}
	if subtle.ConstantTimeCompare(left.value, right.value) != 1 {
		return false
	}
	if right.identity.info == nil {
		return true
	}
	return left.identity.info != nil &&
		os.SameFile(left.identity.info, right.identity.info) &&
		left.identity.mode == right.identity.mode &&
		left.identity.size == right.identity.size &&
		left.identity.modified == right.identity.modified
}

func identifyFile(info os.FileInfo) fileIdentity {
	return fileIdentity{info: info, size: info.Size(), mode: info.Mode(), modified: info.ModTime().UnixNano()}
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

func syncDirectory(directory syncer) error { return syncCredentialDirectory(directory) }
