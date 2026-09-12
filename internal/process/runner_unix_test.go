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
