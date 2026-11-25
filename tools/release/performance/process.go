package performance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
)

// reviewIdentity requires the complete identity of a selected file.
func reviewIdentity(selected *Identity) error {
	if selected == nil || selected.Path == "" || len(selected.SHA256) != 64 || selected.Bytes < 1 || selected.Arch == "" {
		return errors.New("performance execution identity is incomplete")
	}
	return nil
}

// ObserveCommand runs one untimed preflight: the preparation, then the
// measured command, each once. Their streams are retained beside the samples,
// and Measure judges the product's diagnostics from them, so the timed runs'
// output, Hyperfine's own advice included, remains evidence only. A diagnostic
// marks the workload unqualified without stopping the others.
func ObserveCommand(parent context.Context, input Command, workload Workload) error {
	if len(workload.Command) == 0 || input.Output == "" {
		return errors.New("native preflight requires an explicit workload and output")
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	var stdout, stderr []byte
	var runErr error
	for _, command := range [][]string{workload.Prepare, workload.Command} {
		if len(command) == 0 || runErr != nil {
			continue
		}
		out, diagnostics, err := (process.Runner{StdoutLimit: 4 << 20}).RunCaptureStreams(ctx, process.Plan{
			Executable: command[0], Args: command[1:], Env: input.Environment, Directory: input.Directory,
		})
		stdout, stderr, runErr = append(stdout, out...), append(stderr, diagnostics...), err
	}
	runErr = errors.Join(runErr, writePreflightStreams(input, stdout, stderr))
	if runErr != nil {
		return fmt.Errorf("native preflight: %s", domain.Redact(runErr.Error(), input.Sensitive...))
	}
	return nil
}
