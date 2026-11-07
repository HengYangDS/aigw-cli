//go:build windows

package process

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
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

func TestRunStreamReportsTheCancellationThatEndsItsStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	native := nativeWindowsProcessAPI
	t.Cleanup(func() { nativeWindowsProcessAPI = native })
	nativeWindowsProcessAPI.resumeProcess = func(pid uint32) error {
		child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			t.Errorf("observe the suspended child: %v", err)
			return err
		}
		defer func() { _ = windows.CloseHandle(child) }()
		// os/exec terminates the suspended child once its context ends.
		cancel()
		if event, err := windows.WaitForSingleObject(child, 10_000); err != nil || event != windows.WAIT_OBJECT_0 {
			t.Errorf("the cancelled child still runs: event=%d error=%v", event, err)
		}
		return native.resumeProcess(pid)
	}
	err := (Runner{}).RunStream(ctx, Plan{Executable: "cmd.exe", Args: []string{"/c", "exit", "0"}}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "start cmd.exe") {
		t.Fatalf("RunStream() error = %v; want the failed start with the cancellation that ended it", err)
	}
}

func captureProcessExit(t *testing.T, child *os.Process) func() {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(child.Pid)) // #nosec G115 -- exec.Start supplies the native DWORD PID, before its handle is released.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.CloseHandle(handle); err != nil {
			t.Fatal(err)
		}
	})
	return func() {
		t.Helper()
		event, err := windows.WaitForSingleObject(handle, 0)
		if err != nil || event != windows.WAIT_OBJECT_0 {
			t.Fatalf("exact observed child survived runner return: event=%d error=%v", event, err)
		}
	}
}
