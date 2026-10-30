package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
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
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: source, InstallTarget: target, Out: out})
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

func TestInstallCommandReportsUnavailableTargetAndCopyFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: source})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is unavailable") {
		t.Fatalf("missing target error = %v", err)
	}
	command = NewCommand(invocation.Context{Clients: clients.Registry(), Executable: filepath.Join(root, "missing"), InstallTarget: filepath.Join(root, "bin", "aigw")})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "read portable") {
		t.Fatalf("copy failure = %v", err)
	}
}

func TestHomebrewSourceCannotCreatePortableInstallation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Caskroom", "aigw", "0.1.0", "aigw")
	receipt := filepath.Join(root, "Caskroom", "aigw", ".metadata", "INSTALL_RECEIPT.json")
	target := filepath.Join(root, "portable", "aigw")
	for name, data := range map[string]string{source: "managed", receipt: `{"homebrew_version":"7.0.4","source":{"tap":"owner/tap"}}`, target: "prior"} {
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(data), 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: source, InstallTarget: target})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "Homebrew") {
		t.Fatalf("managed source escaped ownership: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "prior" {
		t.Fatalf("portable target changed: %q %v", data, err)
	}
	if _, err := os.Stat(upgrade.RollbackPath(target)); !os.IsNotExist(err) {
		t.Fatalf("backup created: %v", err)
	}
}
