//go:build windows

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Windows fixtures exercise captured executables and discovered batch clients.

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

func TestRunnerRunCaptureExecutesDiscoveredBatchClient(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "client with spaces.cmd")
	if err := os.WriteFile(executable, []byte("@echo off\r\necho %~1^|%~2\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: executable,
		Args:       []string{"first value", "second value"},
	})
	if err != nil {
		t.Fatalf("RunCapture() error = %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "first value|second value" {
		t.Fatalf("RunCapture() output = %q", got)
	}
}

func TestStartCapturedProcessReportsMissingExecutable(t *testing.T) {
	cleanup, err := startCapturedProcess(exec.Command("aigw-definitely-not-a-real-binary"))
	if err == nil || cleanup != nil {
		t.Fatalf("cleanup_present=%t error=%v", cleanup != nil, err)
	}
}
