//go:build windows

package secrets

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileStore struct{ root string }

func newFileStore(root string) Store { return &fileStore{root: root} }

func (store *fileStore) Get(profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	root, err := openWindowsRoot(store.root, false)
	if err != nil {
		return "", err
	}
	if root == nil {
		return "", ErrNotFound
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(profile)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Token file: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect Token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("Token path must be a regular file")
	}
	protected, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read Token file: %w", err)
	}
	plain, err := unprotect(protected)
	if err != nil {
		return "", fmt.Errorf("decrypt Token with Windows DPAPI: %w", err)
	}
	defer clear(plain)
	if len(plain) == 0 {
		return "", ErrNotFound
	}
	return string(plain), nil
}

func (store *fileStore) Set(profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	root, err := openWindowsRoot(store.root, true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	protected, err := protect([]byte(value))
	if err != nil {
		return fmt.Errorf("encrypt Token with Windows DPAPI: %w", err)
	}
	temporary := ".token-" + rand.Text()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create protected Token file: %w", err)
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = root.Remove(temporary)
		}
	}()
	if _, err := file.Write(protected); err != nil {
		return fmt.Errorf("write protected Token file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync protected Token file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close protected Token file: %w", err)
	}
	if err := root.Rename(temporary, profile); err != nil {
		return fmt.Errorf("replace protected Token file: %w", err)
	}
	committed = true
	return nil
}

func (store *fileStore) Delete(profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	root, err := openWindowsRoot(store.root, false)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	defer func() { _ = root.Close() }()
	if err := root.Remove(profile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete Token file: %w", err)
	}
	return nil
}

func (store *fileStore) Has(profile string) bool {
	_, err := store.Get(profile)
	return err == nil
}

func openWindowsRoot(path string, create bool) (*os.Root, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("file secret backend requires an AIGW storage root")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && create {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, fmt.Errorf("create Token directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect Token directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("Token directory must be a real directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open Token directory: %w", err)
	}
	return root, nil
}

func protect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("Token is empty")
	}
	input := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	return consumeLocalBlob(output)
}

func unprotect(protected []byte) ([]byte, error) {
	if len(protected) == 0 {
		return nil, errors.New("protected Token is empty")
	}
	input := windows.DataBlob{Size: uint32(len(protected)), Data: &protected[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	return consumeLocalBlob(output)
}

func consumeLocalBlob(blob windows.DataBlob) ([]byte, error) {
	if blob.Size == 0 || blob.Data == nil {
		return nil, errors.New("Windows DPAPI returned an empty value")
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data))) }()
	result := make([]byte, int(blob.Size))
	copy(result, unsafe.Slice(blob.Data, int(blob.Size)))
	return result, nil
}

type backendChoice struct{ root string }

func newBackendChoice(root string) backendChoice { return backendChoice{root: root} }

func (choice backendChoice) Read() (string, error) {
	root, err := openWindowsRoot(choice.root, false)
	if err != nil {
		return "", err
	}
	if root == nil {
		return "", ErrNotFound
	}
	defer func() { _ = root.Close() }()
	value, err := root.ReadFile("backend")
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
	root, err := openWindowsRoot(choice.root, true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	temporary := ".backend-" + rand.Text()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create secret backend selection: %w", err)
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = root.Remove(temporary)
		}
	}()
	if _, err := file.WriteString(backend + "\n"); err != nil {
		return fmt.Errorf("write secret backend selection: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync secret backend selection: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close secret backend selection: %w", err)
	}
	if err := root.Rename(temporary, "backend"); err != nil {
		return fmt.Errorf("replace secret backend selection: %w", err)
	}
	committed = true
	return nil
}
