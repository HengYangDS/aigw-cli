package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
)

func requireShellFixture(t *testing.T) error {
	t.Helper()
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		return fmt.Errorf("/bin/sh unavailable: %w", err)
	}
	return nil
}

// deadlineSignalContext lets this test begin the command only after its
// background descendant has inherited the output pipes, then deterministically
// deliver the DeadlineExceeded signal that RunCapture reports to callers.
// A fixed short timeout races process start under parallel CI load.
type deadlineSignalContext struct {
	done chan struct{}
	once sync.Once
}

func newDeadlineSignalContext() *deadlineSignalContext {
	return &deadlineSignalContext{done: make(chan struct{})}
}

func (c *deadlineSignalContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (c *deadlineSignalContext) Done() <-chan struct{} { return c.done }

func (c *deadlineSignalContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *deadlineSignalContext) Value(any) any { return nil }

func (c *deadlineSignalContext) expire() {
	c.once.Do(func() { close(c.done) })
}

func waitForFixtureFile(path string, limit time.Duration) error {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-timer.C:
			return fmt.Errorf("timed out after %s", limit)
		case <-ticker.C:
		}
	}
}

func TestRunCaptureBoundsPipeDrainAfterChildExit(t *testing.T) {
	if err := requireShellFixture(t); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := time.Now()
	_, err := (ProcessRunner{}).RunCapture(ctx, adapters.ProcessPlan{
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
	if err := requireShellFixture(t); err != nil {
		t.Skip(err)
	}
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
		_, runErr := (ProcessRunner{}).RunCapture(ctx, adapters.ProcessPlan{
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
