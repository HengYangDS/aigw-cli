package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := (ProcessRunner{}).RunCapture(ctx, adapters.ProcessPlan{
		Executable: "/bin/sh",
		Args:       []string{"-c", "(sleep 5) & printf started; sleep 5"},
		Env:        []string{},
	})
	if err == nil || !strings.Contains(err.Error(), "输出管道未在") {
		t.Fatalf("RunCapture error = %v, want pipe-drain diagnostic", err)
	}
}
