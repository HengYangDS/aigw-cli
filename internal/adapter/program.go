package adapter

import (
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// Program is the program that a client uses, as ClientProgram resolves the
// client's declaration.
type Program struct {
	// Path is the program's file: the declared one, even when it is gone;
	// the one inside the highest installed version of the client's
	// extension; or, for a client that declares neither, the one that
	// discovery found. It is empty when an extensions directory resolves to
	// none and when discovery finds none.
	Path string
	// Version is the version of the client's extension that Path lies in, for
	// a client that declares an extensions directory.
	Version string
	// Discovered marks a client that declares neither, whose program, when
	// discovery found one, a converging command adopts as its declaration.
	Discovered bool
}

// ClientProgram is the program that a client uses, under the one rule that
// every operation follows: the program that the client declares, by its path
// or through its extensions directory, and only when it declares neither the
// one that discovery finds, which convergence then adopts. AIGW never
// replaces a declared program, not even one that is gone: the stop names it,
// and a plan stops the client instead. The stop is nil when the declared
// program runs and when the client declares neither. It runs no program and
// writes nothing.
func ClientProgram(cfg domain.Config, clientID string, discovered discovery.Result, host discovery.Host) (Program, *StopError) {
	binding := cfg.Clients[clientID]
	spec, _ := cfg.Admission().Spec(clientID)
	product := cfg.Admission().ProductName(clientID)
	switch {
	case binding.Program != "":
		return Program{Path: binding.Program}, missingFile(spec, product, binding.Program)
	case binding.Extensions != "":
		return resolveExtensions(spec, product, binding.Extensions, host)
	}
	return Program{Path: discovered.Executable(clientID), Discovered: true}, nil
}

// ResolveProgram is ClientProgram with an operation's dependencies: it
// discovers the client only when the client declares no program.
func ResolveProgram(deps Dependencies, cfg domain.Config, clientID string) (Program, *StopError) {
	var discovered discovery.Result
	if !cfg.Clients[clientID].DeclaresProgram() && deps.Discovery != nil {
		discovered = deps.Discovery.Discover(clientID)
	}
	return ClientProgram(cfg, clientID, discovered, deps.Host)
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

// ProgramRepair is the one fix of a client whose program is missing: a
// declaration of another, by its path or, for a client whose program an
// editor extension ships, by an extensions directory. A sync cannot help,
// since AIGW keeps the declared program.
func ProgramRepair(spec domain.ClientSpec) string {
	fix := "aigw client edit " + spec.ID + " --program <path>"
	if spec.Extension.ID != "" {
		fix += " or --extensions <dir>"
	}
	return fix
}

// MissingProgram returns the stop of the program that the client declares
// when it cannot be used, enabled or not, and nil when the client declares
// none or its program runs.
func MissingProgram(cfg domain.Config, clientID string, host discovery.Host) *StopError {
	_, stop := ClientProgram(cfg, clientID, discovery.Result{}, host)
	return stop
}

// ProgramNotFound is the stop of a client that declares no program and of
// which discovery found none, as a command that names the client to use it,
// such as enabling it, reports it.
func ProgramNotFound(cfg domain.Config, clientID string) *StopError {
	spec, _ := cfg.Admission().Spec(clientID)
	return NewStop(clientID, cfg.Admission().ProductName(clientID)+" program was not found", ProgramRepair(spec), nil)
}

// missingFile is the stop of a program file that cannot run, and nil for one
// that can.
func missingFile(spec domain.ClientSpec, product, path string) *StopError {
	runnable, err := discovery.ExecutableAvailable(path)
	if runnable && err == nil {
		return nil
	}
	problem := product + " program is missing"
	if err != nil {
		problem = product + " program cannot be inspected"
	}
	return NewStop(spec.ID, problem, ProgramRepair(spec), err)
}

// stoppedProgram is the stop of an enabled client whose declared program
// cannot be used, and nil for any other client: the one place where a plan
// and an inspection decide it.
func stoppedProgram(cfg domain.Config, clientID string, host discovery.Host) *StopError {
	if !cfg.Clients[clientID].Enabled {
		return nil
	}
	return MissingProgram(cfg, clientID, host)
}
