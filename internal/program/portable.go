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

// InstallPlan is what Install would do: place the running program at Path,
// keeping the program that it replaces at Rollback, unless Path already
// holds this program.
type InstallPlan struct {
	Path      string
	Rollback  string
	Unchanged bool
}

// PlanInstall returns what Install would do, and writes nothing.
func PlanInstall(source, target string) (InstallPlan, error) {
	plan, _, _, err := planInstall(source, target)
	return plan, err
}

// planInstall decides an installation and returns the program bytes and the
// bytes of the program that it replaces, if any.
func planInstall(source, target string) (InstallPlan, []byte, []byte, error) {
	if err := RequirePortableOwnership(source); err != nil {
		return InstallPlan{}, nil, nil, err
	}
	if err := RequirePortableOwnership(target); err != nil {
		return InstallPlan{}, nil, nil, err
	}
	sourcePath := filepath.Clean(source)
	targetPath := filepath.Clean(target)
	if sourcePath == targetPath {
		return InstallPlan{}, nil, nil, errors.New("source and target resolve to the same path")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return InstallPlan{}, nil, nil, fmt.Errorf("read portable AIGW executable: %w", err)
	}
	previous, err := os.ReadFile(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return InstallPlan{}, nil, nil, fmt.Errorf("read installed portable AIGW executable: %w", err)
	}
	plan := InstallPlan{Path: targetPath}
	switch {
	case err != nil:
		previous = nil
	case bytes.Equal(data, previous):
		plan.Unchanged = true
	default:
		plan.Rollback = RollbackPath(targetPath)
		if err := refuseRollbackDirectory(plan.Rollback); err != nil {
			return InstallPlan{}, nil, nil, err
		}
	}
	return plan, data, previous, nil
}

// Install atomically places the current executable at the requested portable target while retaining one rollback copy.
func Install(source, target string) error {
	plan, data, previous, err := planInstall(source, target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plan.Path), 0o755); err != nil {
		return fmt.Errorf("create portable installation directory: %w", err)
	}
	if plan.Unchanged {
		return os.Chmod(plan.Path, 0o755)
	}
	if plan.Rollback != "" {
		mode := os.FileMode(0o755)
		if current, statErr := os.Stat(plan.Path); statErr == nil {
			mode = current.Mode().Perm()
		}
		if err := writeFileAtomic(plan.Rollback, previous, mode); err != nil {
			return fmt.Errorf("save previous portable AIGW executable: %w", err)
		}
	}
	if err := writeFileAtomic(plan.Path, data, 0o755); err != nil {
		return err
	}
	return os.Chmod(plan.Path, 0o755)
}

// PlanUninstall returns the files among the portable executable and its owned
// rollback copy that Uninstall would remove, and removes nothing.
func PlanUninstall(target string) ([]string, error) {
	if err := admitUninstall(target); err != nil {
		return nil, err
	}
	present := []string{}
	for _, path := range []string{target, RollbackPath(target)} {
		if _, err := os.Lstat(path); err == nil {
			present = append(present, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect portable AIGW file %s: %w", path, err)
		}
	}
	return present, nil
}

// RefuseDirectories refuses a directory at the portable program's path or at
// its rollback copy's path, where AIGW keeps only program files, before
// anything changes.
func RefuseDirectories(target string) error {
	if info, err := os.Lstat(target); err == nil && info.IsDir() {
		return fmt.Errorf("portable AIGW file %s is a directory; name the program file", target)
	}
	return refuseRollbackDirectory(RollbackPath(target))
}

// refuseRollbackDirectory refuses a directory at the rollback copy's path,
// which AIGW did not write and neither removes nor replaces.
func refuseRollbackDirectory(path string) error {
	if info, err := os.Lstat(path); err == nil && info.IsDir() {
		return fmt.Errorf("rollback copy path %s is a directory that AIGW did not write; move it, then run the command again", path)
	}
	return nil
}

// admitUninstall refuses an uninstall target that AIGW does not own, that is
// empty, or whose program or rollback copy path holds a directory.
func admitUninstall(target string) error {
	if err := RequirePortableOwnership(target); err != nil {
		return err
	}
	if strings.TrimSpace(target) == "" {
		return errors.New("portable uninstall target is empty")
	}
	return RefuseDirectories(target)
}

// Uninstall removes the portable executable and its owned rollback copy while tolerating absence.
func Uninstall(target string) error {
	if err := admitUninstall(target); err != nil {
		return err
	}
	for _, path := range []string{target, RollbackPath(target)} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove portable AIGW file %s: %w", path, err)
		}
	}
	return nil
}
