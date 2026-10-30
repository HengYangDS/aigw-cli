//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestStartCapturedWindowsProcessOwnsEveryFailureBoundary(t *testing.T) {
	failure := errors.New("failure")
	tests := []struct {
		name        string
		failAt      string
		wantClosed  int
		wantStopped bool
	}{
		{name: "create job", failAt: "create"},
		{name: "configure job", failAt: "configure", wantClosed: 1},
		{name: "start command", failAt: "start", wantClosed: 1},
		{name: "open process", failAt: "open", wantClosed: 1, wantStopped: true},
		{name: "assign process", failAt: "assign", wantClosed: 2, wantStopped: true},
		{name: "close process", failAt: "close", wantClosed: 2, wantStopped: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			closed, stopped := 0, false
			api := windowsProcessAPI{
				createJob: func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) {
					if test.failAt == "create" {
						return 0, failure
					}
					return 10, nil
				},
				configureJob: func(windows.Handle, uint32, uintptr, uint32) (int, error) {
					if test.failAt == "configure" {
						return 0, failure
					}
					return 1, nil
				},
				startCommand: func(command *exec.Cmd) error {
					if test.failAt == "start" {
						return failure
					}
					command.Process = &os.Process{Pid: 42}
					return nil
				},
				openProcess: func(uint32, bool, uint32) (windows.Handle, error) {
					if test.failAt == "open" {
						return 0, failure
					}
					return 20, nil
				},
				assignProcess: func(windows.Handle, windows.Handle) error {
					if test.failAt == "assign" {
						return failure
					}
					return nil
				},
				closeHandle: func(handle windows.Handle) error {
					closed++
					if test.failAt == "close" && handle == 20 {
						return failure
					}
					return nil
				},
				stopCommand: func(*exec.Cmd) error { stopped = true; return nil },
			}

			cleanup, err := startCapturedWindowsProcess(exec.Command("fixture"), api)
			if err == nil || cleanup != nil || !errors.Is(err, failure) {
				t.Fatalf("cleanup_present=%t error=%v", cleanup != nil, err)
			}
			if closed != test.wantClosed || stopped != test.wantStopped {
				t.Fatalf("closed=%d stopped=%t", closed, stopped)
			}
		})
	}
}

func TestStartCapturedWindowsProcessReturnsIdempotentCleanup(t *testing.T) {
	closed := 0
	api := windowsProcessAPI{
		createJob:    func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) { return 10, nil },
		configureJob: func(windows.Handle, uint32, uintptr, uint32) (int, error) { return 1, nil },
		startCommand: func(command *exec.Cmd) error {
			command.Process = &os.Process{Pid: 42}
			return nil
		},
		openProcess:   func(uint32, bool, uint32) (windows.Handle, error) { return 20, nil },
		assignProcess: func(windows.Handle, windows.Handle) error { return nil },
		closeHandle: func(windows.Handle) error {
			closed++
			return nil
		},
		stopCommand: func(*exec.Cmd) error { return nil },
	}

	cleanup, err := startCapturedWindowsProcess(exec.Command("fixture"), api)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if closed != 2 {
		t.Fatalf("closed=%d", closed)
	}
	closeFailure := errors.New("close job")
	api.closeHandle = func(handle windows.Handle) error {
		closed++
		if handle == 10 {
			return closeFailure
		}
		return nil
	}
	closed = 0
	cleanup, err = startCapturedWindowsProcess(exec.Command("fixture"), api)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := cleanup(); !errors.Is(err, closeFailure) {
			t.Fatalf("cleanup error = %v, want %v", err, closeFailure)
		}
	}
	if closed != 2 {
		t.Fatalf("failed cleanup was repeated: closed=%d", closed)
	}
}

func TestStartCapturedWindowsProcessRetainsCleanupFailures(t *testing.T) {
	configureFailure := errors.New("configure job")
	closeFailure := errors.New("close job")
	api := windowsProcessAPI{
		createJob: func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) { return 10, nil },
		configureJob: func(windows.Handle, uint32, uintptr, uint32) (int, error) {
			return 0, configureFailure
		},
		closeHandle: func(windows.Handle) error { return closeFailure },
	}
	cleanup, err := startCapturedWindowsProcess(exec.Command("fixture"), api)
	if cleanup != nil || !errors.Is(err, configureFailure) || !errors.Is(err, closeFailure) {
		t.Fatalf("cleanup_present=%t error=%v", cleanup != nil, err)
	}
}

func TestStartCapturedWindowsProcessReclaimsRejectedDebuggee(t *testing.T) {
	if os.Getenv("AIGW_TEST_DEBUG_START_REFUSAL") == "1" {
		time.Sleep(time.Minute)
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, boundary := range []string{"open", "assign"} {
		t.Run(boundary, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			failure := errors.New("selected native startup refusal")
			api := nativeWindowsProcessAPI
			if boundary == "open" {
				api.openProcess = func(uint32, bool, uint32) (windows.Handle, error) { return 0, failure }
			} else {
				api.assignProcess = func(windows.Handle, windows.Handle) error { return failure }
			}
			command := exec.Command(program, "-test.run=^TestStartCapturedWindowsProcessReclaimsRejectedDebuggee$")
			command.Env = append(os.Environ(), "AIGW_TEST_DEBUG_START_REFUSAL=1")
			command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DEBUG_PROCESS}
			started := time.Now()
			cleanup, err := startCapturedWindowsProcess(command, api)
			if cleanup != nil || !errors.Is(err, failure) {
				t.Fatalf("native startup refusal was not preserved: cleanup=%t error=%v", cleanup != nil, err)
			}
			if elapsed := time.Since(started); elapsed > 3*time.Second {
				t.Fatalf("rejected native debuggee exceeded the total cleanup bound: %s", elapsed)
			}
			if command.ProcessState == nil || !command.ProcessState.Exited() {
				t.Fatal("rejected native debuggee was not reaped")
			}
		})
	}
}

func TestStartCapturedWindowsProcessRetainsStopFailure(t *testing.T) {
	refused, stopFailure := errors.New("assign refused"), errors.New("stop failed")
	api := windowsProcessAPI{
		createJob:     func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) { return 10, nil },
		configureJob:  func(windows.Handle, uint32, uintptr, uint32) (int, error) { return 1, nil },
		startCommand:  func(command *exec.Cmd) error { command.Process = &os.Process{Pid: 42}; return nil },
		openProcess:   func(uint32, bool, uint32) (windows.Handle, error) { return 20, nil },
		assignProcess: func(windows.Handle, windows.Handle) error { return refused },
		closeHandle:   func(windows.Handle) error { return nil },
		stopCommand:   func(*exec.Cmd) error { return stopFailure },
	}
	cleanup, err := startCapturedWindowsProcess(exec.Command("fixture"), api)
	if cleanup != nil || !errors.Is(err, refused) || !errors.Is(err, stopFailure) {
		t.Fatalf("startup lost exact owned cleanup failure: cleanup=%t error=%v", cleanup != nil, err)
	}
}
