//go:build windows

package main

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func shell() string { return "cmd.exe" }

func shellExitArgs(code int) []string { return []string{"/c", "exit " + strconv.Itoa(code)} }

func shellSleepArgs(seconds int) []string {
	return []string{"/c", "ping -n " + strconv.Itoa(seconds+1) + " 127.0.0.1 >NUL"}
}

func TestStartOwnedReportsMissingExecutable(t *testing.T) {
	cleanup, err := startOwned(exec.Command(filepath.Join(t.TempDir(), "missing.exe")))
	if err == nil || cleanup != nil {
		t.Fatalf("cleanup_present=%t error=%v", cleanup != nil, err)
	}
}
