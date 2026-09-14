//go:build performance_acceptance && windows

package main

import (
	"errors"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processMemoryCounters follows the native PROCESS_MEMORY_COUNTERS ABI.
type processMemoryCounters struct {
	Size, PageFaultCount                               uint32
	PeakWorkingSet, WorkingSet                         uintptr
	QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage       uintptr
	QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage uintptr
	PagefileUsage, PeakPagefileUsage                   uintptr
}

// measurePeakMemory retains a query handle until the completed child's peak is read.
func measurePeakMemory(command *exec.Cmd) (peak uint64, result error) {
	if err := command.Start(); err != nil {
		return 0, err
	}
	defer func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	if command.Process.Pid < 1 || uint64(command.Process.Pid) > math.MaxUint32 {
		return 0, errors.New("child process ID is outside the Windows process range")
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(command.Process.Pid))
	if err != nil {
		return 0, err
	}
	defer func() { result = errors.Join(result, windows.CloseHandle(handle)) }()
	if err := command.Wait(); err != nil {
		return 0, err
	}
	counters := processMemoryCounters{Size: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	var pin runtime.Pinner
	pin.Pin(&counters)
	defer pin.Unpin()
	query := windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")
	if err := query.Find(); err != nil {
		return 0, err
	}
	success, _, callErr := query.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&counters)), // #nosec G103 -- Exact Win32 ABI; the output struct is sized and pinned until this synchronous call returns.
		uintptr(counters.Size),
	)
	if success == 0 {
		return 0, fmt.Errorf("read completed child memory: %w", callErr)
	}
	if counters.PeakWorkingSet == 0 {
		return 0, errors.New("completed child has no positive peak working-set observation")
	}
	return uint64(counters.PeakWorkingSet), nil
}
