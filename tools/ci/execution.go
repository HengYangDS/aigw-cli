package main

import (
	nativeprocess "aigw-cli/internal/platform/process"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type command struct {
	Name string
	Args []string
	// Env extends this process's environment, unless the command is Isolated:
	// an isolated command receives exactly Env.
	Env      []string
	Isolated bool
	Dir      string
	Input    string
	Timeout  time.Duration
}

// environment is the complete environment that call's process receives.
func (call command) environment() []string {
	if call.Isolated {
		return slices.Clone(call.Env)
	}
	inherited := exec.Cmd{Dir: call.Dir}
	return append(inherited.Environ(), call.Env...)
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
	// Native file descriptors preserve diagnostics from tools that exit before
	// their asynchronous pipe writes drain.
	runErr := runOwned(call, func(ctx context.Context, plan nativeprocess.Plan) error {
		return (nativeprocess.Runner{}).RunStream(ctx, plan, capture, capture)
	})
	closeErr := capture.Close()
	output, readErr := os.ReadFile(capture.Name())
	return output, errors.Join(runErr, closeErr, readErr)
}

// runOwned runs call's plan through run and the native process owner: in its
// own process group or Job, stopped when this process is interrupted or the
// call's timeout expires, and with every remaining descendant reclaimed before
// return.
func runOwned(call command, run func(context.Context, nativeprocess.Plan) error) error {
	ctx := context.Background()
	if call.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, call.Timeout)
		defer cancel()
	}
	plan := nativeprocess.Plan{
		Executable: call.Name, Args: call.Args, Directory: call.Dir,
		Env: call.environment(), Stdin: call.Input,
	}
	return errors.Join(run(ctx, plan), ctx.Err())
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

func systemRunner(call command) error {
	// A native descriptor preserves diagnostic writes before immediate exit;
	// standard output remains live and diagnostics replay without a memory limit.
	var diagnostics nativeprocess.Diagnostics
	err := runOwned(call, func(ctx context.Context, plan nativeprocess.Plan) (runErr error) {
		diagnostics, runErr = (nativeprocess.Runner{}).RunDiagnosed(ctx, plan, os.Stdout, os.Stderr)
		return runErr
	})
	text := strings.TrimSpace(string(diagnostics.Head))
	if diagnostics.Truncated {
		text += "\n[diagnostics truncated]"
	}
	if err != nil {
		if text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}
	if diagnostics.Truncated || diagnostics.Failure {
		return fmt.Errorf("native gate diagnostics prevent qualification: %s", text)
	}
	return nil
}
