//go:build windows

package cli

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
)

// These are the Windows equivalents of process_runner_unix_test.go: cmd.exe
// stands in for /usr/bin/true, /usr/bin/false and /bin/echo, and the
// exec.Command-based replaceProcess in replace_windows.go reports "Failed to
// run" (not the POSIX exec/syscall "Failed to resolve") on a lookup failure.

func TestProcessRunnerRunExecutesCommandSuccessfully(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "0"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestProcessRunnerRunReportsChildProcessFailure(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "1"},
	})
	if err == nil || !strings.Contains(err.Error(), "run cmd.exe") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestProcessRunnerRunReplacesProcessErrorSurfacesLookupFailure(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "aigw-definitely-not-a-real-binary",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "Failed to run") {
		t.Fatalf("Run() with Replace error = %v", err)
	}
}

func TestProcessRunnerRunCaptureRejectsReplace(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "0"},
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot replace the current process") {
		t.Fatalf("RunCapture() with Replace error = %v", err)
	}
}

func TestProcessRunnerRunCaptureSurfacesNonZeroExit(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "1"},
	})
	var exitErr *exec.ExitError
	if err == nil || !strings.Contains(err.Error(), "run cmd.exe") || !errors.As(err, &exitErr) {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestProcessRunnerRunCaptureReturnsStdout(t *testing.T) {
	output, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "echo", "AIGW_OK"},
	})
	if err != nil {
		t.Fatalf("RunCapture() error = %v", err)
	}
	if strings.TrimSpace(string(output)) != "AIGW_OK" {
		t.Fatalf("RunCapture() output = %q", output)
	}
}
