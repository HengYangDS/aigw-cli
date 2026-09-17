//go:build !windows

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunCaptureBoundsPipeDrainAfterChildExit(t *testing.T) {
	requireShellFixture(t)
	for _, output := range []string{"capture", "file"} {
		t.Run(output, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), capturedProcessWaitDelay+3*time.Second)
			defer cancel()
			plan := Plan{Executable: "/bin/sh", Args: []string{"-c", "sleep 1.5; (sleep 5) >&2 & printf AIGW_OK"}, Env: []string{}}
			destination := filepath.Join(t.TempDir(), "asset")
			var err error
			if output == "capture" {
				_, err = (Runner{}).RunCapture(ctx, plan)
			} else {
				err = (Runner{}).RunToFile(ctx, destination, plan)
			}
			// ErrWaitDelay proves the native pipe-drain timer expired. Elapsed
			// invocation time also includes startup and host scheduling, neither
			// of which belongs to that timer's contract.
			if !errors.Is(err, exec.ErrWaitDelay) {
				t.Fatalf("%s error = %v, want exec.ErrWaitDelay", output, err)
			}
			if output == "file" {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed stream retained output: %v", err)
				}
			}
		})
	}
}

func TestRunCaptureReportsDeadlineAfterPipeDrain(t *testing.T) {
	requireShellFixture(t)
	for _, startup := range []string{"0", "1.5"} {
		t.Run("startup-"+startup, func(t *testing.T) {
			testDeadlineAfterPipeDrain(t, startup)
		})
	}
}

func testDeadlineAfterPipeDrain(t *testing.T, startup string) {
	t.Helper()
	fixtureDir := t.TempDir()
	marker := filepath.Join(fixtureDir, "descendant.pid")
	t.Cleanup(func() {
		data, readErr := os.ReadFile(marker)
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

	ctx := newControllableDeadlineContext()
	t.Cleanup(ctx.expire)
	result := make(chan error, 1)
	go func() {
		_, runErr := (Runner{}).RunCapture(ctx, Plan{
			Executable: "/bin/sh",
			Args:       []string{"-c", "sleep \"$2\"; (sleep 30) & printf '%s\\n' \"$!\" > \"$1\"; sleep 30", "sh", marker, startup},
			Env:        []string{},
		})
		result <- runErr
	}()
	if err := awaitFixtureFile(marker, pipeDrainFixtureWait); err != nil {
		ctx.expire()
		select {
		case <-result:
		case <-time.After(capturedProcessWaitDelay + pipeDrainFixtureWait):
		}
		t.Fatalf("background descendant did not inherit output before deadline: %v", err)
	}
	ctx.expire()
	var runErr error
	select {
	case runErr = <-result:
	case <-time.After(capturedProcessWaitDelay + pipeDrainFixtureWait):
		t.Fatal("RunCapture did not return after the bounded pipe-drain delay")
	}
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("RunCapture error = %v, want deadline and pipe-drain diagnostic", runErr)
	}
}
