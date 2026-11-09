package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"
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
	if _, err := os.Stat(program.RollbackPath(target)); !os.IsNotExist(err) {
		t.Fatalf("backup created: %v", err)
	}
}

// Installing the running program onto its own file names another file, which
// is the step that lets the command succeed.
func TestInstallCommandNamesAnotherFileForItsOwnPath(t *testing.T) {
	source := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: source, InstallTarget: source, Out: new(bytes.Buffer)})
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs(nil)
	if problem := terminal.ProblemFor(command.Execute()); !strings.Contains(problem.Fix, "--path naming another file") {
		t.Fatalf("problem = %+v", problem)
	}
}

// A target that the command cannot use is named with the step that resolves
// it, where a local check would inspect neither it nor its folder, and the
// failure writes nothing.
func TestInstallCommandNamesTheTargetThatItCannotUse(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	blocked := filepath.Join(root, "blocked")
	for _, path := range []string{source, blocked} {
		if err := os.WriteFile(path, []byte("file"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(blocked, "aigw")
	for _, args := range [][]string{nil, {"--dry-run"}} {
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: source, InstallTarget: target, Out: new(bytes.Buffer)})
		command.SilenceErrors, command.SilenceUsage = true, true
		command.SetArgs(args)
		err := command.Execute()
		if err == nil {
			t.Fatalf("%v: installing under a file passed", args)
		}
		problem := terminal.ProblemFor(err)
		if !strings.Contains(problem.Title, target) || !strings.Contains(problem.Fix, blocked) || strings.Contains(problem.Title, "unexpected error") ||
			!strings.HasSuffix(problem.Fix, "then run the same command again.") || strings.Contains(problem.Fix, "aigw check") {
			t.Fatalf("%v: problem = %+v", args, problem)
		}
	}
	if data, err := os.ReadFile(blocked); err != nil || string(data) != "file" {
		t.Fatalf("the blocking file changed: %q, %v", data, err)
	}
}
