//go:build windows

// Package winjob owns the Windows Job Object lifecycle used by bounded AIGW
// child processes.
package winjob

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Supported reports whether Windows Job Objects are available in this build.
func Supported() bool { return true }

var (
	createJobObject       = windows.CreateJobObject
	setInformationJob     = windows.SetInformationJobObject
	openProcess           = windows.OpenProcess
	assignProcessToJob    = windows.AssignProcessToJobObject
	closeHandle           = windows.CloseHandle
	startCommand          = func(command *exec.Cmd) error { return command.Start() }
	killAndWaitForCommand = func(command *exec.Cmd) { _ = command.Process.Kill(); _ = command.Wait() }
)

// Start launches command in a kill-on-close Job Object and returns an
// idempotent cleanup function.
func Start(command *exec.Cmd) (func(), error) {
	job, err := createJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	closeJob := func() { _ = closeHandle(job) }
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := setInformationJob(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		closeJob()
		return nil, err
	}
	if err := startCommand(command); err != nil {
		closeJob()
		return nil, err
	}
	process, err := openProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(command.Process.Pid),
	)
	if err != nil {
		killAndWaitForCommand(command)
		closeJob()
		return nil, err
	}
	defer closeHandle(process)
	if err := assignProcessToJob(job, process); err != nil {
		killAndWaitForCommand(command)
		closeJob()
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(closeJob) }, nil
}
