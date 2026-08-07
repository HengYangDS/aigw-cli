//go:build windows

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
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

func TestRunForwardsInterrupt(t *testing.T) {
	originalNotify, originalStop := notifySignals, stopSignals
	t.Cleanup(func() { notifySignals, stopSignals = originalNotify, originalStop })
	notifySignals = func(channel chan<- os.Signal, _ ...os.Signal) {
		go func() { time.Sleep(20 * time.Millisecond); channel <- os.Interrupt }()
	}
	stopSignals = func(chan<- os.Signal) {}
	done := make(chan int, 1)
	go func() {
		done <- run(append([]string{"5", shell()}, shellSleepArgs(5)...), &bytes.Buffer{})
	}()
	select {
	case code := <-done:
		if code != 128+int(syscall.SIGINT) {
			t.Fatalf("code=%d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("process supervisor did not observe Ctrl+Break")
	}
}
