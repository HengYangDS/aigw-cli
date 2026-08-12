//go:build !windows

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestRunCaptureBoundsPipeDrainAfterChildExit(t *testing.T) {
	requireShellFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), capturedProcessWaitDelay+3*time.Second)
	defer cancel()
	started := time.Now()
	_, err := (Runner{}).RunCapture(ctx, Plan{
		Executable: "/bin/sh",
		Args:       []string{"-c", "(sleep 5) & printf AIGW_OK"},
		Env:        []string{},
	})
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("RunCapture error = %v, want exec.ErrWaitDelay", err)
	}
	if elapsed := time.Since(started); elapsed > capturedProcessWaitDelay+time.Second {
		t.Fatalf("RunCapture took %s, expected bounded pipe drain", elapsed)
	}
}

func TestRunCaptureReportsPipeDrainAfterContextDeadline(t *testing.T) {
	requireShellFixture(t)
	fixtureDir := t.TempDir()
	marker, err := os.CreateTemp(fixtureDir, "pipe-drain-child-")
	if err != nil {
		t.Fatalf("CreateTemp marker: %v", err)
	}
	if err := marker.Close(); err != nil {
		t.Fatalf("close marker: %v", err)
	}
	if err := os.Remove(marker.Name()); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	ready, err := os.CreateTemp(fixtureDir, "pipe-drain-ready-")
	if err != nil {
		t.Fatalf("CreateTemp ready marker: %v", err)
	}
	if err := ready.Close(); err != nil {
		t.Fatalf("close ready marker: %v", err)
	}
	if err := os.Remove(ready.Name()); err != nil {
		t.Fatalf("remove ready marker: %v", err)
	}
	t.Cleanup(func() {
		data, readErr := os.ReadFile(marker.Name())
		if readErr != nil {
			return
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr != nil || pid <= 0 {
			return
		}
		if process, findErr := os.FindProcess(pid); findErr == nil {
			_ = process.Kill()
		}
	})

	ctx := newDeadlineSignalContext()
	result := make(chan error, 1)
	go func() {
		_, runErr := (Runner{}).RunCapture(ctx, Plan{
			Executable: "/bin/sh",
			Args:       []string{"-c", "(sleep 30) & printf '%s\\n' \"$!\" > \"$1\"; : > \"$2\"; printf started; while [ ! -f \"$3\" ]; do sleep 0.01; done; sleep 30", "sh", marker.Name(), ready.Name(), fixtureDir + "/expire"},
			Env:        []string{},
		})
		result <- runErr
	}()
	if err := waitForFixtureFile(marker.Name(), time.Second); err != nil {
		ctx.expire()
		select {
		case <-result:
		case <-time.After(capturedProcessWaitDelay + time.Second):
		}
		t.Fatalf("background descendant did not inherit output before deadline: %v", err)
	}
	if err := waitForFixtureFile(ready.Name(), time.Second); err != nil {
		ctx.expire()
		select {
		case <-result:
		case <-time.After(capturedProcessWaitDelay + time.Second):
		}
		t.Fatalf("shell fixture did not reach the deadline barrier: %v", err)
	}
	if err := os.WriteFile(fixtureDir+"/expire", []byte("expire\n"), 0o600); err != nil {
		t.Fatalf("release deadline barrier: %v", err)
	}
	ctx.expire()
	var runErr error
	select {
	case runErr = <-result:
	case <-time.After(capturedProcessWaitDelay + time.Second):
		t.Fatal("RunCapture did not return after the bounded pipe-drain delay")
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "output pipes did not close within") {
		t.Fatalf("RunCapture error = %v, want pipe-drain diagnostic", runErr)
	}
}
