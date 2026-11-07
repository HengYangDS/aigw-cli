package adapter

// TargetError is the failure of one client configuration file, and whether
// the configuration after the change keeps that file, as opposed to one that
// the change drops. Its text is the failure's own.
type TargetError struct {
	Desired bool
	Err     error
}

// Error returns the failure's text.
func (e *TargetError) Error() string { return e.Err.Error() }

// Unwrap returns the failure.
func (e *TargetError) Unwrap() error { return e.Err }
