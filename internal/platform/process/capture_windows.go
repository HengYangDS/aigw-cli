//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessAPI struct {
	createJob     func(*windows.SecurityAttributes, *uint16) (windows.Handle, error)
	configureJob  func(windows.Handle, uint32, uintptr, uint32) (int, error)
	startCommand  func(*exec.Cmd) error
	openProcess   func(uint32, bool, uint32) (windows.Handle, error)
	assignProcess func(windows.Handle, windows.Handle) error
	resumeProcess func(uint32) error
	closeHandle   func(windows.Handle) error
	stopCommand   func(*exec.Cmd) error
}

// procGetProcessIDOfThread names a thread's owning process; x/sys does not wrap it.
var procGetProcessIDOfThread = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessIdOfThread")

var nativeWindowsProcessAPI = windowsProcessAPI{
	createJob:     windows.CreateJobObject,
	configureJob:  windows.SetInformationJobObject,
	startCommand:  func(command *exec.Cmd) error { return command.Start() },
	openProcess:   windows.OpenProcess,
	assignProcess: windows.AssignProcessToJobObject,
	resumeProcess: resumeSuspendedProcess,
	closeHandle:   windows.CloseHandle,
	stopCommand:   stopCapturedCommand,
}

// resumeSuspendedProcess resumes the threads of a process created with
// CREATE_SUSPENDED. os/exec closes the creation's thread handle, so the
// threads are found through a snapshot of the system's threads.
func resumeSuspendedProcess(pid uint32) (result error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot suspended child threads: %w", err)
	}
	defer func() { result = errors.Join(result, windows.CloseHandle(snapshot)) }()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := 0
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME|windows.THREAD_QUERY_LIMITED_INFORMATION, false, entry.ThreadID)
		if err != nil {
			return fmt.Errorf("open suspended child thread: %w", err)
		}
		// A thread identifier can be reused once its thread ends; resume only a
		// thread that still belongs to this child.
		owner, _, ownerErr := procGetProcessIDOfThread.Call(uintptr(thread))
		if uint32(owner) != pid { // #nosec G115 -- GetProcessIdOfThread returns a Win32 DWORD process identifier.
			return errors.Join(fmt.Errorf("suspended child thread %d now belongs to another process: %w", entry.ThreadID, ownerErr), windows.CloseHandle(thread))
		}
		_, resumeErr := windows.ResumeThread(thread)
		if err := errors.Join(resumeErr, windows.CloseHandle(thread)); err != nil {
			return fmt.Errorf("resume suspended child thread: %w", err)
		}
		resumed++
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("list suspended child threads: %w", err)
	}
	if resumed == 0 {
		return errors.New("suspended child has no thread to resume")
	}
	return nil
}

func stopCapturedCommand(command *exec.Cmd) (result error) {
	deadline := time.Now().Add(capturedProcessWaitDelay)
	handle, openErr := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(command.Process.Pid)) // #nosec G115 -- exec.Start returns the native DWORD PID; the child has not been reaped.
	killErr := command.Process.Kill()
	if openErr != nil {
		return errors.Join(openErr, killErr)
	}
	defer func() { result = errors.Join(result, windows.CloseHandle(handle)) }()
	event, err := windows.WaitForSingleObject(handle, 0)
	if err == nil && event != windows.WAIT_OBJECT_0 && command.SysProcAttr != nil && command.SysProcAttr.CreationFlags&windows.DEBUG_PROCESS != 0 {
		stopped, _, err := windows.NewLazySystemDLL("kernel32.dll").NewProc("DebugActiveProcessStop").Call(uintptr(command.Process.Pid))
		if stopped == 0 {
			result = fmt.Errorf("detach rejected native debug process: %w", err)
		}
	}
	remaining := max(time.Until(deadline).Milliseconds(), 0)
	event, err = windows.WaitForSingleObject(handle, uint32(remaining)) // #nosec G115 -- The shared remaining cleanup budget is bounded to 0..2000 milliseconds.
	if err != nil || event != windows.WAIT_OBJECT_0 {
		return errors.Join(result, killErr, err, errors.New("rejected native process did not exit within its cleanup bound"))
	}
	command.WaitDelay = max(time.Until(deadline), time.Millisecond)
	if err := command.Wait(); err != nil {
		if _, exited := errors.AsType[*exec.ExitError](err); !exited {
			result = errors.Join(result, err)
		}
	}
	return result
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
	// The child starts suspended and runs no code until it is in the Job: a
	// child that exits at once cannot fail the assignment, and nothing it
	// starts escapes the Job.
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if err := api.startCommand(command); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, api.stopCommand(command))
		}
	}()
	pid := uint32(command.Process.Pid) // #nosec G115 -- exec.Start obtained the process identifier from the Win32 DWORD result.
	process, err := api.openProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return nil, err
	}
	assignErr := api.assignProcess(job, process)
	if err := errors.Join(assignErr, api.closeHandle(process)); err != nil {
		return nil, err
	}
	if err := api.resumeProcess(pid); err != nil {
		return nil, err
	}
	return closeJob, nil
}
