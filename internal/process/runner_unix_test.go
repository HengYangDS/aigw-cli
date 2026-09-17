//go:build !windows

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// requireShellFixture fails loudly (rather than skipping) when this POSIX
// build's /bin/sh fixture is unavailable: a real Unix CI runner is expected
// to always provide one, so its absence is itself a reportable environment
// defect, not a reason to silently drop coverage.
func requireShellFixture(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Fatalf("/bin/sh unavailable: %v", err)
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

func TestRunCaptureStopsOwnedDescendantsAfterParentExit(t *testing.T) {
	requireShellFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	other := exec.CommandContext(ctx, "/bin/sleep", "30")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })
	output, err := (Runner{}).RunCapture(ctx, Plan{
		Executable: "/bin/sh",
		Args:       []string{"-c", "sleep 30 </dev/null >/dev/null 2>&1 & printf '%s' \"$!\""},
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(output))
	if err != nil || pid <= 0 {
		t.Fatalf("owned descendant identity = %q: %v", output, err)
	}
	t.Cleanup(func() { _ = unix.Kill(pid, unix.SIGKILL) })
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, queryErr := exec.CommandContext(ctx, "ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		var exited *exec.ExitError
		absent := errors.As(queryErr, &exited) && exited.ExitCode() == 1
		if queryErr != nil && !absent {
			t.Fatalf("observe owned descendant: %v", queryErr)
		}
		if absent || strings.HasPrefix(strings.TrimSpace(string(status)), "Z") {
			if err := other.Process.Signal(syscall.Signal(0)); err != nil {
				t.Fatalf("unrelated process was stopped: %v", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("owned descendant remains running after the invocation returned")
}
