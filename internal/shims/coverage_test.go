package shims_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func TestManagerMaintainsShellActivationProfiles(t *testing.T) {
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
		{name: "fish", shell: "/usr/local/bin/fish", initialPath: ".config/fish/config.fish", initial: "set -g fish_greeting\n", profilePath: ".config/fish/config.fish", activation: "fish_add_path --prepend --move"},
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
			manager := shims.Manager{GOOS: "linux", Home: home, Shell: tt.shell, BinDir: binDir}

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
			if !strings.Contains(text, tt.initial) || !strings.Contains(text, tt.activation) || !strings.Contains(text, binDir) || !strings.Contains(text, "# >>> AIGW Claude shim PATH >>>") || !strings.Contains(text, "# <<< AIGW Claude shim PATH <<<") {
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
			if strings.Contains(string(removed), "AIGW Claude shim PATH") || !strings.Contains(string(removed), tt.initial) {
				t.Fatalf("profile after removal = %q", removed)
			}
		})
	}
}

func TestManagerActivationShortCircuitsWithoutUnixHome(t *testing.T) {
	for _, manager := range []shims.Manager{
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

func TestManagerReportsActivationPathTypeErrors(t *testing.T) {
	home := t.TempDir()
	profile := filepath.Join(home, ".zshrc")
	if err := os.Mkdir(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	manager := shims.Manager{GOOS: "linux", Home: home, Shell: "/bin/zsh", BinDir: binDir, AIGWExecutable: "/bin/sh"}

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

func TestManagerReportsLauncherPathTypeErrors(t *testing.T) {
	binDir := t.TempDir()
	launcher := filepath.Join(binDir, "claude")
	if err := os.Mkdir(launcher, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "linux", BinDir: binDir, AIGWExecutable: "/bin/sh"}

	if ready, err := manager.ClaudeShimReady(); err == nil || ready || !strings.Contains(err.Error(), "inspect Claude launcher") {
		t.Fatalf("shim readiness = %v, %v", ready, err)
	}
	if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "inspect Claude launcher") {
		t.Fatalf("EnableClaude() error = %v", err)
	}
	if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "inspect Claude launcher") {
		t.Fatalf("DisableClaude() error = %v", err)
	}
}

func TestManagerAcceptsOwnedShimOnUnspecifiedPlatform(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("# AIGW managed Claude shim\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "plan9", BinDir: binDir}
	ready, err := manager.ClaudeShimReady()
	if err != nil || !ready {
		t.Fatalf("unspecified-platform readiness = %v, %v", ready, err)
	}
}

func TestManagerRejectsMalformedOwnedShims(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		file    string
		content string
	}{
		{name: "Unix", goos: "linux", file: "claude", content: "# AIGW managed Claude shim\nexec malformed\n"},
		{name: "Windows", goos: "windows", file: "claude.cmd", content: "REM AIGW managed Claude shim\r\n\"unclosed\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(binDir, tt.file), []byte(tt.content), 0o700); err != nil {
				t.Fatal(err)
			}
			manager := shims.Manager{GOOS: tt.goos, BinDir: binDir}
			ready, err := manager.ClaudeShimReady()
			if err == nil || ready || !strings.Contains(err.Error(), "invalid target") {
				t.Fatalf("malformed shim readiness = %v, %v", ready, err)
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
			launcher := filepath.Join(binDir, "claude")
			original := []byte("# AIGW managed Claude shim\n")
			if launcherExists {
				if err := os.WriteFile(launcher, original, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			manager := shims.Manager{GOOS: "linux", Home: home, Shell: "/bin/sh", BinDir: binDir}
			if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "read Claude shell activation") {
				t.Fatalf("DisableClaude() error = %v", err)
			}
			if launcherExists {
				got, err := os.ReadFile(launcher)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("owned launcher changed: %q, %v", got, err)
				}
			} else if _, err := os.Stat(launcher); !os.IsNotExist(err) {
				t.Fatalf("missing launcher was created: %v", err)
			}
		})
	}
}
