// Package claude owns the process-only Claude route and its AIGW-managed
// launcher. It does not persist provider credentials in Claude-owned files.
package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/transaction"
)

const marker = "AIGW managed Claude launcher"

const (
	pathBegin = "# >>> AIGW Claude launcher PATH >>>"
	pathEnd   = "# <<< AIGW Claude launcher PATH <<<"
)

type Launcher struct {
	GOOS           string
	BinDir         string
	Home           string
	Shell          string
	AIGWExecutable string
}

// LauncherStateSnapshot is a byte-exact transaction boundary for the managed
// launcher and shell activation file. Its fields stay private so callers can
// only capture and restore state through Launcher.
type LauncherStateSnapshot struct {
	launcherPath   string
	launcher       transaction.FileSnapshot
	activationPath string
	activation     transaction.FileSnapshot
}

func (m Launcher) CaptureClaudeState() (LauncherStateSnapshot, error) {
	launcherPath := m.claudePath()
	launcher, err := transaction.CaptureFileSnapshot(launcherPath)
	if err != nil {
		return LauncherStateSnapshot{}, err
	}
	snapshot := LauncherStateSnapshot{launcherPath: launcherPath, launcher: launcher}
	if m.GOOS == "windows" || m.Home == "" {
		return snapshot, nil
	}
	activationPath, err := m.shellProfile()
	if err != nil {
		return LauncherStateSnapshot{}, err
	}
	activation, err := transaction.CaptureFileSnapshot(activationPath)
	if err != nil {
		return LauncherStateSnapshot{}, err
	}
	snapshot.activationPath = activationPath
	snapshot.activation = activation
	return snapshot, nil
}

func (m Launcher) RestoreClaudeState(before, after LauncherStateSnapshot) error {
	if before.launcherPath != after.launcherPath || before.launcherPath != m.claudePath() {
		return fmt.Errorf("Claude launcher path changed during rollback")
	}
	if before.activationPath != after.activationPath {
		return fmt.Errorf("Claude activation path changed during rollback")
	}
	var restoreErr error
	if before.activationPath != "" {
		restoreErr = errors.Join(restoreErr, transaction.RestoreFileAtomicIfPostimage(before.activationPath, before.activation, after.activation))
	}
	restoreErr = errors.Join(restoreErr, transaction.RestoreFileAtomicIfPostimage(before.launcherPath, before.launcher, after.launcher))
	return restoreErr
}

// ClaudeLauncherReady reports whether the expected launcher exists and is owned
// by AIGW. A different executable named "claude" is not a substitute: it
// cannot provide AIGW's process-bound credential mapping.
func (m Launcher) ClaudeLauncherReady() (bool, error) {
	data, err := os.ReadFile(m.claudePath())
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Claude launcher: %w", err)
	}
	content := string(data)
	if !strings.Contains(content, marker) {
		return false, nil
	}
	if m.GOOS == "darwin" || m.GOOS == "linux" {
		target, ok := unixLauncherTarget(content)
		if !ok {
			return false, fmt.Errorf("AIGW-managed Claude launcher has an invalid target; run `aigw repair`")
		}
		return validateLauncherTarget(target, true)
	}
	if m.GOOS == "windows" {
		target, ok := windowsLauncherTarget(content)
		if !ok {
			return false, fmt.Errorf("AIGW-managed Claude launcher has an invalid target; run `aigw repair`")
		}
		return validateLauncherTarget(target, false)
	}
	return true, nil
}

func validateLauncherTarget(target string, requireExecutableBit bool) (bool, error) {
	if requireExecutableBit && isTemporaryPath(target) {
		return false, fmt.Errorf("AIGW-managed Claude launcher target is in a temporary directory: %s; run `aigw repair`", target)
	}
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("AIGW-managed Claude launcher target is unavailable: %s; run `aigw repair`", target)
	}
	if err != nil {
		return false, fmt.Errorf("inspect AIGW-managed Claude launcher target: %w", err)
	}
	if info.IsDir() || (requireExecutableBit && info.Mode()&0o111 == 0) {
		return false, fmt.Errorf("AIGW-managed Claude launcher target is unavailable: %s; run `aigw repair`", target)
	}
	return true, nil
}

func unixLauncherTarget(content string) (string, bool) {
	const prefix = "exec '"
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimPrefix(line, prefix)
		target, _, found := strings.Cut(rest, "' __run-claude")
		return target, found && target != ""
	}
	return "", false
}

func windowsLauncherTarget(content string) (string, bool) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if !strings.HasPrefix(line, `"`) {
			continue
		}
		rest := line[1:]
		closing := -1
		for index := 0; index < len(rest); index++ {
			if rest[index] != '"' {
				continue
			}
			if index+1 < len(rest) && rest[index+1] == '"' {
				index++
				continue
			}
			closing = index
			break
		}
		if closing < 0 {
			return "", false
		}
		target := strings.ReplaceAll(rest[:closing], `""`, `"`)
		if target == "" || strings.TrimSpace(rest[closing+1:]) != "__run-claude %*" {
			return "", false
		}
		return target, true
	}
	return "", false
}

// isTemporaryPath recognizes the well-known Unix temporary directories using
// a portable, forward-slash comparison. filepath.Clean renders separators
// for the compiling GOOS, not for the Unix path this string represents, so a
// literal backslash comparison would silently stop matching whenever these
// tests (or a cross-compiled binary) run with Windows path semantics.
func isTemporaryPath(path string) bool {
	portablePath := strings.ReplaceAll(filepath.Clean(path), "\\", "/")
	return portablePath == "/tmp" || strings.HasPrefix(portablePath, "/tmp/") ||
		portablePath == "/private/tmp" || strings.HasPrefix(portablePath, "/private/tmp/") ||
		portablePath == "/var/folders" || strings.HasPrefix(portablePath, "/var/folders/")
}

func (m Launcher) EnableClaude() (string, error) {
	if isEphemeralBuildExecutable(m.AIGWExecutable) {
		return "", fmt.Errorf("refusing to write AIGW-managed Claude launcher from temporary build executable %s; install AIGW or run its persistent binary", m.AIGWExecutable)
	}
	path := m.claudePath()
	if data, err := os.ReadFile(path); err == nil && !strings.Contains(string(data), marker) {
		return "", fmt.Errorf("existing Claude launcher %s is not owned by AIGW; move it or choose another AIGW bin directory", path)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect Claude launcher: %w", err)
	}
	before, err := m.CaptureClaudeState()
	if err != nil {
		return "", err
	}
	content := m.claudeContent()
	if err := transaction.WriteFileAtomic(path, []byte(content), 0o755); err != nil {
		return "", m.rollbackClaudeEnable(before, err)
	}
	if m.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			return "", m.rollbackClaudeEnable(before, fmt.Errorf("make Claude launcher executable: %w", err))
		}
	}
	if err := m.EnsureClaudeActivation(); err != nil {
		return "", m.rollbackClaudeEnable(before, err)
	}
	return path, nil
}

func (m Launcher) rollbackClaudeEnable(before LauncherStateSnapshot, cause error) error {
	after, err := m.CaptureClaudeState()
	if err != nil {
		return fmt.Errorf("%w; capture Claude launcher rollback postimage: %v", cause, err)
	}
	if err := m.RestoreClaudeState(before, after); err != nil {
		return fmt.Errorf("%w; restore Claude launcher state: %v", cause, err)
	}
	return cause
}

// isEphemeralBuildExecutable identifies Go's short-lived source-run build
// outputs. A generic temporary directory remains a valid test or portable
// execution location; only these recognizable compiler workspaces must never
// become a persistent Claude launcher target.
func isEphemeralBuildExecutable(path string) bool {
	portablePath := strings.ReplaceAll(filepath.Clean(path), "\\", "/")
	for _, part := range strings.Split(portablePath, "/") {
		if strings.HasPrefix(part, "aigw-go-preview-") || strings.HasPrefix(part, "go-build") {
			return true
		}
	}
	return false
}

func (m Launcher) DisableClaude() error {
	path := m.claudePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := m.RemoveClaudeActivation(); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Claude launcher: %w", err)
	}
	if !strings.Contains(string(data), marker) {
		return fmt.Errorf("Claude launcher %s is not owned by AIGW; refusing to remove it", path)
	}
	if err := m.RemoveClaudeActivation(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove Claude launcher: %w", err)
	}
	return nil
}

// ClaudeActivationReady verifies the persistent user-shell PATH projection.
// It deliberately does not inspect the current process PATH: a CLI cannot
// mutate its parent shell, while a new interactive shell will load this block.
func (m Launcher) ClaudeActivationReady() (bool, error) {
	if m.GOOS == "windows" || m.Home == "" {
		return true, nil
	}
	profile, err := m.shellProfile()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(profile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Claude shell activation: %w", err)
	}
	text := string(data)
	return strings.Contains(text, pathBegin) && strings.Contains(text, pathEnd) && strings.Contains(text, m.BinDir), nil
}

func (m Launcher) EnsureClaudeActivation() error {
	if m.GOOS == "windows" || m.Home == "" {
		return nil
	}
	profile, err := m.shellProfile()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read Claude shell activation: %w", err)
	}
	updated := replaceManagedPathBlock(string(data), m.pathBlock())
	if updated == string(data) {
		return nil
	}
	if err := transaction.WriteFileAtomic(profile, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write Claude shell activation: %w", err)
	}
	return nil
}

func (m Launcher) RemoveClaudeActivation() error {
	if m.GOOS == "windows" || m.Home == "" {
		return nil
	}
	for _, profile := range m.shellProfiles() {
		data, err := os.ReadFile(profile)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read Claude shell activation: %w", err)
		}
		updated := removeManagedPathBlock(string(data))
		if updated == string(data) {
			continue
		}
		if err := transaction.WriteFileAtomic(profile, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("remove Claude shell activation: %w", err)
		}
	}
	return nil
}

func (m Launcher) claudePath() string {
	if m.GOOS == "windows" {
		return filepath.Join(m.BinDir, "claude.cmd")
	}
	return filepath.Join(m.BinDir, "claude")
}

func (m Launcher) claudeContent() string {
	if m.GOOS == "windows" {
		executable := strings.ReplaceAll(m.AIGWExecutable, `"`, `""`)
		return "@echo off\r\nREM " + marker + "\r\n\"" + executable + "\" __run-claude %*\r\n"
	}
	executable := strings.ReplaceAll(m.AIGWExecutable, `'`, `'\''`)
	return "#!/bin/sh\n# " + marker + "\nexec '" + executable + "' __run-claude \"$@\"\n"
}

func (m Launcher) shellProfile() (string, error) {
	profiles := m.shellProfiles()
	if len(profiles) == 0 {
		return "", fmt.Errorf("resolve Claude shell activation profile: HOME is not set")
	}
	return profiles[0], nil
}

func (m Launcher) shellProfiles() []string {
	if m.Home == "" {
		return nil
	}
	shell := strings.ToLower(filepath.Base(m.Shell))
	primary := filepath.Join(m.Home, ".profile")
	switch shell {
	case "zsh":
		primary = filepath.Join(m.Home, ".zshrc")
	case "bash":
		bashProfile := filepath.Join(m.Home, ".bash_profile")
		if _, err := os.Stat(bashProfile); err == nil {
			primary = bashProfile
		} else {
			primary = filepath.Join(m.Home, ".bashrc")
		}
	case "fish":
		primary = filepath.Join(m.Home, ".config", "fish", "configuration.fish")
	}
	all := []string{primary}
	for _, candidate := range []string{
		filepath.Join(m.Home, ".zshrc"),
		filepath.Join(m.Home, ".bash_profile"),
		filepath.Join(m.Home, ".bashrc"),
		filepath.Join(m.Home, ".profile"),
		filepath.Join(m.Home, ".config", "fish", "configuration.fish"),
	} {
		if candidate != primary {
			all = append(all, candidate)
		}
	}
	return all
}

func (m Launcher) pathBlock() string {
	dir := strings.ReplaceAll(m.BinDir, `'`, `'\''`)
	line := "export PATH='" + dir + `':$PATH`
	if strings.EqualFold(filepath.Base(m.Shell), "fish") {
		line = "fish_add_path --prepend --move '" + dir + "'"
	}
	return "\n" + pathBegin + "\n" + line + "\n" + pathEnd + "\n"
}

func replaceManagedPathBlock(text, block string) string {
	text = removeManagedPathBlock(text)
	return strings.TrimRight(text, "\r\n") + block
}

func removeManagedPathBlock(text string) string {
	if !strings.Contains(text, pathBegin) {
		return text
	}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		if line == pathBegin {
			skipping = true
			continue
		}
		if line == pathEnd && skipping {
			skipping = false
			continue
		}
		if !skipping {
			kept = append(kept, line)
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\r\n") + "\n"
}
