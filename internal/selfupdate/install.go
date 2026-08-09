package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	executableupdate "github.com/creativeprojects/go-selfupdate/update"
)

// installPortableArchive verifies and extracts a portable archive, then
// installs the contained binary using one cross-platform atomic replacement
// owner. The returned bool is retained as the stable internal result shape and
// is always false: no platform-specific helper process or script is scheduled.
func (u Updater) installPortableArchive(archivePath, tag string) (string, bool, error) {
	archiveName := filepath.Base(archivePath)
	if err := verifyChecksum(archivePath, filepath.Join(filepath.Dir(archivePath), "checksums.txt"), archiveName); err != nil {
		return "", false, err
	}
	binaryName := "aigw"
	if u.GOOS == "windows" {
		binaryName = "aigw.exe"
	}
	binary, err := extractBinary(archivePath, expectedBinaryPath(normalizeVersion(tag), u.GOOS, u.GOARCH, binaryName))
	if err != nil {
		return "", false, err
	}
	if err := u.replacePortableBinary(binary); err != nil {
		return "", false, err
	}
	return "updated to " + tag, false, nil
}

func (u Updater) replacePortableBinary(binary []byte) error {
	if strings.TrimSpace(u.Executable) == "" {
		return errors.New("AIGW executable path is empty")
	}
	information, err := os.Stat(u.Executable)
	if err != nil {
		return fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	if err := executableupdate.Apply(bytes.NewReader(binary), executableupdate.Options{
		TargetPath:  u.Executable,
		TargetMode:  information.Mode().Perm(),
		OldSavePath: rollbackPath(u.Executable),
	}); err != nil {
		if rollbackErr := executableupdate.RollbackError(err); rollbackErr != nil {
			return fmt.Errorf("replace AIGW executable: %w; restore current executable: %v", err, rollbackErr)
		}
		return fmt.Errorf("replace AIGW executable: %w", err)
	}
	return nil
}

func portableArchiveName(version, goos, goarch string) string {
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("aigw_%s_%s_%s%s", version, goos, goarch, extension)
}

func archiveVersion(name, goos, goarch string) (string, error) {
	prefix := "aigw_"
	suffix := "_" + goos + "_" + goarch
	if goos == "windows" {
		suffix += ".zip"
	} else {
		suffix += ".tar.gz"
	}
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("verified local candidate archive must target %s/%s", goos, goarch)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if _, err := parseVersion("v" + version); err != nil {
		return "", err
	}
	return version, nil
}

func expectedBinaryPath(version, goos, goarch, binaryName string) string {
	return fmt.Sprintf("aigw_%s_%s_%s/%s", version, goos, goarch, binaryName)
}

// Rollback restores the immediately preceding portable AIGW executable without
// accessing the network. It swaps the current and previous binaries so the
// action itself remains reversible and never creates an unbounded chain.
func (u Updater) Rollback(_ context.Context) (string, error) {
	if strings.TrimSpace(u.Executable) == "" {
		return "", errors.New("AIGW executable path is empty")
	}
	info, err := os.Stat(u.Executable)
	if err != nil {
		return "", fmt.Errorf("read current AIGW executable: %w", err)
	}
	backup := rollbackPath(u.Executable)
	previous, err := os.ReadFile(backup)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no previous portable AIGW binary is available")
		}
		return "", fmt.Errorf("read previous AIGW executable: %w", err)
	}
	if err := executableupdate.Apply(bytes.NewReader(previous), executableupdate.Options{
		TargetPath:  u.Executable,
		TargetMode:  info.Mode().Perm(),
		OldSavePath: backup,
	}); err != nil {
		if rollbackErr := executableupdate.RollbackError(err); rollbackErr != nil {
			return "", fmt.Errorf("restore previous AIGW executable: %w; restore current executable: %v", err, rollbackErr)
		}
		return "", fmt.Errorf("restore previous AIGW executable: %w", err)
	}
	return "restored the previous program version. If that older program does not support `aigw update --rollback`, download the current portable package and run its installer; it replaces only AIGW and retains one predecessor.", nil
}

// rollbackPath derives the sibling backup path for executable using the
// separator style already present in executable, rather than the host OS's
// native separator. This keeps the result stable across platforms: a
// POSIX-style path (e.g. produced by a Windows binary staged from a
// forward-slash working directory) must not be rewritten with backslashes,
// and vice versa.
func rollbackPath(executable string) string {
	suffix := ".aigw.previous"
	if strings.EqualFold(filepath.Ext(executable), ".exe") {
		suffix += ".exe"
	}
	if strings.Contains(executable, `\`) && !strings.Contains(executable, "/") {
		if index := strings.LastIndex(executable, `\`); index >= 0 {
			return executable[:index+1] + suffix
		}
		return suffix
	}
	if index := strings.LastIndex(executable, "/"); index >= 0 {
		return executable[:index+1] + suffix
	}
	return suffix
}
