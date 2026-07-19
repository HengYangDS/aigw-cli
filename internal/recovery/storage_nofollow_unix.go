//go:build darwin || linux

package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func captureRecoveryFileNoFollow(root, path string) (transaction.FileSnapshot, error) {
	file, err := openRecoveryPathNoFollow(root, path, false)
	if errors.Is(err, os.ErrNotExist) {
		return transaction.FileSnapshot{}, nil
	}
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("open private recovery file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return transaction.FileSnapshot{}, errors.New("inspect private recovery file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("read private recovery file")
	}
	sum := sha256.Sum256(data)
	return transaction.FileSnapshot{
		Exists: true,
		Data:   append([]byte(nil), data...),
		SHA256: hex.EncodeToString(sum[:]),
		Mode:   info.Mode().Perm(),
	}, nil
}

func readRecoveryDirectoryNoFollow(root, path string) ([]os.FileInfo, os.FileInfo, error) {
	file, err := openRecoveryPathNoFollow(root, path, true)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		return nil, nil, errors.New("inspect private recovery directory")
	}
	entries, err := file.Readdir(-1)
	if err != nil {
		return nil, nil, errors.New("read private recovery directory")
	}
	return entries, info, nil
}

func openRecoveryPathNoFollow(root, path string, directory bool) (*os.File, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return nil, errors.New("private recovery path escapes its root")
	}
	current, err := openRecoveryRootNoFollow(root, false, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	if relative == "." {
		if !directory {
			current.Close()
			return nil, errors.New("private recovery file resolves to a directory")
		}
		return current, nil
	}
	parts := strings.Split(relative, string(os.PathSeparator))
	currentPath := root
	for index, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
		if index < len(parts)-1 || directory {
			flags |= unix.O_DIRECTORY
		} else {
			flags |= unix.O_NONBLOCK
		}
		nextFD, openErr := unix.Openat(int(current.Fd()), part, flags, 0)
		if openErr != nil {
			entryExists := recoveryEntryExistsAt(current, part)
			current.Close()
			if errors.Is(openErr, unix.ENOENT) && !entryExists {
				return nil, os.ErrNotExist
			}
			return nil, openErr
		}
		current.Close()
		currentPath = filepath.Join(currentPath, part)
		current = os.NewFile(uintptr(nextFD), currentPath)
		if current == nil {
			_ = unix.Close(nextFD)
			return nil, errors.New("open private recovery path")
		}
	}
	return current, nil
}
