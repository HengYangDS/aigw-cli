package construction

import (
	"aigw-cli/internal/process"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type toolCall struct {
	Name, Directory string
	Args, Env       []string
	Stdout, Stderr  io.Writer
	Timeout         time.Duration
	// Image, when set, receives the executable file that the call runs; the
	// call then runs that resolved file rather than resolving its name again.
	Image *toolImage
}

// toolImage identifies an executable file by its path and SHA-256.
type toolImage struct {
	Path, SHA256 string
}

type toolRunner func(toolCall) error

func executeTool(ctx context.Context) toolRunner {
	return func(call toolCall) (result error) {
		executable := call.Name
		if call.Image != nil {
			image, err := resolveImage(call.Name)
			if err != nil {
				return err
			}
			*call.Image, executable = image, image.Path
		}
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
			Executable: executable, Directory: call.Directory,
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

// resolveImage finds the executable file that a tool name runs and digests it.
func resolveImage(name string) (toolImage, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return toolImage{}, fmt.Errorf("resolve %s: %w", name, err)
	}
	file, err := os.Open(path)
	if err != nil {
		return toolImage{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return toolImage{}, fmt.Errorf("read %s: %w", path, err)
	}
	return toolImage{Path: path, SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}
