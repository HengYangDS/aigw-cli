//go:build !windows

package performance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
)

func preparationCommand(arguments []string) string { return Argv(arguments...) }

// ObserveCurrent leaves Windows-only execution evidence absent on Unix.
func ObserveCurrent() ([]Execution, error) { return nil, nil }

// ObserveCommand runs one untimed preflight, as Windows does: the preparation,
// then the measured command, each once, and their diagnostics decide whether
// the workload may be timed. The timed runs' output, Hyperfine's own advice
// included, then remains evidence only. Unix records no Windows-only execution
// evidence.
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
	if process.DiagnosticFailure(stderr) {
		runErr = errors.Join(runErr, errors.New("native preflight emitted diagnostics"))
	}
	for name, stream := range map[string][]byte{"stdout": stdout, "stderr": stderr} {
		path := strings.TrimSuffix(input.Output, ".json") + ".preflight." + name
		runErr = errors.Join(runErr, os.WriteFile(path, []byte(redaction.Text(string(stream), input.Sensitive...)), 0o600))
	}
	if runErr != nil {
		return nil, nil, fmt.Errorf("native preflight: %s", redaction.Text(runErr.Error(), input.Sensitive...))
	}
	return nil, nil, nil
}
