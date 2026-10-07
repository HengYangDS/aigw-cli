//go:build performance_acceptance && darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func completedProcessUsage(state *os.ProcessState) *processUsage {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return nil
	}
	return &processUsage{MaxRSS: usage.Maxrss, RSSUnit: "bytes", MinorFaults: usage.Minflt, MajorFaults: usage.Majflt,
		InputBlocks: usage.Inblock, OutputBlocks: usage.Oublock, Voluntary: usage.Nvcsw, Involuntary: usage.Nivcsw}
}

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
