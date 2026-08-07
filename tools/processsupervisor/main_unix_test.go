//go:build !windows

package main

import (
	"bytes"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func shell() string { return "/bin/sh" }

func shellExitArgs(code int) []string { return []string{"-c", "exit " + strconv.Itoa(code)} }

func shellSleepArgs(seconds int) []string { return []string{"-c", "sleep " + strconv.Itoa(seconds)} }

func TestRunForwardsTerminationSignal(t *testing.T) {
	done := make(chan int, 1)
	go func() {
		done <- run(append([]string{"5", shell()}, shellSleepArgs(5)...), &bytes.Buffer{})
	}()
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 128+int(syscall.SIGTERM) {
		t.Fatalf("code=%d", code)
	}
}
