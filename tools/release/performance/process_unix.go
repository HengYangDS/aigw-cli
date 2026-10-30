//go:build !windows

package performance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
)

func preparationCommand(arguments []string) string { return Argv(arguments...) }

// ObserveCurrent leaves Windows-only execution evidence absent on Unix.
func ObserveCurrent() ([]Execution, error) { return nil, nil }

// ObserveCommand runs one untimed preflight, as Windows does: the preparation,
// then the measured command, each once. Their streams are retained beside the
// samples, and Measure judges the product's diagnostics from them, so the
// timed runs' output, Hyperfine's own advice included, remains evidence only.
// A diagnostic marks the workload unqualified without stopping the others.
// Unix records no Windows-only execution evidence.
func ObserveCommand(parent context.Context, input Command, workload Workload) ([]Execution, []Execution, error) {
	if len(workload.Command) == 0 || input.Output == "" {
		return nil, nil, errors.New("native preflight requires an explicit workload and output")
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
		return nil, nil, fmt.Errorf("native preflight: %s", domain.Redact(runErr.Error(), input.Sensitive...))
	}
	return nil, nil, nil
}
