//go:build windows

package winjob

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestStartOwnsEveryFailureBoundary(t *testing.T) {
	if !Supported() {
		t.Fatal("Windows build reported no Job Object support")
	}
	originalCreate := createJobObject
	originalSet := setInformationJob
	originalStart := startCommand
	originalOpen := openProcess
	originalAssign := assignProcessToJob
	originalClose := closeHandle
	originalKill := killAndWaitForCommand
	t.Cleanup(func() {
		createJobObject = originalCreate
		setInformationJob = originalSet
		startCommand = originalStart
		openProcess = originalOpen
		assignProcessToJob = originalAssign
		closeHandle = originalClose
		killAndWaitForCommand = originalKill
	})

	failure := errors.New("failure")
	tests := []struct {
		name       string
		failAt     string
		wantClosed int
		wantKilled bool
	}{
		{name: "create", failAt: "create"},
		{name: "configure", failAt: "configure", wantClosed: 1},
		{name: "start", failAt: "start", wantClosed: 1},
		{name: "open", failAt: "open", wantClosed: 1, wantKilled: true},
		{name: "assign", failAt: "assign", wantClosed: 2, wantKilled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			closed, killed := 0, false
			createJobObject = func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) {
				if test.failAt == "create" {
					return 0, failure
				}
				return 10, nil
			}
			setInformationJob = func(windows.Handle, uint32, uintptr, uint32) (ret int, err error) {
				if test.failAt == "configure" {
					return 0, failure
				}
				return 1, nil
			}
			startCommand = func(command *exec.Cmd) error {
				if test.failAt == "start" {
					return failure
				}
				command.Process = &os.Process{Pid: 42}
				return nil
			}
			openProcess = func(uint32, bool, uint32) (windows.Handle, error) {
				if test.failAt == "open" {
					return 0, failure
				}
				return 20, nil
			}
			assignProcessToJob = func(windows.Handle, windows.Handle) error {
				if test.failAt == "assign" {
					return failure
				}
				return nil
			}
			closeHandle = func(windows.Handle) error { closed++; return nil }
			killAndWaitForCommand = func(*exec.Cmd) { killed = true }

			cleanup, err := Start(exec.Command("fixture"))
			if err == nil || cleanup != nil || !errors.Is(err, failure) {
				t.Fatalf("cleanup_present=%t error=%v", cleanup != nil, err)
			}
			if closed != test.wantClosed || killed != test.wantKilled {
				t.Fatalf("closed=%d killed=%t", closed, killed)
			}
		})
	}
}

func TestStartReturnsIdempotentCleanup(t *testing.T) {
	originalCreate := createJobObject
	originalSet := setInformationJob
	originalStart := startCommand
	originalOpen := openProcess
	originalAssign := assignProcessToJob
	originalClose := closeHandle
	t.Cleanup(func() {
		createJobObject = originalCreate
		setInformationJob = originalSet
		startCommand = originalStart
		openProcess = originalOpen
		assignProcessToJob = originalAssign
		closeHandle = originalClose
	})

	closed := 0
	createJobObject = func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) { return 10, nil }
	setInformationJob = func(windows.Handle, uint32, uintptr, uint32) (int, error) { return 1, nil }
	startCommand = func(command *exec.Cmd) error { command.Process = &os.Process{Pid: 42}; return nil }
	openProcess = func(uint32, bool, uint32) (windows.Handle, error) { return 20, nil }
	assignProcessToJob = func(windows.Handle, windows.Handle) error { return nil }
	closeHandle = func(windows.Handle) error { closed++; return nil }

	cleanup, err := Start(exec.Command("fixture"))
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	cleanup()
	if closed != 2 {
		t.Fatalf("closed=%d", closed)
	}
}
