//go:build windows

package process

import (
	"fmt"
	"os/exec"
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
func startCapturedProcess(cmd *exec.Cmd) (func(), error) {
	cleanup, err := startCapturedWindowsProcess(cmd, nativeWindowsProcessAPI)
	if err != nil {
		return nil, fmt.Errorf("start captured child: %w", err)
	}
	return cleanup, nil
}

func startCapturedWindowsProcess(command *exec.Cmd, api windowsProcessAPI) (func(), error) {
	job, err := api.createJob(nil, nil)
	if err != nil {
		return nil, err
	}
	closeJob := func() { _ = api.closeHandle(job) }
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := api.configureJob(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		closeJob()
		return nil, err
	}
	if err := api.startCommand(command); err != nil {
		closeJob()
		return nil, err
	}
	process, err := api.openProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(command.Process.Pid),
	)
	if err != nil {
		api.stopCommand(command)
		closeJob()
		return nil, err
	}
	defer api.closeHandle(process)
	if err := api.assignProcess(job, process); err != nil {
		api.stopCommand(command)
		closeJob()
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(closeJob) }, nil
}
