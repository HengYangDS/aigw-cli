package program

import (
	"aigw-cli/internal/platform"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var writeFileAtomic = platform.WriteFileAtomic

// Install atomically places the current executable at the requested portable target while retaining one rollback copy.
func Install(source, target string) error {
	if err := RequirePortableOwnership(source); err != nil {
		return err
	}
	if err := RequirePortableOwnership(target); err != nil {
		return err
	}
	sourcePath := filepath.Clean(source)
	targetPath := filepath.Clean(target)
	if sourcePath == targetPath {
		return errors.New("source and target resolve to the same path")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read portable AIGW executable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create portable installation directory: %w", err)
	}
	previous, err := os.ReadFile(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read installed portable AIGW executable: %w", err)
	}
	if err == nil && bytes.Equal(data, previous) {
		return os.Chmod(targetPath, 0o755)
	}
	if err == nil {
		mode := os.FileMode(0o755)
		if current, statErr := os.Stat(targetPath); statErr == nil {
			mode = current.Mode().Perm()
		}
		if err := writeFileAtomic(RollbackPath(targetPath), previous, mode); err != nil {
			return fmt.Errorf("save previous portable AIGW executable: %w", err)
		}
	}
	if err := writeFileAtomic(targetPath, data, 0o755); err != nil {
		return err
	}
	return os.Chmod(targetPath, 0o755)
}

// Uninstall removes the portable executable and its owned rollback copy while tolerating absence.
func Uninstall(target string) error {
	if err := RequirePortableOwnership(target); err != nil {
		return err
	}
	if strings.TrimSpace(target) == "" {
		return errors.New("portable uninstall target is empty")
	}
	for _, path := range []string{target, RollbackPath(target)} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove portable AIGW file %s: %w", path, err)
		}
	}
	return nil
}
