//go:build windows

package main

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startOwned(command *exec.Cmd) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if err := command.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { windows.CloseHandle(job) }) }, nil
}
