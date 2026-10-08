//go:build !windows

package performance

import "context"

func preparationCommand(arguments []string) string { return Argv(arguments...) }

// ObserveCurrent leaves Windows-only execution evidence absent on Unix.
func ObserveCurrent() ([]Execution, error) { return nil, nil }

// ObserveCommand performs no extra Unix workload; native timing remains unchanged.
func ObserveCommand(context.Context, Command, Workload) ([]Execution, []Execution, error) {
	return nil, nil, nil
}
