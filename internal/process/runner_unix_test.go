//go:build !windows

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// These tests assume a POSIX process model: /usr/bin/true and /usr/bin/false
// as fixed-exit-code fixtures, /bin/echo for stdout capture, and the
// exec/syscall-based replaceProcess in replace_unix.go. They are compiled
// only for non-Windows targets; process_runner_windows_test.go exercises the
// same Runner behavior with cmd.exe-based fixtures and the
// exec.Command-based replaceProcess in replace_windows.go.

func TestRunnerRunExecutesCommandSuccessfully(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "/usr/bin/true",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunReportsChildProcessFailure(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "/usr/bin/false",
	})
	if err == nil || !strings.Contains(err.Error(), "run /usr/bin/false") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunReplacesProcessErrorSurfacesLookupFailure(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: "aigw-definitely-not-a-real-binary",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "Failed to resolve") {
		t.Fatalf("Run() with Replace error = %v", err)
	}
}

func TestRunnerRunCaptureRejectsReplace(t *testing.T) {
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: "/usr/bin/true",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot replace the current process") {
		t.Fatalf("RunCapture() with Replace error = %v", err)
	}
}

func TestRunnerRunCaptureSurfacesNonZeroExit(t *testing.T) {
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: "/usr/bin/false",
	})
	var exitErr *exec.ExitError
	if err == nil || !strings.Contains(err.Error(), "run /usr/bin/false") || !errors.As(err, &exitErr) {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestRunnerRunCaptureReturnsStdout(t *testing.T) {
	output, err := (Runner{}).RunCapture(context.Background(), Plan{
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

func TestReplaceProcessSurfacesExecFailure(t *testing.T) {
	original := unixExec
	t.Cleanup(func() { unixExec = original })
	unixExec = func(argv0 string, argv []string, envv []string) error {
		return errors.New("forced exec failure")
	}
	err := replaceProcess(Plan{Executable: "/usr/bin/true", Env: []string{"A=1"}})
	if err == nil || !strings.Contains(err.Error(), "Failed to replace AIGW with") || !strings.Contains(err.Error(), "forced exec failure") {
		t.Fatalf("replaceProcess() error = %v", err)
	}
}

func TestReplaceProcessReturnsNilWhenExecSucceeds(t *testing.T) {
	original := unixExec
	t.Cleanup(func() { unixExec = original })
	var sawArgs []string
	unixExec = func(argv0 string, argv []string, envv []string) error {
		sawArgs = append([]string(nil), argv...)
		return nil
	}
	if err := replaceProcess(Plan{Executable: "/usr/bin/true", Args: []string{"--version"}, Env: []string{"A=1"}}); err != nil {
		t.Fatalf("replaceProcess() error = %v", err)
	}
	if len(sawArgs) < 2 || sawArgs[1] != "--version" {
		t.Fatalf("exec argv = %#v", sawArgs)
	}
}

func TestRunCaptureRejectsOversizedStdout(t *testing.T) {
	requireShellFixture(t)
	// Stream more than capturedProcessOutputLimit bytes through the capture buffer.
	script := fmt.Sprintf(`head -c %d /dev/zero | tr '\0' x`, capturedProcessOutputLimit+2048)
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: "/bin/sh",
		Args:       []string{"-c", script},
		Env:        os.Environ(),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("RunCapture() error = %v, want oversized output", err)
	}
}

func TestRunCaptureReportsDeadlineExceededPipeDrain(t *testing.T) {
	requireShellFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	// Keep an output pipe open past the deadline so WaitDelay collides with ctx timeout.
	_, err := (Runner{}).RunCapture(ctx, Plan{
		Executable: "/bin/sh",
		Args:       []string{"-c", "(sleep 8) & printf started; sleep 8"},
		Env:        []string{},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeded its verification limit") {
		t.Fatalf("RunCapture() error = %v, want verification-limit diagnostic", err)
	}
}
