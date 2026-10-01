//go:build !windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func prepareVerificationInterrupt(_ *exec.Cmd) {}

func interruptVerificationCommand(command *exec.Cmd, _ []string) error {
	return command.Process.Signal(syscall.SIGTERM)
}

func sendVerificationConsoleInterrupt(_ int) error {
	return errors.New("console interruption is a Windows fixture operation")
}

func observeVerificationProcess(t *testing.T, pid int) func() bool {
	t.Helper()
	return func() bool {
		if runtime.GOOS == "linux" {
			data, err := os.ReadFile(filepath.Join(string(filepath.Separator), "proc", strconv.Itoa(pid), "stat"))
			if errors.Is(err, os.ErrNotExist) {
				return false
			}
			if err != nil {
				t.Fatalf("observe owned process %d: %v", pid, err)
			}
			fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
			if len(fields) > 0 && (fields[0] == "Z" || fields[0] == "X") {
				return false
			}
		}
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return false
		}
		if err != nil {
			t.Fatalf("observe owned process %d: %v", pid, err)
		}
		return true
	}
}
