// Command processsupervisor runs one owned command for a bounded number of seconds.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const maxSeconds = 300

var (
	notifySignals = signal.Notify
	stopSignals   = signal.Stop
)

func main() {
	os.Exit(runProcess(os.Args[1:], os.Stderr, startOwned))
}

func run(args []string, stderr io.Writer) int {
	return runProcess(args, stderr, startOwned)
}

func runProcess(args []string, stderr io.Writer, start func(*exec.Cmd) (func(), error)) int {
	if len(args) < 2 {
		_, _ = fmt.Fprintln(stderr, "usage: processsupervisor <seconds> <command> [args...]")
		return 2
	}
	seconds, err := strconv.Atoi(args[0])
	if err != nil || seconds < 1 || seconds > maxSeconds {
		_, _ = fmt.Fprintln(stderr, "timeout must be an integer from 1 through 300")
		return 2
	}

	signals := make(chan os.Signal, 1)
	notifySignals(signals, os.Interrupt, syscall.SIGTERM)
	defer stopSignals(signals)
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	command := exec.Command(args[1], args[2:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	cleanup, err := start(command)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 127
	}
	defer cleanup()

	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		cleanup()
		return exitStatus(err)
	case <-timer.C:
		cleanup()
		<-done
		return 124
	case received := <-signals:
		cleanup()
		<-done
		return 128 + signalNumber(received)
	}
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 127
}

func signalNumber(received os.Signal) int {
	return int(received.(syscall.Signal))
}
