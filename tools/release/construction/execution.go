package construction

import (
	"aigw-cli/internal/platform"
	"aigw-cli/internal/platform/process"
	"context"
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
		diagnostics, err := (process.Runner{}).RunDiagnosed(callContext, process.Plan{
			Executable: executable, Directory: call.Directory,
			Args: call.Args, Env: append(os.Environ(), call.Env...),
		}, stdoutWriter, stderr)
		if diagnostics.Failure {
			err = errors.Join(err, fmt.Errorf("%s diagnostics prevent qualification", call.Name))
		}
		return err
	}
}

// resolveImage finds the executable file that a tool name runs and digests it.
func resolveImage(name string) (toolImage, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return toolImage{}, fmt.Errorf("resolve %s: %w", name, err)
	}
	digest, err := platform.FileSHA256(path)
	if err != nil {
		return toolImage{}, fmt.Errorf("read %s: %w", path, err)
	}
	return toolImage{Path: path, SHA256: digest}, nil
}
