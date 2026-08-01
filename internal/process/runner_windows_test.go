//go:build windows

package process

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// These are the Windows equivalents of process_runner_unix_test.go: cmd.exe
// stands in for /usr/bin/true, /usr/bin/false and /bin/echo, and the
// exec.Command-based replaceProcess in replace_windows.go reports "Failed to
// run" (not the POSIX exec/syscall "Failed to resolve") on a lookup failure.

func TestRunnerRunExecutesCommandSuccessfully(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "0"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunReportsChildProcessFailure(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "1"},
	})
	if err == nil || !strings.Contains(err.Error(), "run cmd.exe") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunReplacesProcessErrorSurfacesLookupFailure(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "aigw-definitely-not-a-real-binary",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "Failed to run") {
		t.Fatalf("Run() with Replace error = %v", err)
	}
}

func TestRunnerRunReplacesProcessSuccessfully(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "0"},
		Replace:    true,
	})
	if err != nil {
		t.Fatalf("Run() with Replace error = %v", err)
	}
}

func TestRunnerRunCaptureRejectsReplace(t *testing.T) {
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "0"},
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot replace the current process") {
		t.Fatalf("RunCapture() with Replace error = %v", err)
	}
}

func TestRunnerRunCaptureSurfacesNonZeroExit(t *testing.T) {
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: "cmd.exe",
		Args:       []string{"/c", "exit", "1"},
	})
	var exitErr *exec.ExitError
	if err == nil || !strings.Contains(err.Error(), "run cmd.exe") || !errors.As(err, &exitErr) {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestRunnerRunCaptureReturnsStdout(t *testing.T) {
	output, err := (Runner{}).RunCapture(context.Background(), Plan{
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
