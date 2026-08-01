package claude

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLauncherTargetReportsFilesystemErrors(t *testing.T) {
	t.Run("symbolic link loop", func(t *testing.T) {
		loop := filepath.Join(t.TempDir(), "loop")
		if err := os.Symlink(loop, loop); err != nil {
			t.Fatal(err)
		}
		ready, err := validateLauncherTarget(loop, false)
		if err == nil || ready || !strings.Contains(err.Error(), "inspect AIGW-managed Claude launcher target") {
			t.Fatalf("validateLauncherTarget() = %v, %v", ready, err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		ready, err := validateLauncherTarget(t.TempDir(), false)
		if err == nil || ready || !strings.Contains(err.Error(), "target is unavailable") {
			t.Fatalf("validateLauncherTarget() = %v, %v", ready, err)
		}
	})
}

func TestUnixLauncherTargetParsing(t *testing.T) {
	tests := []struct {
		name    string
		content string
		target  string
		ok      bool
	}{
		{name: "no exec line", content: "# managed\nexit 1\n"},
		{name: "valid", content: "#!/bin/sh\nexec '/opt/aigw' __run-claude \"$@\"\n", target: "/opt/aigw", ok: true},
		{name: "empty target", content: "exec '' __run-claude \"$@\"\n"},
		{name: "missing command", content: "exec '/opt/aigw' other\n", target: "/opt/aigw' other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, ok := unixLauncherTarget(tt.content)
			if target != tt.target || ok != tt.ok {
				t.Fatalf("unixLauncherTarget() = %q, %v; want %q, %v", target, ok, tt.target, tt.ok)
			}
		})
	}
}

func TestWindowsLauncherTargetParsing(t *testing.T) {
	tests := []struct {
		name    string
		content string
		target  string
		ok      bool
	}{
		{name: "no command line", content: "@echo off\r\nREM managed\r\n"},
		{name: "unclosed quote", content: `"C:\aigw.exe __run-claude %*`},
		{name: "empty target", content: `"" __run-claude %*`},
		{name: "invalid suffix", content: `"C:\aigw.exe" --wrong`},
		{name: "valid", content: "@echo off\r\n\"C:\\aigw.exe\" __run-claude %*\r\n", target: `C:\aigw.exe`, ok: true},
		{name: "escaped quote", content: `"C:\AIGW ""portable""\aigw.exe" __run-claude %*`, target: `C:\AIGW "portable"\aigw.exe`, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, ok := windowsLauncherTarget(tt.content)
			if target != tt.target || ok != tt.ok {
				t.Fatalf("windowsLauncherTarget() = %q, %v; want %q, %v", target, ok, tt.target, tt.ok)
			}
		})
	}
}

func TestRemoveManagedPathBlock(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "absent", text: "export PATH=/custom\n", want: "export PATH=/custom\n"},
		{name: "middle", text: "before\n" + pathBegin + "\nmanaged\n" + pathEnd + "\nafter\n", want: "before\nafter\n"},
		{name: "only block", text: pathBegin + "\nmanaged\n" + pathEnd, want: "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeManagedPathBlock(tt.text); got != tt.want {
				t.Fatalf("removeManagedPathBlock() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestRestoreClaudeStateRejectsChangedPaths(t *testing.T) {
	manager := Launcher{GOOS: "linux", BinDir: t.TempDir(), Home: t.TempDir(), Shell: "/bin/sh"}
	before, err := manager.CaptureClaudeState()
	if err != nil {
		t.Fatal(err)
	}

	differentLauncher := Launcher{GOOS: "linux", BinDir: t.TempDir(), Home: manager.Home, Shell: manager.Shell}
	launcherState, err := differentLauncher.CaptureClaudeState()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreClaudeState(before, launcherState); err == nil || !strings.Contains(err.Error(), "launcher path changed") {
		t.Fatalf("launcher mismatch error = %v", err)
	}

	differentActivation := Launcher{GOOS: "linux", BinDir: manager.BinDir, Home: t.TempDir(), Shell: manager.Shell}
	activationState, err := differentActivation.CaptureClaudeState()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreClaudeState(before, activationState); err == nil || !strings.Contains(err.Error(), "activation path changed") {
		t.Fatalf("activation mismatch error = %v", err)
	}
}

func TestRestoreClaudeStateRejectsConcurrentEditsToBothFiles(t *testing.T) {
	binDir := t.TempDir()
	home := t.TempDir()
	manager := Launcher{GOOS: "linux", BinDir: binDir, Home: home, Shell: "/bin/sh"}
	launcher := filepath.Join(binDir, "claude")
	profile := filepath.Join(home, ".profile")
	if err := os.WriteFile(launcher, []byte("launcher-before"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("profile-before"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := manager.CaptureClaudeState()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("launcher-after"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("profile-after"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := manager.CaptureClaudeState()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("launcher-external"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("profile-external"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = manager.RestoreClaudeState(before, after)
	if err == nil || strings.Count(err.Error(), "postimage changed") != 2 {
		t.Fatalf("RestoreClaudeState() error = %v", err)
	}
	for path, want := range map[string]string{launcher: "launcher-external", profile: "profile-external"} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != want {
			t.Fatalf("concurrent edit at %s = %q, %v", path, got, readErr)
		}
	}
}

func TestRollbackClaudeEnableReportsCaptureAndRestoreErrors(t *testing.T) {
	cause := errors.New("activation failed")

	t.Run("capture postimage", func(t *testing.T) {
		manager := Launcher{GOOS: "linux", BinDir: t.TempDir()}
		before, err := manager.CaptureClaudeState()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(manager.claudePath(), 0o700); err != nil {
			t.Fatal(err)
		}
		err = manager.rollbackClaudeEnable(before, cause)
		if !errors.Is(err, cause) || !strings.Contains(err.Error(), "capture Claude launcher rollback postimage") {
			t.Fatalf("rollback error = %v", err)
		}
	})

	t.Run("restore path guard", func(t *testing.T) {
		manager := Launcher{GOOS: "linux", BinDir: t.TempDir()}
		other := Launcher{GOOS: "linux", BinDir: t.TempDir()}
		before, err := other.CaptureClaudeState()
		if err != nil {
			t.Fatal(err)
		}
		err = manager.rollbackClaudeEnable(before, cause)
		if !errors.Is(err, cause) || !strings.Contains(err.Error(), "restore Claude launcher state") || !strings.Contains(err.Error(), "launcher path changed") {
			t.Fatalf("rollback error = %v", err)
		}
	})
}

func TestShellProfileRequiresHome(t *testing.T) {
	manager := Launcher{}
	if profiles := manager.shellProfiles(); profiles != nil {
		t.Fatalf("shellProfiles() = %#v", profiles)
	}
	if _, err := manager.shellProfile(); err == nil || !strings.Contains(err.Error(), "HOME is not set") {
		t.Fatalf("shellProfile() error = %v", err)
	}
}
