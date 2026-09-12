package configuration

import "fmt"

// LoadPhase identifies the configuration-read boundary that failed.
type LoadPhase string

const (
	// LoadPhaseRead identifies a configuration file read failure.
	LoadPhaseRead LoadPhase = "read"
	// LoadPhaseParse identifies a TOML decoding failure.
	LoadPhaseParse LoadPhase = "parse"
	// LoadPhaseValidate identifies a typed configuration validation failure.
	LoadPhaseValidate LoadPhase = "validate"
)

// LoadError preserves the failed load phase and its underlying cause.
type LoadError struct {
	Phase LoadPhase
	Err   error
}

// Error formats the configuration load phase and underlying failure.
func (e *LoadError) Error() string {
	return fmt.Sprintf("%s config: %v", e.Phase, e.Err)
}

// Unwrap exposes the underlying configuration load failure for [errors.Is] and [errors.As].
func (e *LoadError) Unwrap() error { return e.Err }

func newLoadError(phase LoadPhase, err error) error {
	return &LoadError{Phase: phase, Err: err}
}
