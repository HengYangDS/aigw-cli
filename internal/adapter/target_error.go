package adapter

import "errors"

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
// file that client's adapter failed on, as ApplyError does for a write.
func namedTarget(client ClientLabel, home string, err error) error {
	if failed, atTarget := errors.AsType[*TargetError](err); atTarget && failed.Client == "" {
		failed.Client, failed.Home = string(client), home
	}
	return err
}
