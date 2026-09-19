package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/upgrade"
)

func TestInstallCommandUsesPlatformDefaultTarget(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	originalPath := os.Getenv("PATH")
	root := t.TempDir()
	source := filepath.Join(root, "download", "aigw")
	target := filepath.Join(root, "bin", "aigw")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	command := NewInstallCommand(invocation.Context{Executable: source, InstallTarget: target, Out: out})
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "current" {
		t.Fatalf("installed program = %q, %v", data, err)
	}
	if !strings.Contains(out.String(), target) || !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "PATH is unchanged") ||
		!strings.Contains(out.String(), "installed path directly") || os.Getenv("PATH") != originalPath {
		t.Fatalf("installation did not explain unchanged command discovery: %q", out.String())
	}
}

func TestInstallationCommandReportsHumanStateAndOutputFailures(t *testing.T) {
	root := t.TempDir()
	program := filepath.Join(root, "aigw")
	for path, content := range map[string]string{program: "current", upgrade.RollbackPath(program): "previous"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, retained := range []bool{true, false} {
		out := new(bytes.Buffer)
		command := NewInspectionCommand(invocation.Context{Executable: program, Version: "1.2.3", Out: out})
		command.SetArgs(nil)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		want := "No retained predecessor"
		if retained {
			resolved, err := filepath.EvalSymlinks(upgrade.RollbackPath(program))
			if err != nil {
				t.Fatal(err)
			}
			want = resolved
		}
		if !strings.Contains(out.String(), "1.2.3") || !strings.Contains(out.String(), want) {
			t.Fatalf("human installation output = %s", out)
		}
		if retained {
			if err := os.Remove(upgrade.RollbackPath(program)); err != nil {
				t.Fatal(err)
			}
		}
	}
	closed, err := os.CreateTemp(root, "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"--json"}} {
		command := NewInspectionCommand(invocation.Context{Executable: program, Out: closed})
		command.SetArgs(args)
		if err := command.Execute(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("output error = %v", err)
		}
	}
	command := NewInspectionCommand(invocation.Context{Executable: root, Out: new(bytes.Buffer)})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "current portable program") {
		t.Fatalf("invalid installation observation = %v", err)
	}
}

func TestInstallCommandReportsUnavailableTargetAndCopyFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := NewInstallCommand(invocation.Context{Executable: source})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is unavailable") {
		t.Fatalf("missing target error = %v", err)
	}
	command = NewInstallCommand(invocation.Context{Executable: filepath.Join(root, "missing"), InstallTarget: filepath.Join(root, "bin", "aigw")})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "read portable") {
		t.Fatalf("copy failure = %v", err)
	}
}

func TestUninstallCommandDefaultsToRunningExecutableAndReportsFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "aigw")
	if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	command := NewUninstallCommand(invocation.Context{Executable: target, Out: out})
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target remains: %v", err)
	}
	if !strings.Contains(out.String(), "credential-store secrets were preserved") {
		t.Fatalf("output = %q", out.String())
	}
	command = NewUninstallCommand(invocation.Context{Executable: ""})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is empty") {
		t.Fatalf("empty target error = %v", err)
	}
}

func TestUninstallCommandHandlesConfigurationAndWithdrawalFailures(t *testing.T) {
	t.Run("configuration load", func(t *testing.T) {
		root := t.TempDir()
		command := NewUninstallCommand(invocation.Context{Executable: filepath.Join(root, "aigw"), Config: configuration.NewStore(root)})
		command.SetArgs(nil)
		if err := command.Execute(); err == nil {
			t.Fatal("uninstall succeeded despite an unreadable configuration path")
		}
	})

	t.Run("configuration inspection", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "aigw")
		if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
			t.Fatal(err)
		}
		command := NewUninstallCommand(invocation.Context{
			Executable: target,
			Config:     configuration.NewStore(filepath.Join(root, "invalid") + "\x00configuration.toml"),
		})
		command.SetArgs(nil)
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "inspect AIGW configuration") {
			t.Fatalf("configuration inspection error = %v", err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("program was removed after failed configuration inspection: %v", err)
		}
	})

	t.Run("client withdrawal", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "aigw")
		if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
			t.Fatal(err)
		}
		store := configuration.NewStore(filepath.Join(root, "configuration.toml"))
		cfg := configuration.NewConfig()
		cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
		cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
		cfg.Routes[configuration.ClientClaude] = "claude"
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		blockedSettings := filepath.Join(root, "blocked-settings")
		if err := os.Mkdir(blockedSettings, 0o700); err != nil {
			t.Fatal(err)
		}
		command := NewUninstallCommand(invocation.Context{
			Executable:         target,
			Config:             store,
			ClaudeSettingsPath: blockedSettings,
		})
		command.SetArgs(nil)
		if err := command.Execute(); err == nil {
			t.Fatal("uninstall succeeded despite failed client withdrawal")
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("program was removed after failed client withdrawal: %v", err)
		}
	})
}

func TestInstallCopiesCurrentExecutableAndPreservesOnePredecessor(t *testing.T) {
	for _, previous := range []string{"", "previous"} {
		t.Run("predecessor="+previous, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source-aigw")
			target := filepath.Join(root, "bin", "aigw")
			backup := filepath.Join(root, "bin", ".aigw.previous")
			if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			wantFiles := map[string]string{target: "current"}
			if previous != "" {
				if err := os.WriteFile(target, []byte(previous), 0o755); err != nil {
					t.Fatal(err)
				}
				wantFiles[backup] = previous
			}
			for attempt := range 2 {
				if err := Install(source, target); err != nil {
					t.Fatal(err)
				}
				for path, want := range wantFiles {
					got, err := os.ReadFile(path)
					if err != nil || string(got) != want {
						t.Fatalf("install %d: %s=%q,%v want %q", attempt+1, path, got, err, want)
					}
				}
				if previous == "" {
					if _, err := os.Stat(backup); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("fresh install created a predecessor: %v", err)
					}
				}
			}
		})
	}
}

func TestInstallKeepsIdenticalExecutableInPlace(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-aigw")
	target := filepath.Join(root, "aigw")
	for _, path := range []string{source, target} {
		if err := os.WriteFile(path, []byte("current"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(target)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("identical executable was replaced: %v", err)
	}
}

func TestUninstallRemovesOnlyOwnedProgramFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "aigw")
	backup := filepath.Join(root, ".aigw.previous")
	foreign := filepath.Join(root, "foreign")
	for path := range map[string]bool{target: true, backup: true, foreign: true} {
		if err := os.WriteFile(path, []byte(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := Uninstall(target); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target, backup} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned file retained: %s", path)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file removed: %v", err)
	}
}

func TestInstallRejectsInvalidSourceAndSamePath(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	if err := Install(missing, filepath.Join(root, "target")); err == nil {
		t.Fatal("missing source accepted")
	}
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, source); err == nil || !strings.Contains(err.Error(), "same path") {
		t.Fatalf("same path=%v", err)
	}
}

func TestInstallAcceptsPortableFileAndRejectsBlockedDestinations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "bin", "aigw")
	if err := Install(source, target); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "binary" {
		t.Fatalf("portable target = %q, %v", got, err)
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, filepath.Join(blocked, "aigw")); err == nil {
		t.Fatalf("blocked parent = %v", err)
	}
	if err := Install(source, root); err == nil || !strings.Contains(err.Error(), "read installed") {
		t.Fatalf("directory target = %v", err)
	}
}

func TestInstallRejectsDirectorySource(t *testing.T) {
	root := t.TempDir()
	if err := Install(root, filepath.Join(root, "target")); err == nil || !strings.Contains(err.Error(), "read portable") {
		t.Fatalf("directory source = %v", err)
	}
}

func TestInstallAndUninstallReportOwnedFileFailures(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(upgrade.RollbackPath(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err == nil || !strings.Contains(err.Error(), "save previous") {
		t.Fatalf("blocked backup = %v", err)
	}
	if err := os.RemoveAll(upgrade.RollbackPath(target)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err == nil {
		t.Fatal("target file unexpectedly replaced by directory")
	}
	uninstallTarget := filepath.Join(root, "owned-directory")
	if err := os.Mkdir(uninstallTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uninstallTarget, "child"), []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(uninstallTarget); err == nil || !strings.Contains(err.Error(), "remove portable") {
		t.Fatalf("non-empty owned path = %v", err)
	}
}

func TestInstallReportsAtomicTargetReplacementFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := writeFileAtomic
	writeFileAtomic = func(path string, data []byte, mode os.FileMode) error {
		if path == filepath.Join(root, "aigw") {
			return errors.New("replace failed")
		}
		return original(path, data, mode)
	}
	t.Cleanup(func() { writeFileAtomic = original })
	if err := Install(source, filepath.Join(root, "aigw")); err == nil || !strings.Contains(err.Error(), "replace failed") {
		t.Fatalf("atomic replacement error = %v", err)
	}
}

func TestBackupPathUsesWindowsExecutableSuffix(t *testing.T) {
	if got := upgrade.RollbackPath(filepath.Join("root", "aigw.exe")); filepath.Base(got) != ".aigw.previous.exe" {
		t.Fatalf("backup path = %q", got)
	}
}

func TestPortableInstallAndUninstallPreserveHomebrewOwnership(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "Cellar", "aigw", "0.1.0")
	target := filepath.Join(version, "bin", "aigw")
	source := filepath.Join(root, "download")
	for name, data := range map[string]string{target: "managed", source: "download", filepath.Join(version, "INSTALL_RECEIPT.json"): `{"homebrew_version":"7.0.2","source":{"tap":"owner/tap"}}`} {
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, operation := range map[string]func() error{
		"install":   func() error { return Install(source, target) },
		"uninstall": func() error { return Uninstall(target) },
		"command": func() error {
			command := NewUninstallCommand(invocation.Context{Executable: target, Config: configuration.NewStore("invalid\x00config")})
			command.SetArgs(nil)
			return command.Execute()
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := operation()
			if err == nil || !strings.Contains(err.Error(), "Homebrew") {
				t.Fatalf("ownership error = %v", err)
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "managed" {
				t.Fatalf("managed program changed: %q, %v", data, err)
			}
		})
	}
}
