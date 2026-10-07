//go:build windows

package performance

import (
	"aigw-cli/internal/process"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsExecutionPreflightOwnsImmediateDescendants(t *testing.T) {
	if os.Getenv("AIGW_TEST_NATIVE_EXECUTION") == "1" {
		if os.Args[len(os.Args)-1] == "reader" {
			command := exec.Command(os.Args[0], "-test.run=^TestWindowsExecutionPreflightOwnsImmediateDescendants$", "worker")
			command.Env = os.Environ()
			if err := command.Run(); err != nil {
				os.Exit(1)
			}
		}
		os.Exit(0)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := Identify(program)
	if err != nil {
		t.Fatal(err)
	}
	shell, err := Identify(os.Getenv("ComSpec"))
	if err != nil {
		t.Fatal(err)
	}
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	script := filepath.Join(directory, "credential.cmd")
	if err := os.WriteFile(script, []byte("@echo off\r\n\""+program+"\" -test.run=^TestWindowsExecutionPreflightOwnsImmediateDescendants$ reader\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := Command{Tool: tool, Output: filepath.Join(directory, "observed.json"), Directory: directory,
		Environment: append(os.Environ(), "AIGW_TEST_NATIVE_EXECUTION=1"),
		Measurement: Measurement{Case: "credential", Backend: "keyring", Executable: &shell, Reader: &reader}}
	workload := Workload{Command: []string{shell.Path, "/d", "/c", script}}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	rows, controllers, err := ObserveCommand(ctx, input, workload)
	if err != nil {
		t.Fatal(err)
	}
	roles := make(map[string]int)
	for _, row := range rows {
		roles[row.Role]++
	}
	if len(controllers) != 1 || roles["workload"] != 1 || roles["reader"] != 1 || roles["credential-worker"] != 1 {
		t.Fatalf("native short-lived process chain was not observed: %#v", roles)
	}
	if _, err := os.Stat(input.Output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("untimed preflight produced timed samples: %v", err)
	}
	for _, stream := range []string{"stdout", "stderr"} {
		if _, err := os.Stat(strings.TrimSuffix(input.Output, ".json") + ".execution." + stream); err != nil {
			t.Fatal(err)
		}
	}
	input.Output, input.Measurement.Executable = filepath.Join(directory, "refused.json"), &reader
	if _, _, err := ObserveCommand(t.Context(), input, workload); err == nil || !strings.Contains(err.Error(), "selected workload") {
		t.Fatalf("native preflight admitted a different process image: %v", err)
	}
}

func TestWindowsExecutionPreflightReclaimsInterruptedProcesses(t *testing.T) {
	if os.Getenv("AIGW_TEST_NATIVE_EXECUTION_WAIT") == "1" {
		time.Sleep(time.Minute)
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := Identify(program)
	if err != nil {
		t.Fatal(err)
	}
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	input := Command{Tool: tool, Output: filepath.Join(t.TempDir(), "interrupted.json"),
		Environment: append(os.Environ(), "AIGW_TEST_NATIVE_EXECUTION_WAIT=1"), Measurement: Measurement{Case: "status", Executable: &image}}
	started := time.Now()
	rows, controllers, err := ObserveCommand(ctx, input, Workload{Command: []string{program, "-test.run=^TestWindowsExecutionPreflightReclaimsInterruptedProcesses$"}})
	if err == nil || ctx.Err() == nil {
		t.Fatalf("interrupted native preflight qualified: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("interrupted native preflight exceeded its total cleanup bound: %s", elapsed)
	}
	if len(controllers) != 1 || len(rows) == 0 {
		t.Fatalf("interruption did not exercise the owned native process chain: %d controllers, %d descendants", len(controllers), len(rows))
	}
	assertWindowsExecutionsExited(t, append(controllers, rows...))
}

func TestWindowsExecutionPreflightReclaimsBeforeFirstEvent(t *testing.T) {
	if os.Getenv("AIGW_TEST_NATIVE_EXECUTION_WAIT") == "1" {
		time.Sleep(time.Minute)
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	observer := windowsObserver{handles: make(map[uint32]windows.Handle), active: make(map[uint32]windows.Handle), continueEvent: continueWindowsDebugEvent}
	var child *os.Process
	started := time.Now()
	_, _, err = (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: program, Args: []string{"-test.run=^TestWindowsExecutionPreflightReclaimsBeforeFirstEvent$"},
		Env: append(os.Environ(), "AIGW_TEST_NATIVE_EXECUTION_WAIT=1"), DebugProcess: true,
		OnStart: func(started *os.Process) error {
			child = started
			cancel()
			return observer.collect(ctx, uint32(started.Pid)) // #nosec G115 -- exec.Start returns the native DWORD PID.
		},
	})
	if !errors.Is(err, context.Canceled) || child == nil || len(observer.controllers) != 0 || len(observer.rows) != 0 {
		t.Fatalf("pre-first-event refusal was not exercised: child=%v error=%v", child, err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("unobserved native debuggee exceeded its total cleanup bound: %s", elapsed)
	}
	if err := child.Kill(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("unobserved native debuggee survived runner return: %v", err)
	}
}

func TestWindowsExecutionPreflightUsesOneCleanupBudget(t *testing.T) {
	if os.Getenv("AIGW_TEST_NATIVE_EXECUTION_WAIT") == "1" {
		time.Sleep(time.Minute)
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	observer := windowsObserver{handles: make(map[uint32]windows.Handle), active: make(map[uint32]windows.Handle)}
	for range 4 {
		command := exec.Command(program, "-test.run=^TestWindowsExecutionPreflightUsesOneCleanupBudget$")
		command.Env = append(os.Environ(), "AIGW_TEST_NATIVE_EXECUTION_WAIT=1")
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = command.Process.Kill()
			_ = command.Wait()
		})
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(command.Process.Pid)) // #nosec G115 -- exec.Start returns the native DWORD PID.
		if err != nil {
			t.Fatal(err)
		}
		observer.handles[uint32(command.Process.Pid)] = handle // #nosec G115 -- exec.Start returns the native DWORD PID.
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	started := time.Now()
	if err := observer.collect(ctx, 0); !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "did not exit") {
		t.Fatalf("unsettled native handles did not reject qualification: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("cleanup multiplied the total budget by retained process count: %s", elapsed)
	}
	for _, handle := range observer.handles {
		if _, err := windows.WaitForSingleObject(handle, 0); !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
			t.Fatalf("cleanup did not close an exact retained native handle: %v", err)
		}
	}
}

func assertWindowsExecutionsExited(t *testing.T, rows []Execution) {
	t.Helper()
	for _, row := range rows {
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, row.PID)
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		created, creationErr := processCreation(handle)
		event, waitErr := windows.WaitForSingleObject(handle, 0)
		closeErr := windows.CloseHandle(handle)
		if err := errors.Join(creationErr, waitErr, closeErr); err != nil {
			t.Fatal(err)
		}
		if created == row.Created && event != windows.WAIT_OBJECT_0 {
			t.Fatalf("exact owned native %s process %d survived cleanup", row.Role, row.PID)
		}
	}
}

func TestWindowsExecutionPreflightRetainsUncontinuedExitOwnership(t *testing.T) {
	if os.Getenv("AIGW_TEST_NATIVE_EXIT_EVENT") == "1" {
		os.Exit(0)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := Identify(program)
	if err != nil {
		t.Fatal(err)
	}
	observer := windowsObserver{selected: Measurement{Executable: &image}, handles: make(map[uint32]windows.Handle), active: make(map[uint32]windows.Handle)}
	exitSeen := false
	observer.continueEvent = func(event debugEvent, status uintptr) error {
		if event.Code == 5 {
			exitSeen = true
			event.TID++ // The native API rejects continuation for a thread that does not own this event.
		}
		return continueWindowsDebugEvent(event, status)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := time.Now()
	_, _, err = (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: program, Args: []string{"-test.run=^TestWindowsExecutionPreflightRetainsUncontinuedExitOwnership$"},
		Env: append(os.Environ(), "AIGW_TEST_NATIVE_EXIT_EVENT=1"), DebugProcess: true,
		OnStart: func(child *os.Process) error { return observer.collect(ctx, uint32(child.Pid)) }, // #nosec G115 -- exec.Start returns the native DWORD PID.
	})
	if !exitSeen || err == nil || !strings.Contains(err.Error(), "continue owned native debug event") {
		t.Fatalf("native failed-exit continuation was not exercised: exit_seen=%t error=%v", exitSeen, err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("failed-exit continuation exceeded the total cleanup bound: %s", elapsed)
	}
	assertWindowsExecutionsExited(t, observer.controllers)
}
