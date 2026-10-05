package construction

import (
	"aigw-cli/internal/process"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type toolCall struct {
	Name, Directory string
	Args, Env       []string
	Stdout, Stderr  io.Writer
	Timeout         time.Duration
}

type toolRunner func(toolCall) error

func executeTool(ctx context.Context) toolRunner {
	return func(call toolCall) (result error) {
		callContext := ctx
		if call.Timeout > 0 {
			var cancel context.CancelFunc
			callContext, cancel = context.WithTimeout(ctx, call.Timeout)
			defer cancel()
		}
		stdout := call.Stdout
		if stdout == nil {
			stdout = os.Stdout
		}
		stderr := call.Stderr
		if stderr == nil {
			stderr = os.Stderr
		}
		diagnostics, err := os.CreateTemp("", "aigw-release-diagnostics-*")
		if err != nil {
			return fmt.Errorf("create native tool diagnostics: %w", err)
		}
		defer func() {
			result = errors.Join(result, diagnostics.Close(), os.Remove(diagnostics.Name()))
		}()
		stdoutWriter := stdout
		file, native := stdout.(*os.File)
		if native {
			info, err := file.Stat()
			if err != nil {
				return fmt.Errorf("inspect native tool output: %w", err)
			}
			native = info.Mode().IsRegular()
		}
		if !native {
			capture, err := os.CreateTemp("", "aigw-release-output-*")
			if err != nil {
				return fmt.Errorf("create native tool output: %w", err)
			}
			defer func() {
				_, seekErr := capture.Seek(0, io.SeekStart)
				if seekErr == nil {
					_, seekErr = io.Copy(stdout, capture)
				}
				result = errors.Join(result, seekErr, capture.Close(), os.Remove(capture.Name()))
			}()
			stdoutWriter = capture
		}
		runErr := (process.Runner{}).RunStream(callContext, process.Plan{
			Executable: call.Name, Directory: call.Directory,
			Args: call.Args, Env: append(os.Environ(), call.Env...),
		}, stdoutWriter, diagnostics)
		if _, err := diagnostics.Seek(0, io.SeekStart); err != nil {
			return errors.Join(runErr, err)
		}
		reader := bufio.NewReader(diagnostics)
		for {
			line, readErr := reader.ReadBytes('\n')
			_, writeErr := stderr.Write(line)
			if process.DiagnosticFailure(line) {
				runErr = errors.Join(runErr, fmt.Errorf("%s diagnostics prevent qualification", call.Name))
			}
			if readErr != nil || writeErr != nil {
				if errors.Is(readErr, io.EOF) {
					readErr = nil
				}
				return errors.Join(runErr, readErr, writeErr)
			}
		}
	}
}
