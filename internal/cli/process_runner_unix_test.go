//go:build !windows

package cli

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
)

// These tests assume a POSIX process model: /usr/bin/true and /usr/bin/false
// as fixed-exit-code fixtures, /bin/echo for stdout capture, and the
// exec/syscall-based replaceProcess in replace_unix.go. They are compiled
// only for non-Windows targets; process_runner_windows_test.go exercises the
// same ProcessRunner behavior with cmd.exe-based fixtures and the
// exec.Command-based replaceProcess in replace_windows.go.

func TestProcessRunnerRunExecutesCommandSuccessfully(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/true",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestProcessRunnerRunReportsChildProcessFailure(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/false",
	})
	if err == nil || !strings.Contains(err.Error(), "run /usr/bin/false") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestProcessRunnerRunReplacesProcessErrorSurfacesLookupFailure(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "aigw-definitely-not-a-real-binary",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "Failed to resolve") {
		t.Fatalf("Run() with Replace error = %v", err)
	}
}

func TestProcessRunnerRunCaptureRejectsReplace(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/true",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot replace the current process") {
		t.Fatalf("RunCapture() with Replace error = %v", err)
	}
}

func TestProcessRunnerRunCaptureSurfacesNonZeroExit(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/false",
	})
	var exitErr *exec.ExitError
	if err == nil || !strings.Contains(err.Error(), "run /usr/bin/false") || !errors.As(err, &exitErr) {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestProcessRunnerRunCaptureReturnsStdout(t *testing.T) {
	output, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "/bin/echo",
		Args:       []string{"AIGW_OK"},
	})
	if err != nil {
		t.Fatalf("RunCapture() error = %v", err)
	}
	if strings.TrimSpace(string(output)) != "AIGW_OK" {
		t.Fatalf("RunCapture() output = %q", output)
	}
}
