package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Runner struct{}

// CaptureRunner runs a bounded process and returns its standard output.
type CaptureRunner interface {
	RunCapture(context.Context, Plan) ([]byte, error)
}

const (
	capturedProcessOutputLimit = 64 * 1024
	capturedProcessWaitDelay   = 2 * time.Second
)

var errCapturedProcessOutputLimit = errors.New("captured process output exceeds limit")

// limitedBuffer deliberately does not embed bytes.Buffer: embedding would
// promote ReadFrom, and io.Copy would bypass Write's capture ceiling.
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

func (b *limitedBuffer) Len() int { return b.buf.Len() }

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

func (b *limitedBuffer) String() string { return b.buf.String() }

func (Runner) Run(ctx context.Context, plan Plan) error {
	if plan.Replace {
		return replaceProcess(plan)
	}
	cmd := commandContext(ctx, plan)
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", plan.Executable, err)
	}
	return nil
}

// RunCapture runs a bounded, non-interactive process invocation. It never
// embeds captured output in returned errors, so a misbehaving client cannot
// accidentally surface process environment or response material.
func (Runner) RunCapture(ctx context.Context, plan Plan) ([]byte, error) {
	if plan.Replace {
		return nil, fmt.Errorf("a captured process cannot replace the current process")
	}
	cmd := commandContext(ctx, plan)
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	cmd.WaitDelay = capturedProcessWaitDelay
	stdout := &limitedBuffer{limit: capturedProcessOutputLimit}
	stderr := &limitedBuffer{limit: capturedProcessOutputLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cleanup, err := startCapturedProcess(cmd)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", plan.Executable, err)
	}
	defer cleanup()
	if err := cmd.Wait(); err != nil {
		if stdout.overflow || stderr.overflow || errors.Is(err, errCapturedProcessOutputLimit) {
			return nil, fmt.Errorf("captured output from %s exceeds %d bytes", plan.Executable, capturedProcessOutputLimit)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%s exceeded its verification limit and its output pipes did not close within %s: %w", plan.Executable, capturedProcessWaitDelay, err)
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			return nil, fmt.Errorf("output pipes for %s did not close within %s: %w", plan.Executable, capturedProcessWaitDelay, err)
		}
		return nil, fmt.Errorf("run %s: %w", plan.Executable, err)
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}
