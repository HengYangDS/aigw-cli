package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

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
	if u.GOOS == "windows" && runtime.GOOS == "windows" {
		message, err := u.scheduleWindowsReplacement(binary, tag)
		return message, false, err
	}
	if err := u.replacePortableBinary(binary); err != nil {
		return "", false, err
	}
	return "updated to " + tag, false, nil
}

func (u Updater) replacePortableBinary(binary []byte) error {
	if err := preservePreviousBinary(u.Executable); err != nil {
		return err
	}
	if err := transaction.WriteFileAtomic(u.Executable, binary, 0o755); err != nil {
		return fmt.Errorf("replace AIGW executable: %w", err)
	}
	if err := os.Chmod(u.Executable, 0o755); err != nil {
		return fmt.Errorf("make updated AIGW executable runnable: %w", err)
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

func localCandidateVersion(directory, goos, goarch string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read verified local candidate: %w", err)
	}
	prefix := "aigw_"
	suffix := "_" + goos + "_" + goarch
	if goos == "windows" {
		suffix += ".zip"
	} else {
		suffix += ".tar.gz"
	}
	versions := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		version := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), suffix)
		if _, err := parseVersion("v" + version); err == nil {
			versions = append(versions, version)
		}
	}
	if len(versions) != 1 {
		return "", fmt.Errorf("verified local candidate must contain exactly one portable archive for %s/%s", goos, goarch)
	}
	return versions[0], nil
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
	if u.Channel == "" {
		u.Channel = ChannelPortable
	}
	if u.Channel != ChannelPortable {
		return "", fmt.Errorf("program rollback is available only for a portable installation; use the native package manager for %s", u.Channel)
	}
	if strings.TrimSpace(u.Executable) == "" {
		return "", errors.New("AIGW executable path is empty")
	}
	if u.GOOS == "windows" && runtime.GOOS == "windows" {
		return u.scheduleWindowsRollback()
	}
	current, err := os.ReadFile(u.Executable)
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
	info, err := os.Stat(u.Executable)
	if err != nil {
		return "", fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	if err := transaction.WriteFileAtomic(u.Executable, previous, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("restore previous AIGW executable: %w", err)
	}
	if err := os.Chmod(u.Executable, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("make restored AIGW executable runnable: %w", err)
	}
	if err := transaction.WriteFileAtomic(backup, current, info.Mode().Perm()); err != nil {
		rollbackErr := transaction.WriteFileAtomic(u.Executable, current, info.Mode().Perm())
		if rollbackErr != nil {
			return "", fmt.Errorf("save reversible AIGW rollback copy: %w; restore current binary also failed: %v", err, rollbackErr)
		}
		return "", fmt.Errorf("save reversible AIGW rollback copy failed and current binary was restored: %w", err)
	}
	if err := os.Chmod(backup, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("make reversible AIGW rollback copy runnable: %w", err)
	}
	return "restored the previous program version. If that older program does not support `aigw update --rollback`, download the current portable package and run its installer; it replaces only AIGW and retains one predecessor.", nil
}

func (u Updater) scheduleWindowsRollback() (string, error) {
	backup := rollbackPath(u.Executable)
	previous, err := os.ReadFile(backup)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no previous portable AIGW binary is available")
		}
		return "", fmt.Errorf("read previous AIGW executable: %w", err)
	}
	info, err := os.Stat(u.Executable)
	if err != nil {
		return "", fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	staged := windowsRollbackStagePath(u.Executable)
	if err := os.WriteFile(staged, previous, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("stage Windows AIGW rollback: %w", err)
	}
	if err := os.Chmod(staged, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("make staged Windows AIGW rollback executable: %w", err)
	}
	script := u.Executable + ".rollback.cmd"
	content, err := WindowsRollbackPlan(u.Executable)
	if err != nil {
		_ = os.Remove(staged)
		return "", err
	}
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		_ = os.Remove(staged)
		return "", fmt.Errorf("write Windows AIGW rollback helper: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/B", script)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(script)
		_ = os.Remove(staged)
		return "", fmt.Errorf("start Windows AIGW rollback helper: %w", err)
	}
	return "scheduled restoration of the previous program version; rollback completes after this command exits", nil
}

func preservePreviousBinary(executable string) error {
	previous, err := os.ReadFile(executable)
	if err != nil {
		return fmt.Errorf("read current AIGW executable: %w", err)
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	backup := rollbackPath(executable)
	if err := transaction.WriteFileAtomic(backup, previous, info.Mode().Perm()); err != nil {
		return fmt.Errorf("save previous AIGW executable: %w", err)
	}
	if err := os.Chmod(backup, info.Mode().Perm()); err != nil {
		return fmt.Errorf("make previous AIGW executable runnable: %w", err)
	}
	return nil
}

func rollbackPath(executable string) string {
	if strings.Contains(executable, `\`) && !strings.Contains(executable, "/") {
		directory := executable[:strings.LastIndex(executable, `\`)+1]
		if strings.EqualFold(filepath.Ext(executable), ".exe") {
			return directory + ".aigw.previous.exe"
		}
		return directory + ".aigw.previous"
	}
	directory := filepath.Dir(executable)
	if strings.EqualFold(filepath.Ext(executable), ".exe") {
		return filepath.Join(directory, ".aigw.previous.exe")
	}
	return filepath.Join(directory, ".aigw.previous")
}

func windowsRollbackStagePath(executable string) string {
	return executable + ".rollback"
}

func (u Updater) packageAssetName(version string) string {
	switch u.Channel {
	case ChannelPKG:
		if u.GOOS != "darwin" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_darwin_universal.pkg", version)
	case ChannelDeb:
		if u.GOOS != "linux" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_linux_%s.deb", version, u.GOARCH)
	case ChannelRPM:
		if u.GOOS != "linux" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_linux_%s.rpm", version, u.GOARCH)
	case ChannelMSI:
		if u.GOOS != "windows" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_windows_%s.msi", version, u.GOARCH)
	default:
		return ""
	}
}

func (u Updater) runPackageInstaller(ctx context.Context, path string) error {
	switch u.Channel {
	case ChannelPKG:
		_, err := u.Runner.Run(ctx, "open", path)
		if err != nil {
			return fmt.Errorf("open macOS installer: %w", err)
		}
	case ChannelDeb:
		_, err := u.Runner.Run(ctx, "sudo", "dpkg", "-i", path)
		if err != nil {
			return fmt.Errorf("install deb package: %w", err)
		}
	case ChannelRPM:
		_, err := u.Runner.Run(ctx, "sudo", "rpm", "-Uvh", path)
		if err != nil {
			return fmt.Errorf("install rpm package: %w", err)
		}
	case ChannelMSI:
		_, err := u.Runner.Run(ctx, "msiexec", "/i", path)
		if err != nil {
			return fmt.Errorf("start Windows installer: %w", err)
		}
	default:
		return fmt.Errorf("unknown installation channel %q", u.Channel)
	}
	return nil
}

func detectChannel(executable string) Channel {
	if channel, ok := parseChannel(InstallChannel); ok {
		return channel
	}
	if value := strings.TrimSpace(os.Getenv("AIGW_INSTALL_CHANNEL")); value != "" {
		if channel, ok := parseChannel(value); ok {
			return channel
		}
	}
	dir := filepath.Dir(executable)
	if runtime.GOOS == "darwin" && strings.HasPrefix(executable, "/usr/local/") {
		return ChannelPKG
	}
	if runtime.GOOS == "linux" && (dir == "/usr/bin" || dir == "/usr/local/bin") {
		return ChannelDeb
	}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(executable), `program files\aigw`) {
		return ChannelMSI
	}
	return ChannelPortable
}

func parseChannel(value string) (Channel, bool) {
	switch channel := Channel(strings.ToLower(strings.TrimSpace(value))); channel {
	case ChannelPortable, ChannelPKG, ChannelDeb, ChannelRPM, ChannelMSI:
		return channel, true
	default:
		return "", false
	}
}

func (u Updater) scheduleWindowsReplacement(binary []byte, tag string) (string, error) {
	staged := u.Executable + ".update"
	if err := os.WriteFile(staged, binary, 0o755); err != nil {
		return "", fmt.Errorf("stage Windows update: %w", err)
	}
	script := u.Executable + ".update.cmd"
	content, err := WindowsReplacementPlan(u.Executable)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write Windows update helper: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/B", script)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start Windows update helper: %w", err)
	}
	return "downloaded " + tag + "; the update completes after this command exits", nil
}

// WindowsReplacementPlan returns the delayed replacement script used when the
// running executable cannot be renamed immediately. It retains exactly one
// immediate predecessor beside the portable executable.
func WindowsReplacementPlan(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", errors.New("Windows AIGW executable path is empty")
	}
	staged := executable + ".update"
	previous := rollbackPath(executable)
	return fmt.Sprintf("@echo off\r\nping 127.0.0.1 -n 3 > nul\r\nif exist \"%s\" move /Y \"%s\" \"%s\" > nul\r\nmove /Y \"%s\" \"%s\" > nul\r\ndel \"%%~f0\"\r\n", executable, executable, previous, staged, executable), nil
}

// WindowsRollbackPlan returns the delayed, reversible program-only rollback
// script. It runs after the invoking executable exits, swaps the current and
// previous portable binaries, and restores the original pair if activation of
// the staged predecessor fails. It deliberately has no network operations.
func WindowsRollbackPlan(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", errors.New("Windows AIGW executable path is empty")
	}
	previous := rollbackPath(executable)
	staged := windowsRollbackStagePath(executable)
	return fmt.Sprintf("@echo off\r\nping 127.0.0.1 -n 3 > nul\r\nmove /Y \"%s\" \"%s\" > nul\r\nif errorlevel 1 goto :failed_before_swap\r\nmove /Y \"%s\" \"%s\" > nul\r\nif not errorlevel 1 goto :success\r\nmove /Y \"%s\" \"%s\" > nul\r\nif errorlevel 1 goto :failed\r\nmove /Y \"%s\" \"%s\" > nul\r\ngoto :failed\r\n:failed_before_swap\r\ndel \"%s\" > nul 2>&1\r\n:failed\r\ndel \"%%~f0\"\r\nexit /b 1\r\n:success\r\ndel \"%%~f0\"\r\nexit /b 0\r\n", executable, previous, staged, executable, previous, executable, staged, previous, staged), nil
}
