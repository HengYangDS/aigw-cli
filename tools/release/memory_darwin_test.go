//go:build performance_acceptance && darwin

package main

import (
	"errors"
	"os/exec"
	"syscall"
)

// measurePeakMemory reads this child's wait result, not cumulative child usage.
func measurePeakMemory(command *exec.Cmd) (uint64, error) {
	if err := command.Run(); err != nil {
		return 0, err
	}
	usage, ok := command.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok || usage.Maxrss <= 0 {
		return 0, errors.New("completed child has no positive peak resident memory observation")
	}
	return uint64(usage.Maxrss), nil
}
