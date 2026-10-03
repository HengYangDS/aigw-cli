//go:build !windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func prepareVerificationInterrupt(_ *exec.Cmd) {}

func interruptVerificationCommand(command *exec.Cmd, _ []string) error {
	return command.Process.Signal(syscall.SIGTERM)
}

func sendVerificationConsoleInterrupt(_ int) error {
	return errors.New("console interruption is a Windows fixture operation")
}

func observeVerificationProcess(t *testing.T, pid int, control, role string) func() bool {
	t.Helper()
	var owned verificationResourceProcess
	if err := json.Unmarshal(readFile(t, filepath.Join(control, role+".json")), &owned); err != nil ||
		owned.PID != pid || owned.Control != control || owned.Role != role {
		t.Fatalf("owned process identity does not match its private control channel: %v", err)
	}
	alive := func() bool {
		if runtime.GOOS == "linux" {
			data, err := os.ReadFile(filepath.Join(string(filepath.Separator), "proc", strconv.Itoa(pid), "stat"))
			if verificationProcessAbsent(err) {
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
		if verificationProcessAbsent(err) {
			return false
		}
		if err != nil {
			t.Fatalf("observe owned process %d: %v", pid, err)
		}
		return true
	}
	if role == "unrelated" {
		return alive
	}
	t.Cleanup(func() {
		if !alive() {
			return
		}
		if err := os.WriteFile(filepath.Join(control, role+".stop"), []byte("stop"), 0o600); err != nil {
			t.Errorf("stop exact owned fixture %s: %v", role, err)
			return
		}
		deadline := time.Now().Add(5 * time.Second)
		for alive() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if alive() {
			t.Errorf("owned fixture %s did not stop after assertion failure", role)
		}
	})
	return alive
}

func verificationProcessAbsent(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

func TestVerificationProcessAbsenceClassifiesKernelExitRace(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		absent bool
	}{
		{"running", nil, false},
		{"removed", &os.PathError{Op: "open", Path: "/proc/123/stat", Err: syscall.ENOENT}, true},
		{"exited during read", &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.ESRCH}, true},
		{"signal target exited", syscall.ESRCH, true},
		{"permission denied", &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.EACCES}, false},
		{"operation denied", syscall.EPERM, false},
		{"read failed", &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.EIO}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := verificationProcessAbsent(test.err); got != test.absent {
				t.Errorf("process absence for %v = %t, want %t", test.err, got, test.absent)
			}
		})
	}
}
