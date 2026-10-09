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
	"path/filepath"
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
	plan := nativeprocess.Plan{
		Executable: call.Name, Args: call.Args, Directory: call.Dir,
		Env: append(os.Environ(), call.Env...), Stdin: call.Input,
	}
	// Native file descriptors preserve diagnostics from tools that exit before
	// their asynchronous pipe writes drain.
	runErr := (nativeprocess.Runner{}).RunStream(ctx, plan, capture, capture)
	closeErr := capture.Close()
	output, readErr := os.ReadFile(capture.Name())
	return output, errors.Join(runErr, closeErr, readErr, ctx.Err())
}

func qualifyProtectedResource(path string, stdout io.Writer, inquiry outputRunner) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("native review requires an exact absolute protected resource")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("protected resource identity is unproved: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("protected resource identity is unproved: not a regular file")
	}
	file, err := os.Open(path)
	if err == nil {
		return errors.Join(errors.New("protected resource is readable by native review identity"), file.Close())
	}
	if !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("protected resource denial is unproved: %w", err)
	}
	output, err := inquiry(command{Name: "sudo", Args: []string{"-n", "-l"}, Env: []string{"LC_ALL=C", "LANG=C"}, Timeout: 5 * time.Second})
	noGrant := strings.Contains(string(output), "is not allowed to run sudo") || strings.Contains(string(output), "is not in the sudoers file")
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !noGrant {
		return errors.New("native review identity has an elevation grant or its absence is unproved")
	}
	_, err = fmt.Fprintf(stdout, "Native review resource=%s uid=%d direct_read=denied sudo_grant=none\n", path, os.Geteuid())
	return err
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
