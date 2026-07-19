//go:build !darwin && !linux

package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func writeRecoveryFileAtomicIfUnchanged(root, path string, expected transaction.FileSnapshot, data []byte, defaultMode os.FileMode) (transaction.FileSnapshot, error) {
	if err := validateRecoveryMutationParents(root, path, true); err != nil {
		return transaction.FileSnapshot{}, err
	}
	current, err := captureRecoveryFileNoFollow(root, path)
	if err != nil {
		return transaction.FileSnapshot{}, err
	}
	if !sameRecoverySnapshot(current, expected) {
		return transaction.FileSnapshot{}, errors.New("private recovery preimage changed")
	}
	if err := transaction.WriteFileAtomic(path, data, defaultMode); err != nil {
		return transaction.FileSnapshot{}, err
	}
	return captureRecoveryFileNoFollow(root, path)
}

func removeRecoveryFileIfUnchanged(root, path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
	if err := validateRecoveryMutationParents(root, path, false); err != nil {
		return transaction.FileSnapshot{}, err
	}
	current, err := captureRecoveryFileNoFollow(root, path)
	if err != nil {
		return transaction.FileSnapshot{}, err
	}
	if !sameRecoverySnapshot(current, expected) {
		return transaction.FileSnapshot{}, errors.New("private recovery preimage changed")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return transaction.FileSnapshot{}, err
	}
	return transaction.FileSnapshot{}, nil
}

func restoreRecoveryFileAtomicIfPostimage(root, path string, preimage, postimage transaction.FileSnapshot) error {
	if err := validateRecoveryMutationParents(root, path, preimage.Exists); err != nil {
		return err
	}
	current, err := captureRecoveryFileNoFollow(root, path)
	if err != nil {
		return err
	}
	if !sameRecoverySnapshot(current, postimage) {
		return errors.New("private recovery postimage changed")
	}
	if preimage.Exists {
		return transaction.WriteFileAtomic(path, preimage.Data, preimage.Mode)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func validateRecoveryMutationParents(root, path string, create bool) error {
	if !recoveryOwnedPath(root, path) {
		return errors.New("private recovery path escapes its root")
	}
	root = filepath.Clean(root)
	if create {
		if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
			return err
		}
	}
	parent := filepath.Dir(filepath.Clean(path))
	relative, err := filepath.Rel(root, parent)
	if err != nil {
		return err
	}
	parts := []string{}
	if relative != "." {
		parts = strings.Split(relative, string(os.PathSeparator))
	}
	current := root
	for index := -1; index < len(parts); index++ {
		if index >= 0 {
			current = filepath.Join(current, parts[index])
		}
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && create {
			if err := os.Mkdir(current, 0o700); err != nil {
				return err
			}
			info, err = os.Lstat(current)
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("private recovery parent is unsafe")
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
			return errors.New("private recovery parent has unsafe permissions")
		}
	}
	return nil
}
