//go:build performance_acceptance && linux

package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// measurePeakMemory uses GNU time's small post-exec supervisor so the Go
// parent's address space cannot become the measured child's initial high-water.
func measurePeakMemory(command *exec.Cmd) (peak uint64, result error) {
	report, err := os.CreateTemp("", "aigw-peak-memory-*")
	if err != nil {
		return 0, err
	}
	defer func() { result = errors.Join(result, os.Remove(report.Name())) }()
	if err := report.Close(); err != nil {
		return 0, err
	}
	arguments := []string{"time", "--format=%M", "--output=" + report.Name(), "--", command.Path}
	command.Args = append(arguments, command.Args[1:]...)
	command.Path = "/usr/bin/time"
	if err := command.Run(); err != nil {
		return 0, err
	}
	data, err := os.ReadFile(report.Name())
	if err != nil {
		return 0, err
	}
	kibibytes, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || kibibytes == 0 || kibibytes > ^uint64(0)/1024 {
		return 0, errors.New("GNU time did not report a positive native peak resident set")
	}
	return kibibytes * 1024, nil
}
