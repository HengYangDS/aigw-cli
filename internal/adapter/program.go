package adapter

import (
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// ClientProgram is the program that a client uses, under the one rule that
// every operation follows: the configured program whenever one is configured,
// and the one that discovery finds only when none is. Convergence writes an
// adopted program into the configuration. AIGW never replaces a configured
// program, not even one that is gone: a plan stops the client instead.
func ClientProgram(binding domain.ClientBinding, discovered discovery.Result, clientID string) string {
	if binding.Executable != "" {
		return binding.Executable
	}
	return discovered.Executable(clientID)
}

// StopError is why a client stops: AIGW cannot produce what the client's
// files are to hold, so a synchronization writes none of its files, its
// model table or AIGW's record of them, and a check fails it, both with the
// same problem and fix. A stopped client's dropped files are still
// withdrawn, which needs no program. Its text names no path, which a local
// check never shows.
type StopError struct {
	client, problem, fix string
	cause                error
}

// NewStop returns the stop of client, with its problem, its one fix and the
// failure behind it, if there is one.
func NewStop(client, problem, fix string, cause error) *StopError {
	return &StopError{client: client, problem: problem, fix: fix, cause: cause}
}

// Error states the problem.
func (e *StopError) Error() string { return e.problem }

// Unwrap returns the failure behind the stop, if there is one.
func (e *StopError) Unwrap() error { return e.cause }

// Client returns the ID of the stopped client.
func (e *StopError) Client() string { return e.client }

// NextAction is the stop's one fix.
func (e *StopError) NextAction() string { return e.fix }

// MissingProgram returns the stop of the client's configured program when
// it is not runnable, enabled or not, and nil when the client configures none
// or its program runs. Its fix names another program: a sync cannot help,
// since AIGW keeps the configured one.
func MissingProgram(cfg domain.Config, clientID string) *StopError {
	configured := cfg.Clients[clientID].Executable
	if configured == "" {
		return nil
	}
	runnable, err := discovery.ExecutableAvailable(configured)
	if runnable && err == nil {
		return nil
	}
	problem := cfg.Admission().ProductName(clientID) + " executable is missing"
	if err != nil {
		problem = cfg.Admission().ProductName(clientID) + " executable cannot be inspected"
	}
	return NewStop(clientID, problem, "aigw client edit "+clientID+" --executable <path>", err)
}

// stoppedProgram is the stop of an enabled client whose configured program
// is missing, and nil for any other client: the one place where a plan and
// an inspection decide it.
func stoppedProgram(cfg domain.Config, clientID string) *StopError {
	if !cfg.Clients[clientID].Enabled {
		return nil
	}
	return MissingProgram(cfg, clientID)
}
