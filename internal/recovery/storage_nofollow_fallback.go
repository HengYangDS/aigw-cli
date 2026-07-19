//go:build !darwin && !linux

package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func captureRecoveryFileNoFollow(root, path string) (transaction.FileSnapshot, error) {
	info, err := lstatRecoveryPath(root, path)
	if errors.Is(err, os.ErrNotExist) {
		return transaction.FileSnapshot{}, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return transaction.FileSnapshot{}, errors.New("open private recovery file")
	}
	file, err := os.Open(path)
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("open private recovery file")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return transaction.FileSnapshot{}, errors.New("inspect private recovery file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return transaction.FileSnapshot{}, errors.New("read private recovery file")
	}
	sum := sha256.Sum256(data)
	return transaction.FileSnapshot{Exists: true, Data: data, SHA256: hex.EncodeToString(sum[:]), Mode: opened.Mode().Perm()}, nil
}

func readRecoveryDirectoryNoFollow(root, path string) ([]os.FileInfo, os.FileInfo, error) {
	info, err := lstatRecoveryPath(root, path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.IsDir() {
		return nil, nil, errors.New("inspect private recovery directory")
	}
	entries, err := file.Readdir(-1)
	return entries, opened, err
}

func lstatRecoveryPath(root, path string) (os.FileInfo, error) {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return nil, errors.New("private recovery path escapes its root")
	}
	return os.Lstat(path)
}
