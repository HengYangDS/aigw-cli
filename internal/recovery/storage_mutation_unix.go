//go:build darwin || linux

package recovery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func writeRecoveryFileAtomicIfUnchanged(root, path string, expected transaction.FileSnapshot, data []byte, defaultMode os.FileMode) (transaction.FileSnapshot, error) {
	directory, name, err := openRecoveryParentNoFollow(root, path, true)
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("open private recovery parent")
	}
	defer func() { _ = directory.Close() }()
	current, err := captureRecoveryFileAt(directory, name)
	if err != nil {
		return transaction.FileSnapshot{}, err
	}
	if !sameRecoverySnapshot(current, expected) {
		return transaction.FileSnapshot{}, errors.New("private recovery preimage changed")
	}
	mode := defaultMode
	if expected.Exists {
		mode = expected.Mode
	}
	if err := writeRecoveryFileAt(directory, name, data, mode); err != nil {
		return transaction.FileSnapshot{}, err
	}
	return captureRecoveryFileAt(directory, name)
}

func removeRecoveryFileIfUnchanged(root, path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
	directory, name, err := openRecoveryParentNoFollow(root, path, false)
	if errors.Is(err, os.ErrNotExist) {
		if sameRecoverySnapshot(transaction.FileSnapshot{}, expected) {
			return transaction.FileSnapshot{}, nil
		}
		return transaction.FileSnapshot{}, errors.New("private recovery preimage changed")
	}
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("open private recovery parent")
	}
	defer func() { _ = directory.Close() }()
	current, err := captureRecoveryFileAt(directory, name)
	if err != nil {
		return transaction.FileSnapshot{}, err
	}
	if !sameRecoverySnapshot(current, expected) {
		return transaction.FileSnapshot{}, errors.New("private recovery preimage changed")
	}
	if err := unix.Unlinkat(int(directory.Fd()), name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
		return transaction.FileSnapshot{}, errors.New("remove private recovery file")
	}
	return transaction.FileSnapshot{}, nil
}

func restoreRecoveryFileAtomicIfPostimage(root, path string, preimage, postimage transaction.FileSnapshot) error {
	directory, name, err := openRecoveryParentNoFollow(root, path, preimage.Exists)
	if errors.Is(err, os.ErrNotExist) {
		if sameRecoverySnapshot(transaction.FileSnapshot{}, postimage) && !preimage.Exists {
			return nil
		}
		return errors.New("private recovery postimage changed")
	}
	if err != nil {
		return errors.New("open private recovery parent")
	}
	defer func() { _ = directory.Close() }()
	current, err := captureRecoveryFileAt(directory, name)
	if err != nil {
		return err
	}
	if !sameRecoverySnapshot(current, postimage) {
		return errors.New("private recovery postimage changed")
	}
	if preimage.Exists {
		return writeRecoveryFileAt(directory, name, preimage.Data, preimage.Mode)
	}
	if err := unix.Unlinkat(int(directory.Fd()), name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
		return errors.New("remove private recovery file")
	}
	return nil
}

func openRecoveryParentNoFollow(root, path string, create bool) (*os.File, string, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return nil, "", errors.New("private recovery path escapes its root")
	}
	parts := strings.Split(relative, string(os.PathSeparator))
	name := parts[len(parts)-1]
	if name == "" || name == "." || name == ".." {
		return nil, "", errors.New("private recovery file name is invalid")
	}
	current, err := openRecoveryRootNoFollow(root, create, true)
	if err != nil {
		return nil, "", err
	}
	currentPath := root
	for _, part := range parts[:len(parts)-1] {
		if part == "" || part == "." || part == ".." {
			_ = current.Close()
			return nil, "", errors.New("private recovery parent is invalid")
		}
		if create {
			if mkdirErr := unix.Mkdirat(int(current.Fd()), part, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				_ = current.Close()
				return nil, "", mkdirErr
			}
		}
		nextFD, openErr := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			entryExists := recoveryEntryExistsAt(current, part)
			_ = current.Close()
			if errors.Is(openErr, unix.ENOENT) && !entryExists {
				return nil, "", os.ErrNotExist
			}
			return nil, "", openErr
		}
		_ = current.Close()
		currentPath = filepath.Join(currentPath, part)
		current = os.NewFile(uintptr(nextFD), currentPath)
		if current == nil {
			_ = unix.Close(nextFD)
			return nil, "", errors.New("open private recovery parent")
		}
		info, statErr := current.Stat()
		if statErr != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			_ = current.Close()
			return nil, "", errors.New("private recovery parent has unsafe permissions")
		}
	}
	return current, name, nil
}

func openRecoveryRootNoFollow(root string, create, requirePrivateMode bool) (*os.File, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root = canonicalRecoveryRootPath(filepath.Clean(root))
	filesystemRoot := string(os.PathSeparator)
	rootFD, err := unix.Open(filesystemRoot, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(rootFD), filesystemRoot)
	if current == nil {
		_ = unix.Close(rootFD)
		return nil, errors.New("open filesystem root")
	}
	currentPath := filesystemRoot
	relative := strings.TrimPrefix(root, filesystemRoot)
	for _, part := range strings.Split(relative, filesystemRoot) {
		if part == "" {
			continue
		}
		if create {
			if mkdirErr := unix.Mkdirat(int(current.Fd()), part, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				_ = current.Close()
				return nil, mkdirErr
			}
		}
		nextFD, openErr := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			entryExists := recoveryEntryExistsAt(current, part)
			_ = current.Close()
			if errors.Is(openErr, unix.ENOENT) && !entryExists {
				return nil, os.ErrNotExist
			}
			return nil, openErr
		}
		_ = current.Close()
		currentPath = filepath.Join(currentPath, part)
		current = os.NewFile(uintptr(nextFD), currentPath)
		if current == nil {
			_ = unix.Close(nextFD)
			return nil, errors.New("open private recovery root")
		}
	}
	info, err := current.Stat()
	if err != nil || !info.IsDir() || (requirePrivateMode && info.Mode().Perm() != 0o700) {
		_ = current.Close()
		return nil, errors.New("private recovery root has unsafe permissions")
	}
	return current, nil
}

func canonicalRecoveryRootPath(path string) string {
	if runtime.GOOS != "darwin" {
		return path
	}
	for _, alias := range []string{"/var", "/tmp", "/etc"} {
		if path == alias || strings.HasPrefix(path, alias+string(os.PathSeparator)) {
			return filepath.Join("/private", path)
		}
	}
	return path
}

func recoveryEntryExistsAt(directory *os.File, name string) bool {
	var stat unix.Stat_t
	return unix.Fstatat(int(directory.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW) == nil
}

func captureRecoveryFileAt(directory *os.File, name string) (transaction.FileSnapshot, error) {
	fd, err := unix.Openat(int(directory.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ENOENT) {
		return transaction.FileSnapshot{}, nil
	}
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("open private recovery file")
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return transaction.FileSnapshot{}, errors.New("open private recovery file")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return transaction.FileSnapshot{}, errors.New("inspect private recovery file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("read private recovery file")
	}
	sum := sha256.Sum256(data)
	return transaction.FileSnapshot{Exists: true, Data: append([]byte(nil), data...), SHA256: hex.EncodeToString(sum[:]), Mode: info.Mode().Perm()}, nil
}

func writeRecoveryFileAt(directory *os.File, name string, data []byte, mode os.FileMode) error {
	temporary, temporaryName, err := createRecoveryTempAt(directory)
	if err != nil {
		return err
	}
	defer func() {
		_ = temporary.Close()
		_ = unix.Unlinkat(int(directory.Fd()), temporaryName, 0)
	}()
	if err := temporary.Chmod(mode.Perm()); err != nil {
		return errors.New("set private recovery temporary mode")
	}
	if _, err := temporary.Write(data); err != nil {
		return errors.New("write private recovery temporary file")
	}
	if err := temporary.Sync(); err != nil {
		return errors.New("sync private recovery temporary file")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close private recovery temporary file")
	}
	if err := unix.Renameat(int(directory.Fd()), temporaryName, int(directory.Fd()), name); err != nil {
		return errors.New("replace private recovery file")
	}
	return nil
}

func createRecoveryTempAt(directory *os.File) (*os.File, string, error) {
	for attempt := 0; attempt < 128; attempt++ {
		var token [8]byte
		if _, err := rand.Read(token[:]); err != nil {
			return nil, "", errors.New("name private recovery temporary file")
		}
		name := fmt.Sprintf(".aigw-write-%x", token[:])
		fd, err := unix.Openat(int(directory.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return nil, "", errors.New("create private recovery temporary file")
		}
		file := os.NewFile(uintptr(fd), name)
		if file == nil {
			_ = unix.Close(fd)
			_ = unix.Unlinkat(int(directory.Fd()), name, 0)
			return nil, "", errors.New("create private recovery temporary file")
		}
		return file, name, nil
	}
	return nil, "", errors.New("allocate private recovery temporary file")
}
