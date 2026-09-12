package process

import (
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

// Runner executes process plans without consulting shell startup state.
type Runner struct {
	// StdoutLimit bounds captured result data; zero uses the diagnostic default.
	// Standard error always retains the smaller diagnostic limit.
	StdoutLimit int
}

// CaptureRunner runs a bounded process. It returns standard output on success
// and standard error with an execution error.
type CaptureRunner interface {
	RunCapture(context.Context, Plan) ([]byte, error)
}

// FileRunner streams a process's standard output into an owned file.
type FileRunner interface {
	RunToFile(context.Context, string, Plan) error
}

const (
	capturedProcessOutputLimit = 64 * 1024
	capturedProcessWaitDelay   = 2 * time.Second
)

var errCapturedProcessOutputLimit = errors.New("captured process output exceeds limit")

// limitedBuffer deliberately does not embed [bytes.Buffer]: embedding would
// promote ReadFrom, and [io.Copy] would bypass Write's capture ceiling.
type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.overflow = true
		return 0, errCapturedProcessOutputLimit
	}
	if len(data) > remaining {
		_, _ = b.buf.Write(data[:remaining])
		b.overflow = true
		return remaining, errCapturedProcessOutputLimit
	}
	return b.buf.Write(data)
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

func (b *limitedBuffer) String() string { return b.buf.String() }

// RunCapture runs a bounded, non-interactive process invocation. It returns
// standard output on success and standard error with a child-process failure.
// Captured bytes remain untrusted until the owning caller redacts them.
func (runner Runner) RunCapture(ctx context.Context, plan Plan) ([]byte, error) {
	if runner.StdoutLimit < 0 {
		return nil, fmt.Errorf("captured stdout limit must not be negative")
	}
	outputLimit := runner.StdoutLimit
	if outputLimit == 0 {
		outputLimit = capturedProcessOutputLimit
	}
	stdout := &limitedBuffer{limit: outputLimit}
	diagnostic, err := runCaptured(ctx, plan, stdout)
	if stdout.overflow {
		return nil, fmt.Errorf("captured stdout from %s exceeds %d bytes", plan.Executable, outputLimit)
	}
	if err != nil {
		return diagnostic, err
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

// RunToFile streams standard output without a memory capture limit. Failure
// removes the partial file; diagnostics retain the shared capture budget.
func (Runner) RunToFile(ctx context.Context, destination string, plan Plan) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open command output %s: %w", filepath.Base(destination), err)
	}
	// Keep the destination handle in this process. os/exec otherwise passes
	// *os.File directly to the child, whose descendants can retain it and
	// prevent failure cleanup on Windows after the pipe-drain deadline.
	diagnostic, runErr := runCaptured(ctx, plan, struct{ io.Writer }{file})
	closeErr := file.Close()
	if runErr == nil && closeErr == nil {
		return nil
	}
	removeErr := os.Remove(destination)
	if runErr != nil {
		runErr = fmt.Errorf("%s failed: %w: %s", plan.Executable, runErr, strings.TrimSpace(string(diagnostic)))
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("close command output %s: %w", filepath.Base(destination), closeErr)
	}
	return errors.Join(runErr, closeErr, removeErr)
}

func runCaptured(ctx context.Context, plan Plan, stdout io.Writer) (diagnostic []byte, err error) {
	cmd := commandContext(ctx, plan)
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	cmd.WaitDelay = capturedProcessWaitDelay
	stderr := &limitedBuffer{limit: capturedProcessOutputLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cleanup, err := startCapturedProcess(cmd)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", plan.Executable, err)
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	if err := cmd.Wait(); err != nil {
		if stderr.overflow {
			return nil, fmt.Errorf("captured stderr from %s exceeds %d bytes", plan.Executable, capturedProcessOutputLimit)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%s exceeded its verification limit and its output pipes did not close within %s: %w", plan.Executable, capturedProcessWaitDelay, err)
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			return nil, fmt.Errorf("output pipes for %s did not close within %s: %w", plan.Executable, capturedProcessWaitDelay, err)
		}
		return append([]byte(nil), stderr.Bytes()...), fmt.Errorf("run %s: %w", plan.Executable, err)
	}
	return nil, nil
}
