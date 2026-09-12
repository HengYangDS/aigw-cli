//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessAPI struct {
	createJob     func(*windows.SecurityAttributes, *uint16) (windows.Handle, error)
	configureJob  func(windows.Handle, uint32, uintptr, uint32) (int, error)
	startCommand  func(*exec.Cmd) error
	openProcess   func(uint32, bool, uint32) (windows.Handle, error)
	assignProcess func(windows.Handle, windows.Handle) error
	closeHandle   func(windows.Handle) error
	stopCommand   func(*exec.Cmd)
}

var nativeWindowsProcessAPI = windowsProcessAPI{
	createJob:     windows.CreateJobObject,
	configureJob:  windows.SetInformationJobObject,
	startCommand:  func(command *exec.Cmd) error { return command.Start() },
	openProcess:   windows.OpenProcess,
	assignProcess: windows.AssignProcessToJobObject,
	closeHandle:   windows.CloseHandle,
	stopCommand: func(command *exec.Cmd) {
		_ = command.Process.Kill()
		_ = command.Wait()
	},
}

// startCapturedProcess puts an AIGW-owned non-interactive child into a Job
// Object. Closing the Job after Wait returns terminates only this invocation's
// remaining descendants, including wrappers that inherited stdout/stderr.
func startCapturedProcess(cmd *exec.Cmd) (func() error, error) {
	cleanup, err := startCapturedWindowsProcess(cmd, nativeWindowsProcessAPI)
	if err != nil {
		return nil, fmt.Errorf("start captured child: %w", err)
	}
	return cleanup, nil
}

func startCapturedWindowsProcess(command *exec.Cmd, api windowsProcessAPI) (cleanup func() error, err error) {
	job, err := api.createJob(nil, nil)
	if err != nil {
		return nil, err
	}
	closeJob := sync.OnceValue(func() error { return api.closeHandle(job) })
	defer func() {
		if err != nil {
			err = errors.Join(err, closeJob())
		}
	}()
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	var pin runtime.Pinner
	pin.Pin(&info)
	defer pin.Unpin()
	if _, err := api.configureJob(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), // #nosec G103 -- The correctly sized Win32 struct remains pinned through the call.
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		return nil, err
	}
	if err := api.startCommand(command); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			api.stopCommand(command)
		}
	}()
	process, err := api.openProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(command.Process.Pid), // #nosec G115 -- exec.Start obtained the process identifier from the Win32 DWORD result.
	)
	if err != nil {
		return nil, err
	}
	assignErr := api.assignProcess(job, process)
	if err := errors.Join(assignErr, api.closeHandle(process)); err != nil {
		return nil, err
	}
	return closeJob, nil
}
