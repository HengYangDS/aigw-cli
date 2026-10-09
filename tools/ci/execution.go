package main

import (
	nativeprocess "aigw-cli/internal/process"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type command struct {
	Name    string
	Args    []string
	Env     []string
	Dir     string
	Input   string
	Timeout time.Duration
}

const commandDiagnosticLimit = 64 * 1024

type diagnosticCapture struct {
	output    bytes.Buffer
	truncated bool
}

func (capture *diagnosticCapture) Write(data []byte) (int, error) {
	remaining := commandDiagnosticLimit - capture.output.Len()
	if remaining > 0 {
		_, _ = capture.output.Write(data[:min(len(data), remaining)])
	}
	capture.truncated = capture.truncated || len(data) > remaining
	return len(data), nil
}

func (capture *diagnosticCapture) text() string {
	diagnostics := strings.TrimSpace(capture.output.String())
	if capture.truncated {
		return diagnostics + "\n[diagnostics truncated]"
	}
	return diagnostics
}

type commandRunner func(command) error

type outputRunner func(command) ([]byte, error)

func systemOutputRunner(call command) (output []byte, err error) {
	capture, err := os.CreateTemp("", "aigw-ci-output-*")
	if err != nil {
		return nil, fmt.Errorf("create command output: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.Remove(capture.Name()))
	}()
	ctx := context.Background()
	if call.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, call.Timeout)
		defer cancel()
	}
	environment := exec.Cmd{Dir: call.Dir}
	plan := nativeprocess.Plan{
		Executable: call.Name, Args: call.Args, Directory: call.Dir,
		Env: append(environment.Environ(), call.Env...), Stdin: call.Input,
	}
	// Native file descriptors preserve diagnostics from tools that exit before
	// their asynchronous pipe writes drain.
	runErr := (nativeprocess.Runner{}).RunStream(ctx, plan, capture, capture)
	closeErr := capture.Close()
	output, readErr := os.ReadFile(capture.Name())
	return output, errors.Join(runErr, closeErr, readErr, ctx.Err())
}

func runCommands(commands []command, stdout io.Writer, runner commandRunner) error {
	for _, call := range commands {
		if _, err := fmt.Fprintf(stdout, "==> %s\n", call.Name); err != nil {
			return fmt.Errorf("report gate %s before execution: %w", call.Name, err)
		}
		if err := runner(call); err != nil {
			return fmt.Errorf("%s: %w", call.Name, err)
		}
	}
	return nil
}

func systemRunner(call command) (err error) {
	capture, err := os.CreateTemp("", "aigw-ci-diagnostics-*")
	if err != nil {
		return fmt.Errorf("create command diagnostics: %w", err)
	}
	defer func() {
		err = errors.Join(err, capture.Close(), os.Remove(capture.Name()))
	}()
	process := exec.Command(call.Name, call.Args...)
	process.Dir = call.Dir
	process.Env = append(process.Environ(), call.Env...)
	process.Stdin = strings.NewReader(call.Input)
	process.Stdout = os.Stdout
	// A native descriptor preserves diagnostic writes before immediate exit;
	// standard output remains live and diagnostics replay without a memory limit.
	process.Stderr = capture
	runErr := process.Run()
	var failureOutput diagnosticCapture
	_, seekErr := capture.Seek(0, io.SeekStart)
	var readErr error
	if seekErr == nil {
		_, readErr = io.Copy(io.MultiWriter(os.Stderr, &failureOutput), capture)
	}
	if err = errors.Join(runErr, seekErr, readErr); err != nil {
		if text := failureOutput.text(); text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}
	if failureOutput.truncated || nativeprocess.DiagnosticFailure(failureOutput.output.Bytes()) {
		return fmt.Errorf("native gate diagnostics prevent qualification: %s", failureOutput.text())
	}
	return nil
}
