package adapter

import (
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// ClientProgram is the program that a client uses, under the one rule that
// every operation follows: the configured program whenever one is configured,
// and the one that discovery finds only when none is. Convergence writes an
// adopted program into the configuration. AIGW never replaces a configured
// program, not even one that is gone: MissingProgram reports that.
func ClientProgram(binding domain.ClientBinding, discovered discovery.Result, clientID string) string {
	if binding.Executable != "" {
		return binding.Executable
	}
	return discovered.Executable(clientID)
}

// MissingProgramError reports a client whose configured program is not
// runnable. It stops an enabled client: synchronization writes none of its
// files or AIGW's record of them, and a check fails it, both with the one fix
// that names another program. Its text names no path, which a local check
// never shows.
type MissingProgramError struct {
	client, product string
	cause           error
}

// Error states the missing program for the client's product.
func (e *MissingProgramError) Error() string {
	if e.cause != nil {
		return e.product + " executable cannot be inspected"
	}
	return e.product + " executable is missing"
}

// Unwrap returns the failure that kept the program from being inspected, if
// one did.
func (e *MissingProgramError) Unwrap() error { return e.cause }

// Client returns the ID of the client whose program is missing.
func (e *MissingProgramError) Client() string { return e.client }

// NextAction is the one fix: configure a program that runs. A sync cannot
// help, since AIGW keeps the configured program.
func (e *MissingProgramError) NextAction() string {
	return "aigw client edit " + e.client + " --executable <path>"
}

// MissingProgram returns the refusal of the client's configured program when
// it is not runnable, enabled or not, and nil when the client configures none
// or its program runs.
func MissingProgram(cfg domain.Config, clientID string) *MissingProgramError {
	configured := cfg.Clients[clientID].Executable
	if configured == "" {
		return nil
	}
	runnable, err := discovery.ExecutableAvailable(configured)
	if runnable && err == nil {
		return nil
	}
	return &MissingProgramError{client: clientID, product: cfg.Admission().ProductName(clientID), cause: err}
}

// stopped returns the refusal of an enabled client whose configured program
// is missing, which stops it, and nil for any other client.
func stopped(cfg domain.Config, clientID string) *MissingProgramError {
	if !cfg.Clients[clientID].Enabled {
		return nil
	}
	return MissingProgram(cfg, clientID)
}

// MissingPrograms returns, in admission order, the refusal of each enabled
// client whose configured program is missing: the clients that a
// synchronization of cfg stops.
func (registry Registry) MissingPrograms(cfg domain.Config) []*MissingProgramError {
	var missing []*MissingProgramError
	for _, id := range registry.ids {
		if refusal := stopped(cfg, id); refusal != nil {
			missing = append(missing, refusal)
		}
	}
	return missing
}
