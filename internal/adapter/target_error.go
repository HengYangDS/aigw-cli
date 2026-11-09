package adapter

import (
	"errors"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// TargetError is the failure of one client configuration file, and whether
// the configuration after the change keeps that file, as opposed to one that
// the change drops. Path is the file as the operator configured it, never
// with its symbolic links resolved, and Client and Home, which the registry
// adds, are the client's label and the home directory against which a report
// names the file. Err says why without naming the file.
type TargetError struct {
	Client  string
	Home    string
	Path    string
	Desired bool
	Err     error
	// OperatorCreates, which the registry adds from the client's spec, says
	// that the operator creates the file and its folder, as AIGW does not
	// create a Codex home, unless the failure itself says who does.
	OperatorCreates bool
}

// Error names the file, when there is one, and why it failed.
func (e *TargetError) Error() string {
	if e.Path == "" {
		return e.Err.Error()
	}
	return e.Path + ": " + e.Err.Error()
}

// Unwrap returns the failure.
func (e *TargetError) Unwrap() error { return e.Err }

// namedTarget names, in err, the client and the home directory of a client
// file that spec's adapter failed on, as ApplyError does for a write, and who
// creates the client's files.
func namedTarget(spec domain.ClientSpec, home string, err error) error {
	if failed, atTarget := errors.AsType[*TargetError](err); atTarget && failed.Client == "" {
		failed.Client, failed.Home, failed.OperatorCreates = spec.Label, home, spec.OperatorCreatesTargets
	}
	return err
}

// CaptureTarget reads a client file that an adapter is about to plan. An
// operating-system error is the file's own, so the report names that file and
// what resolves it there, never AIGW's folder. kept says whether the
// configuration after the change keeps the file.
func CaptureTarget(path string, kept bool) (platform.FileSnapshot, error) {
	snapshot, err := platform.CaptureFileSnapshot(path)
	if err != nil {
		return platform.FileSnapshot{}, &TargetError{Path: path, Desired: kept, Err: err}
	}
	return snapshot, nil
}
