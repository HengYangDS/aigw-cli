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
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, errors.New("open private recovery directory")
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
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return nil, errors.New("private recovery path escapes its root")
	}
	current := root
	info, err := os.Lstat(current)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("private recovery root is unsafe")
	}
	if relative == "." {
		return info, nil
	}
	parts := strings.Split(relative, string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("private recovery path contains a link")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, errors.New("private recovery parent is unsafe")
		}
	}
	return info, nil
}
