package installation

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/upgrade"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: program, Version: "1.2.3", Out: out})
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
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: program, Out: closed})
		command.SetArgs(args)
		if err := command.Execute(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("output error = %v", err)
		}
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: root, Out: new(bytes.Buffer)})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "current portable program") {
		t.Fatalf("invalid installation observation = %v", err)
	}
}
