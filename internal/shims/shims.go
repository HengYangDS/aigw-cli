package shims

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const marker = "AIGW managed Claude shim"

const (
	pathBegin = "# >>> AIGW Claude shim PATH >>>"
	pathEnd   = "# <<< AIGW Claude shim PATH <<<"
)

type Manager struct {
	GOOS           string
	BinDir         string
	Home           string
	Shell          string
	AIGWExecutable string
}

// ClaudeShimReady reports whether the expected launcher exists and is owned
// by AIGW. A different executable named "claude" is not a substitute: it
// cannot provide AIGW's process-bound credential mapping.
func (m Manager) ClaudeShimReady() (bool, error) {
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
		target, ok := unixShimTarget(content)
		if !ok {
			return false, fmt.Errorf("AIGW-managed Claude shim has an invalid target; run `aigw repair`")
		}
		return validateShimTarget(target, true)
	}
	if m.GOOS == "windows" {
		target, ok := windowsShimTarget(content)
		if !ok {
			return false, fmt.Errorf("AIGW-managed Claude shim has an invalid target; run `aigw repair`")
		}
		return validateShimTarget(target, false)
	}
	return true, nil
}

func validateShimTarget(target string, requireExecutableBit bool) (bool, error) {
	if requireExecutableBit && isTemporaryPath(target) {
		return false, fmt.Errorf("AIGW-managed Claude shim target is in a temporary directory: %s; run `aigw repair`", target)
	}
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("AIGW-managed Claude shim target is unavailable: %s; run `aigw repair`", target)
	}
	if err != nil {
		return false, fmt.Errorf("inspect AIGW-managed Claude shim target: %w", err)
	}
	if info.IsDir() || (requireExecutableBit && info.Mode()&0o111 == 0) {
		return false, fmt.Errorf("AIGW-managed Claude shim target is unavailable: %s; run `aigw repair`", target)
	}
	return true, nil
}

func unixShimTarget(content string) (string, bool) {
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

func windowsShimTarget(content string) (string, bool) {
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

func isTemporaryPath(path string) bool {
	clean := filepath.Clean(path)
	return clean == "/tmp" || strings.HasPrefix(clean, "/tmp/") ||
		clean == "/private/tmp" || strings.HasPrefix(clean, "/private/tmp/") ||
		clean == "/var/folders" || strings.HasPrefix(clean, "/var/folders/")
}

func (m Manager) EnableClaude() (string, error) {
	path := m.claudePath()
	existed := false
	if data, err := os.ReadFile(path); err == nil && !strings.Contains(string(data), marker) {
		return "", fmt.Errorf("existing Claude launcher %s is not owned by AIGW; move it or choose another AIGW bin directory", path)
	} else if err == nil {
		existed = true
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect Claude launcher: %w", err)
	}
	content := m.claudeContent()
	if err := transaction.WriteFileAtomic(path, []byte(content), 0o755); err != nil {
		return "", err
	}
	if m.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			return "", fmt.Errorf("make Claude launcher executable: %w", err)
		}
	}
	if err := m.EnsureClaudeActivation(); err != nil {
		if !existed {
			_ = os.Remove(path)
		}
		return "", err
	}
	return path, nil
}

func (m Manager) DisableClaude() error {
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
func (m Manager) ClaudeActivationReady() (bool, error) {
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

func (m Manager) EnsureClaudeActivation() error {
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

func (m Manager) RemoveClaudeActivation() error {
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

func (m Manager) claudePath() string {
	if m.GOOS == "windows" {
		return filepath.Join(m.BinDir, "claude.cmd")
	}
	return filepath.Join(m.BinDir, "claude")
}

func (m Manager) claudeContent() string {
	if m.GOOS == "windows" {
		executable := strings.ReplaceAll(m.AIGWExecutable, `"`, `""`)
		return "@echo off\r\nREM " + marker + "\r\n\"" + executable + "\" __run-claude %*\r\n"
	}
	executable := strings.ReplaceAll(m.AIGWExecutable, `'`, `'\''`)
	return "#!/bin/sh\n# " + marker + "\nexec '" + executable + "' __run-claude \"$@\"\n"
}

func (m Manager) shellProfile() (string, error) {
	profiles := m.shellProfiles()
	if len(profiles) == 0 {
		return "", fmt.Errorf("resolve Claude shell activation profile: HOME is not set")
	}
	return profiles[0], nil
}

func (m Manager) shellProfiles() []string {
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
		primary = filepath.Join(m.Home, ".config", "fish", "config.fish")
	}
	all := []string{primary}
	for _, candidate := range []string{
		filepath.Join(m.Home, ".zshrc"),
		filepath.Join(m.Home, ".bash_profile"),
		filepath.Join(m.Home, ".bashrc"),
		filepath.Join(m.Home, ".profile"),
		filepath.Join(m.Home, ".config", "fish", "config.fish"),
	} {
		if candidate != primary {
			all = append(all, candidate)
		}
	}
	return all
}

func (m Manager) pathBlock() string {
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
