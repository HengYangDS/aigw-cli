package shims

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const marker = "AIGW managed Claude shim"

type Manager struct {
	GOOS           string
	BinDir         string
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
	return strings.Contains(string(data), marker), nil
}

func (m Manager) EnableClaude() (string, error) {
	path := m.claudePath()
	if data, err := os.ReadFile(path); err == nil && !strings.Contains(string(data), marker) {
		return "", fmt.Errorf("existing Claude launcher %s is not owned by AIGW; move it or choose another AIGW bin directory", path)
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
	return path, nil
}

func (m Manager) DisableClaude() error {
	path := m.claudePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Claude launcher: %w", err)
	}
	if !strings.Contains(string(data), marker) {
		return fmt.Errorf("Claude launcher %s is not owned by AIGW; refusing to remove it", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove Claude launcher: %w", err)
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
		return "@echo off\r\nREM " + marker + "\r\n\"%~dp0aigw.exe\" __run-claude %*\r\n"
	}
	executable := strings.ReplaceAll(m.AIGWExecutable, `'`, `'\''`)
	return "#!/bin/sh\n# " + marker + "\nexec '" + executable + "' __run-claude \"$@\"\n"
}
