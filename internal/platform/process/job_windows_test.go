//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"slices"
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
		{name: "resume process", failAt: "resume", wantClosed: 2, wantStopped: true},
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
				resumeProcess: func(uint32) error {
					if test.failAt == "resume" {
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
		resumeProcess: func(uint32) error { return nil },
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

// A real child that would exit at once is held until it joins its Job: a
// pause before the assignment finds it still running, and it then completes.
func TestStartCapturedWindowsProcessContainsAnImmediatelyExitingChild(t *testing.T) {
	interpreter := os.Getenv("ComSpec")
	if interpreter == "" {
		interpreter = "cmd.exe"
	}
	api := nativeWindowsProcessAPI
	assign := api.assignProcess
	exitedBeforeAssign := false
	command := exec.Command(interpreter, "/d", "/c", "exit 0")
	api.assignProcess = func(job, child windows.Handle) error {
		// The startup handle carries only the assignment rights, so the pause
		// waits on its own handle to the started child.
		waiter, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(command.Process.Pid)) // #nosec G115 -- exec.Start supplies the native DWORD PID.
		if err != nil {
			return err
		}
		defer func() { _ = windows.CloseHandle(waiter) }()
		event, err := windows.WaitForSingleObject(waiter, 250)
		if err != nil {
			return err
		}
		exitedBeforeAssign = event == windows.WAIT_OBJECT_0
		return assign(job, child)
	}
	cleanup, err := startCapturedWindowsProcess(command, api)
	if err != nil {
		t.Fatalf("an immediately exiting child failed to start: %v", err)
	}
	waitErr := command.Wait()
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if exitedBeforeAssign {
		t.Fatal("the child ran and exited before it joined its Job")
	}
	if waitErr != nil {
		t.Fatalf("the contained child did not complete: %v", waitErr)
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
		resumeProcess: func(uint32) error { return nil },
		closeHandle:   func(windows.Handle) error { return nil },
		stopCommand:   func(*exec.Cmd) error { return stopFailure },
	}
	cleanup, err := startCapturedWindowsProcess(exec.Command("fixture"), api)
	if cleanup != nil || !errors.Is(err, refused) || !errors.Is(err, stopFailure) {
		t.Fatalf("startup lost exact owned cleanup failure: cleanup=%t error=%v", cleanup != nil, err)
	}
}

// A child runs no code before it is in its Job: it starts suspended, joins the
// Job, and only then resumes. A child that exits at once can therefore never
// make the assignment fail, and nothing it starts escapes the Job.
func TestStartCapturedWindowsProcessAssignsBeforeTheChildRuns(t *testing.T) {
	var steps []string
	api := windowsProcessAPI{
		createJob:    func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) { return 10, nil },
		configureJob: func(windows.Handle, uint32, uintptr, uint32) (int, error) { return 1, nil },
		startCommand: func(command *exec.Cmd) error {
			if command.SysProcAttr == nil || command.SysProcAttr.CreationFlags&windows.CREATE_SUSPENDED == 0 {
				steps = append(steps, "start running")
			} else {
				steps = append(steps, "start suspended")
			}
			command.Process = &os.Process{Pid: 42}
			return nil
		},
		openProcess: func(uint32, bool, uint32) (windows.Handle, error) { return 20, nil },
		assignProcess: func(windows.Handle, windows.Handle) error {
			steps = append(steps, "assign")
			return nil
		},
		resumeProcess: func(pid uint32) error {
			if pid != 42 {
				t.Fatalf("resumed process %d, want 42", pid)
			}
			steps = append(steps, "resume")
			return nil
		},
		closeHandle: func(windows.Handle) error { return nil },
		stopCommand: func(*exec.Cmd) error { return nil },
	}
	command := exec.Command("fixture")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DEBUG_PROCESS}
	cleanup, err := startCapturedWindowsProcess(command, api)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"start suspended", "assign", "resume"}; !slices.Equal(steps, want) {
		t.Fatalf("start sequence = %v, want %v", steps, want)
	}
	if command.SysProcAttr.CreationFlags&windows.DEBUG_PROCESS == 0 {
		t.Fatal("suspended start dropped the caller's creation flags")
	}
}
