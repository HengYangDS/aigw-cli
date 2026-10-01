//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func prepareVerificationInterrupt(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_CONSOLE,
		HideWindow:    true,
	}
}

func interruptVerificationCommand(command *exec.Cmd, environment []string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sender := exec.CommandContext(ctx, executable, strconv.Itoa(command.Process.Pid))
	sender.Env = environmentWith(environment, map[string]string{"AIGW_TEST_RESOURCE_ROLE": "interrupt"})
	sender.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := sender.CombinedOutput(); err != nil {
		return fmt.Errorf("send owned native console interruption: %w: %s", err, output)
	}
	return nil
}

func sendVerificationConsoleInterrupt(pid int) (result error) {
	processID, err := strconv.ParseUint(strconv.Itoa(pid), 10, 32)
	if err != nil || processID == 0 {
		return errors.New("owned verification process ID is outside the native DWORD range")
	}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	_, _, _ = kernel.NewProc("FreeConsole").Call()
	attached, _, err := kernel.NewProc("AttachConsole").Call(uintptr(processID))
	if attached == 0 {
		return fmt.Errorf("attach owned verification console: %w", err)
	}
	defer func() {
		freed, _, err := kernel.NewProc("FreeConsole").Call()
		if freed == 0 {
			result = errors.Join(result, fmt.Errorf("detach owned verification console: %w", err))
		}
	}()
	handled := make(chan struct{}, 1)
	handler := windows.NewCallback(func(event uint32) uintptr {
		if event == windows.CTRL_BREAK_EVENT {
			select {
			case handled <- struct{}{}:
			default:
			}
		}
		return 1
	})
	ignored, _, err := kernel.NewProc("SetConsoleCtrlHandler").Call(handler, 1)
	if ignored == 0 {
		return fmt.Errorf("exclude owned interruption sender from console event: %w", err)
	}
	defer func() {
		removed, _, err := kernel.NewProc("SetConsoleCtrlHandler").Call(handler, 0)
		if removed == 0 {
			result = errors.Join(result, fmt.Errorf("remove owned console interruption handler: %w", err))
		}
	}()
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, 0); err != nil {
		return err
	}
	// Win32 delivers the event on another thread. Keep the sender protected
	// until that thread acknowledges it, before unregistering the handler.
	select {
	case <-handled:
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("owned interruption sender did not acknowledge its console event")
	}
}

func observeVerificationProcess(t *testing.T, pid int, _, role string) func() bool {
	t.Helper()
	processID, err := strconv.ParseUint(strconv.Itoa(pid), 10, 32)
	if err != nil || processID == 0 {
		t.Fatal("owned process ID is outside the native DWORD range")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(processID))
	if err != nil {
		t.Fatalf("retain owned process %d handle: %v", pid, err)
	}
	t.Cleanup(func() {
		state, err := windows.WaitForSingleObject(handle, 0)
		if role != "unrelated" && err == nil && state == uint32(windows.WAIT_TIMEOUT) {
			if err := windows.TerminateProcess(handle, 1); err != nil {
				t.Errorf("terminate exact owned process %d after test failure: %v", pid, err)
			} else if state, err := windows.WaitForSingleObject(handle, 5000); err != nil || state != windows.WAIT_OBJECT_0 {
				t.Errorf("owned process %d termination was not confirmed: state=%d error=%v", pid, state, err)
			}
		}
		if err := windows.CloseHandle(handle); err != nil {
			t.Errorf("close owned process %d handle: %v", pid, err)
		}
	})
	return func() bool {
		state, err := windows.WaitForSingleObject(handle, 0)
		if err != nil {
			t.Fatalf("observe owned process %d handle: %v", pid, err)
		}
		return state == uint32(windows.WAIT_TIMEOUT)
	}
}
