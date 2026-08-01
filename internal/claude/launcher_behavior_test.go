package claude_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
)

func TestLauncherMaintainsShellActivationProfiles(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		initialPath string
		initial     string
		profilePath string
		activation  string
	}{
		{name: "zsh", shell: "/bin/zsh", initialPath: ".zshrc", initial: "setopt autocd\n", profilePath: ".zshrc", activation: "export PATH="},
		{name: "missing zsh profile", shell: "/bin/zsh", profilePath: ".zshrc", activation: "export PATH="},
		{name: "bash profile", shell: "/bin/bash", initialPath: ".bash_profile", initial: "source ~/.bashrc\n", profilePath: ".bash_profile", activation: "export PATH="},
		{name: "bash rc fallback", shell: "/bin/bash", initialPath: ".bashrc", initial: "shopt -s histappend\n", profilePath: ".bashrc", activation: "export PATH="},
		{name: "fish", shell: "/usr/local/bin/fish", initialPath: ".config/fish/configuration.fish", initial: "set -g fish_greeting\n", profilePath: ".config/fish/configuration.fish", activation: "fish_add_path --prepend --move"},
		{name: "other shell", shell: "/bin/sh", initialPath: ".profile", initial: "umask 077\n", profilePath: ".profile", activation: "export PATH="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			binDir := filepath.Join(home, "aigw", "bin")
			if tt.initialPath != "" {
				path := filepath.Join(home, tt.initialPath)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tt.initial), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			manager := claude.Launcher{GOOS: "linux", Home: home, Shell: tt.shell, BinDir: binDir}

			ready, err := manager.ClaudeActivationReady()
			if err != nil || ready {
				t.Fatalf("readiness before activation = %v, %v", ready, err)
			}
			if err := manager.EnsureClaudeActivation(); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(home, tt.profilePath)
			activated, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			text := string(activated)
			if !strings.Contains(text, tt.initial) || !strings.Contains(text, tt.activation) || !strings.Contains(text, binDir) || !strings.Contains(text, "# >>> AIGW Claude launcher PATH >>>") || !strings.Contains(text, "# <<< AIGW Claude launcher PATH <<<") {
				t.Fatalf("activated profile = %q", text)
			}
			ready, err = manager.ClaudeActivationReady()
			if err != nil || !ready {
				t.Fatalf("readiness after activation = %v, %v", ready, err)
			}

			if err := manager.EnsureClaudeActivation(); err != nil {
				t.Fatal(err)
			}
			unchanged, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(unchanged, activated) {
				t.Fatalf("idempotent activation changed profile: %q", unchanged)
			}

			if err := manager.RemoveClaudeActivation(); err != nil {
				t.Fatal(err)
			}
			removed, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(removed), "AIGW Claude launcher PATH") || !strings.Contains(string(removed), tt.initial) {
				t.Fatalf("profile after removal = %q", removed)
			}
		})
	}
}

func TestLauncherActivationShortCircuitsWithoutUnixHome(t *testing.T) {
	for _, manager := range []claude.Launcher{
		{GOOS: "windows", Home: t.TempDir(), Shell: "/bin/zsh", BinDir: t.TempDir()},
		{GOOS: "linux", Home: "", Shell: "/bin/zsh", BinDir: t.TempDir()},
	} {
		ready, err := manager.ClaudeActivationReady()
		if err != nil || !ready {
			t.Fatalf("short-circuit readiness = %v, %v", ready, err)
		}
		if err := manager.EnsureClaudeActivation(); err != nil {
			t.Fatal(err)
		}
		if err := manager.RemoveClaudeActivation(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLauncherReportsActivationPathTypeErrors(t *testing.T) {
	home := t.TempDir()
	profile := filepath.Join(home, ".zshrc")
	if err := os.Mkdir(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	manager := claude.Launcher{GOOS: "linux", Home: home, Shell: "/bin/zsh", BinDir: binDir, AIGWExecutable: "/bin/sh"}

	if ready, err := manager.ClaudeActivationReady(); err == nil || ready || !strings.Contains(err.Error(), "inspect Claude shell activation") {
		t.Fatalf("activation readiness = %v, %v", ready, err)
	}
	if err := manager.EnsureClaudeActivation(); err == nil || !strings.Contains(err.Error(), "read Claude shell activation") {
		t.Fatalf("EnsureClaudeActivation() error = %v", err)
	}
	if err := manager.RemoveClaudeActivation(); err == nil || !strings.Contains(err.Error(), "read Claude shell activation") {
		t.Fatalf("RemoveClaudeActivation() error = %v", err)
	}
	if _, err := manager.CaptureClaudeState(); err == nil || !strings.Contains(err.Error(), "read "+profile) {
		t.Fatalf("CaptureClaudeState() error = %v", err)
	}
	if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "read "+profile) {
		t.Fatalf("EnableClaude() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "claude")); !os.IsNotExist(err) {
		t.Fatalf("failed enable wrote launcher: %v", err)
	}
}

func TestLauncherReportsLauncherPathTypeErrors(t *testing.T) {
	binDir := t.TempDir()
	launcherPath := filepath.Join(binDir, "claude")
	if err := os.Mkdir(launcherPath, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{GOOS: "linux", BinDir: binDir, AIGWExecutable: "/bin/sh"}

	if ready, err := manager.ClaudeLauncherReady(); err == nil || ready || !strings.Contains(err.Error(), "inspect Claude launcher") {
		t.Fatalf("launcher readiness = %v, %v", ready, err)
	}
	if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "inspect Claude launcher") {
		t.Fatalf("EnableClaude() error = %v", err)
	}
	if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "inspect Claude launcher") {
		t.Fatalf("DisableClaude() error = %v", err)
	}
}

func TestLauncherAcceptsOwnedLauncherOnUnspecifiedPlatform(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("# AIGW managed Claude launcher\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{GOOS: "plan9", BinDir: binDir}
	ready, err := manager.ClaudeLauncherReady()
	if err != nil || !ready {
		t.Fatalf("unspecified-platform readiness = %v, %v", ready, err)
	}
}

func TestLauncherRejectsMalformedOwnedClaudeLauncher(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		file    string
		content string
	}{
		{name: "Unix", goos: "linux", file: "claude", content: "# AIGW managed Claude launcher\nexec malformed\n"},
		{name: "Windows", goos: "windows", file: "claude.cmd", content: "REM AIGW managed Claude launcher\r\n\"unclosed\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(binDir, tt.file), []byte(tt.content), 0o700); err != nil {
				t.Fatal(err)
			}
			manager := claude.Launcher{GOOS: tt.goos, BinDir: binDir}
			ready, err := manager.ClaudeLauncherReady()
			if err == nil || ready || !strings.Contains(err.Error(), "invalid target") {
				t.Fatalf("malformed launcher readiness = %v, %v", ready, err)
			}
		})
	}
}

func TestDisableClaudePreservesStateWhenActivationCannotBeRead(t *testing.T) {
	for _, launcherExists := range []bool{false, true} {
		name := "missing launcher"
		if launcherExists {
			name = "owned launcher"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.Mkdir(filepath.Join(home, ".profile"), 0o700); err != nil {
				t.Fatal(err)
			}
			binDir := t.TempDir()
			launcherPath := filepath.Join(binDir, "claude")
			original := []byte("# AIGW managed Claude launcher\n")
			if launcherExists {
				if err := os.WriteFile(launcherPath, original, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			manager := claude.Launcher{GOOS: "linux", Home: home, Shell: "/bin/sh", BinDir: binDir}
			if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "read Claude shell activation") {
				t.Fatalf("DisableClaude() error = %v", err)
			}
			if launcherExists {
				got, err := os.ReadFile(launcherPath)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("owned launcher changed: %q, %v", got, err)
				}
			} else if _, err := os.Stat(launcherPath); !os.IsNotExist(err) {
				t.Fatalf("missing launcher was created: %v", err)
			}
		})
	}
}
