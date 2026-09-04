//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"testing"

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
				closeHandle: func(windows.Handle) error {
					closed++
					return nil
				},
				stopCommand: func(*exec.Cmd) { stopped = true },
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
		stopCommand: func(*exec.Cmd) {},
	}

	cleanup, err := startCapturedWindowsProcess(exec.Command("fixture"), api)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	cleanup()
	if closed != 2 {
		t.Fatalf("closed=%d", closed)
	}
}
