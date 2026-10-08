//go:build windows

package performance

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	processKernel  = windows.NewLazySystemDLL("kernel32.dll")
	processMachine = processKernel.NewProc("GetProcessInformation")
	debugWait      = processKernel.NewProc("WaitForDebugEvent")
	debugContinue  = processKernel.NewProc("ContinueDebugEvent")
	debugStop      = processKernel.NewProc("DebugActiveProcessStop")
)

// The largest DEBUG_EVENT union member supplies the native pointer alignment
// and size on both 32-bit and 64-bit Windows.
type debugException struct {
	Code, Flags     uint32
	Record, Address uintptr
	Parameters      uint32
	Information     [15]uintptr
}

type debugEvent struct {
	Code, PID, TID uint32
	Info           struct {
		Exception   debugException
		FirstChance uint32
	}
}

type debugCreate struct {
	File, Process, Thread windows.Handle
	Base                  uintptr
	Offset, Size          uint32
	TLS, Start, Name      uintptr
	Unicode               uint16
}

func processCreation(handle windows.Handle) (uint64, error) {
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), nil
}

func observeWindowsProcess(handle windows.Handle, role string, owned map[uint32]windows.Handle) (Execution, error) {
	var row Execution
	var basic windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(handle, windows.ProcessBasicInformation, unsafe.Pointer(&basic), uint32(unsafe.Sizeof(basic)), nil); err != nil { // #nosec G103 -- Win32 writes a correctly sized process record during this synchronous call.
		return row, err
	}
	if basic.UniqueProcessId > 1<<32-1 || basic.InheritedFromUniqueProcessId > 1<<32-1 {
		return row, errors.New("Windows process identity exceeds DWORD")
	}
	row.PID, row.ParentPID, row.Role = uint32(basic.UniqueProcessId), uint32(basic.InheritedFromUniqueProcessId), role // #nosec G115 -- Both native values were bounded to DWORD above.
	var err error
	row.Created, err = processCreation(handle)
	if err != nil {
		return row, err
	}
	parent := owned[row.ParentPID]
	closeParent := false
	if parent == 0 {
		parent, err = windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, row.ParentPID)
		if err != nil {
			return row, err
		}
		closeParent = true
	}
	row.ParentCreated, err = processCreation(parent)
	if closeParent {
		err = errors.Join(err, windows.CloseHandle(parent))
	}
	if err != nil {
		return row, err
	}
	path := make([]uint16, 32768)
	size := uint32(32768)
	if err := windows.QueryFullProcessImageName(handle, 0, &path[0], &size); err != nil {
		return row, err
	}
	row.Image, err = Identify(windows.UTF16ToString(path[:size]))
	if err != nil {
		return row, err
	}
	if err := windows.IsWow64Process2(handle, &row.WOW64Machine, &row.NativeMachine); err != nil {
		return row, err
	}
	var machine struct {
		Machine, Reserved uint16
		Attributes        uint32
	}
	result, _, callErr := processMachine.Call(uintptr(handle), 9, uintptr(unsafe.Pointer(&machine)), unsafe.Sizeof(machine)) // #nosec G103 -- ProcessMachineTypeInfo uses this exact eight-byte Win32 output, alive through the call.
	runtime.KeepAlive(&machine)
	if result == 0 {
		return row, fmt.Errorf("native process machine observation: %w", callErr)
	}
	row.Machine, row.Arch, row.Attributes = machine.Machine, processArchitecture(machine.Machine), machine.Attributes
	return row, nil
}

// ObserveCurrent binds the verifier to its actual native process and parent.
func ObserveCurrent() (rows []Execution, result error) {
	row, err := observeWindowsProcess(windows.CurrentProcess(), "controller", nil)
	if err != nil {
		return nil, err
	}
	return []Execution{row}, reviewExecution(&row.Image, []Execution{row}, "windows")
}

// ObserveCommand runs one untimed native preflight. DEBUG_PROCESS delivers
// each descendant's create event before it executes, so even immediate-exit
// credential workers cannot escape into a PID/PE-header inference.
func ObserveCommand(parent context.Context, input Command, workload Workload) (rows, controllers []Execution, result error) {
	if len(workload.Command) == 0 || input.Measurement.Executable == nil {
		return nil, nil, errors.New("native preflight requires an explicit workload")
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if len(workload.Prepare) != 0 {
		_, _, err := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: workload.Prepare[0], Args: workload.Prepare[1:], Env: input.Environment, Directory: input.Directory})
		if err != nil {
			return nil, nil, fmt.Errorf("native preflight preparation: %s", redaction.Text(err.Error(), input.Sensitive...))
		}
	}
	selected, err := Identify(input.Tool)
	if err != nil {
		return nil, nil, err
	}
	observer := windowsObserver{selected: input.Measurement, handles: make(map[uint32]windows.Handle), active: make(map[uint32]windows.Handle), continueEvent: continueWindowsDebugEvent}
	stdout, stderr, runErr := (process.Runner{StdoutLimit: 4 << 20}).RunCaptureStreams(ctx, process.Plan{
		Executable: input.Tool, Args: []string{"--shell=none", "--warmup", "0", "--runs", "1", "--output=inherit", "--style", "basic", Argv(workload.Command...)},
		Env: input.Environment, Directory: input.Directory, DebugProcess: true,
		OnStart: func(child *os.Process) error { return observer.collect(ctx, uint32(child.Pid)) }, // #nosec G115 -- exec.Start obtains the PID from the native DWORD result.
	})
	if process.DiagnosticFailure(stderr) {
		runErr = errors.Join(runErr, errors.New("native preflight emitted diagnostics"))
	}
	for name, stream := range map[string][]byte{"stdout": stdout, "stderr": stderr} {
		path := strings.TrimSuffix(input.Output, ".json") + ".execution." + name
		runErr = errors.Join(runErr, os.WriteFile(path, []byte(redaction.Text(string(stream), input.Sensitive...)), 0o600))
	}
	rows, controllers = observer.rows, observer.controllers
	row := input.Measurement
	row.Execution, row.ControllerExecution = rows, controllers
	runErr = errors.Join(runErr, reviewWorkloadExecution(row, selected, "windows"))
	if runErr != nil {
		return rows, controllers, errors.New(redaction.Text(runErr.Error(), input.Sensitive...))
	}
	return rows, controllers, nil
}

type windowsObserver struct {
	selected          Measurement
	rows, controllers []Execution
	handles           map[uint32]windows.Handle
	active            map[uint32]windows.Handle
	continueEvent     func(debugEvent, uintptr) error
	waitEvent         func(*debugEvent, uint32) (uintptr, error)
}

func (o *windowsObserver) collect(ctx context.Context, root uint32) (result error) {
	defer func() { result = errors.Join(result, o.reclaim()) }()
	wait := o.waitEvent
	if wait == nil {
		wait = waitWindowsDebugEvent
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var event debugEvent
		ready, err := wait(&event, 50)
		if ready == 0 {
			if errors.Is(err, windows.ERROR_SEM_TIMEOUT) {
				continue
			}
			return fmt.Errorf("wait for owned native debug event: %w", err)
		}
		status := uintptr(0x00010002) // DBG_CONTINUE
		var eventErr error
		switch event.Code {
		case 3: // CREATE_PROCESS_DEBUG_EVENT
			info := (*debugCreate)(unsafe.Pointer(&event.Info)) // #nosec G103 -- The create-event discriminator selects this exact native union member.
			eventErr = o.created(event.PID, root, info.Process)
			if info.File != 0 {
				eventErr = errors.Join(eventErr, windows.CloseHandle(info.File))
			}
		case 5: // EXIT_PROCESS_DEBUG_EVENT
			if event.Info.Exception.Code != 0 {
				eventErr = fmt.Errorf("observed native process exited with status %d", event.Info.Exception.Code)
			}
		case 6: // LOAD_DLL_DEBUG_EVENT owns an image-file handle, not credential data.
			info := (*debugCreate)(unsafe.Pointer(&event.Info)) // #nosec G103 -- The DLL-event union starts with its native file handle.
			if info.File != 0 {
				eventErr = windows.CloseHandle(info.File)
			}
		case 1: // Only native debugger breakpoints are consumed; application faults remain faults.
			if event.Info.Exception.Code != 0x80000003 && event.Info.Exception.Code != 0x80000004 {
				status = 0x80010001 // DBG_EXCEPTION_NOT_HANDLED
			}
		case 9:
			eventErr = errors.New("native debugger reported a RIP event")
		}
		if event.Code == 3 && eventErr != nil {
			return eventErr
		}
		continueErr := o.continueEvent(event, status)
		if continueErr == nil && event.Code == 5 {
			delete(o.active, event.PID)
		}
		eventErr = errors.Join(eventErr, continueErr)
		if eventErr != nil {
			return eventErr
		}
		if len(o.active) == 0 && len(o.controllers) != 0 {
			return nil
		}
	}
}

func waitWindowsDebugEvent(event *debugEvent, timeout uint32) (uintptr, error) {
	ready, _, err := debugWait.Call(uintptr(unsafe.Pointer(event)), uintptr(timeout)) // #nosec G103 -- The native DEBUG_EVENT buffer remains typed until this direct Win32 call.
	runtime.KeepAlive(event)
	return ready, err
}

func continueWindowsDebugEvent(event debugEvent, status uintptr) error {
	continued, _, err := debugContinue.Call(uintptr(event.PID), uintptr(event.TID), status)
	if continued == 0 {
		return fmt.Errorf("continue owned native debug event: %w", err)
	}
	return nil
}

func (o *windowsObserver) reclaim() (result error) {
	deadline := time.Now().Add(2 * time.Second)
	for pid, handle := range o.active {
		result = errors.Join(result, windows.TerminateProcess(handle, 1))
		stopped, _, err := debugStop.Call(uintptr(pid))
		if stopped == 0 {
			result = errors.Join(result, fmt.Errorf("detach owned debug process: %w", err))
		}
	}
	for _, handle := range o.handles {
		remaining := max(time.Until(deadline).Milliseconds(), 0)
		event, err := windows.WaitForSingleObject(handle, uint32(remaining)) // #nosec G115 -- The shared remaining budget is bounded to 0..2000 milliseconds.
		result = errors.Join(result, err)
		if err == nil && event != windows.WAIT_OBJECT_0 {
			result = errors.Join(result, errors.New("owned native preflight process did not exit"))
		}
		result = errors.Join(result, windows.CloseHandle(handle))
	}
	return result
}

func (o *windowsObserver) created(pid, root uint32, handle windows.Handle) error {
	if _, duplicate := o.handles[pid]; duplicate {
		return errors.New("native preflight observed reused process identity")
	}
	// Windows owns the event handle; cleanup owns its process even if duplication fails.
	o.active[pid] = handle
	var retained windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), handle, windows.CurrentProcess(), &retained, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	o.handles[pid], o.active[pid] = retained, retained
	row, err := observeWindowsProcess(retained, "descendant", o.handles)
	if err != nil {
		return err
	}
	if row.PID != pid {
		return errors.New("native process event differs from its retained handle")
	}
	if pid == root {
		row.Role = "controller"
		o.controllers = append(o.controllers, row)
		return nil
	}
	if row.ParentPID == root {
		if len(o.rows) != 0 || !o.selected.Executable.sameFile(row.Image) {
			return errors.New("native preflight did not execute exactly its selected workload")
		}
		row.Role = "workload"
		o.rows = append(o.rows, row)
		return nil
	}
	if o.selected.Reader != nil && o.selected.Reader.sameFile(row.Image) {
		for _, parent := range o.rows {
			if parent.PID == row.ParentPID && parent.Created == row.ParentCreated {
				switch parent.Role {
				case "workload":
					row.Role = "reader"
				case "reader":
					row.Role = "credential-worker"
				}
			}
		}
	}
	o.rows = append(o.rows, row)
	return nil
}
