package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRunReturnsCommandStatus(t *testing.T) {
	if code := run(append([]string{"5", shell()}, shellExitArgs(7)...), &bytes.Buffer{}); code != 7 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunTimesOutOwnedCommand(t *testing.T) {
	started := time.Now()
	if code := run(append([]string{"1", shell()}, shellSleepArgs(5)...), &bytes.Buffer{}); code != 124 {
		t.Fatalf("code=%d", code)
	}
	if time.Since(started) > 4*time.Second {
		t.Fatal("timeout exceeded its bounded cleanup window")
	}
}

func TestRunForwardsTerminationSignal(t *testing.T) {
	done := make(chan int, 1)
	go func() {
		done <- run(append([]string{"5", shell()}, shellSleepArgs(5)...), &bytes.Buffer{})
	}()
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 128+int(syscall.SIGTERM) {
		t.Fatalf("code=%d", code)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"0", "true"}, {"301", "true"}, {"not-a-number", "true"}} {
		if code := run(args, &bytes.Buffer{}); code != 2 {
			t.Fatalf("args=%v code=%d", args, code)
		}
	}
}

func TestRunReturnsStartupFailure(t *testing.T) {
	if code := run([]string{"1", filepath.Join(t.TempDir(), "missing")}, &bytes.Buffer{}); code != 127 {
		t.Fatalf("code=%d", code)
	}
}

func TestExitStatusAndSignalNumber(t *testing.T) {
	if exitStatus(nil) != 0 || exitStatus(errors.New("plain")) != 127 {
		t.Fatal("generic exit status contract failed")
	}
	command := exec.Command(shell(), shellExitArgs(9)...)
	err := command.Run()
	if exitStatus(err) != 9 {
		t.Fatalf("exit status = %d", exitStatus(err))
	}
	if signalNumber(syscall.SIGINT) != int(syscall.SIGINT) {
		t.Fatal("signal conversion failed")
	}
}

func TestRunProcessUsesInjectedStartFailure(t *testing.T) {
	code := runProcess([]string{"1", shell()}, &bytes.Buffer{}, func(*exec.Cmd) (func(), error) {
		return nil, errors.New("start rejected")
	})
	if code != 127 {
		t.Fatalf("code=%d", code)
	}
}

func TestCommandContextFixtureIsAvailable(t *testing.T) {
	command := exec.CommandContext(context.Background(), shell(), shellExitArgs(0)...)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}
