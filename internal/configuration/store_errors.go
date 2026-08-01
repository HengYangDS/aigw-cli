package configuration

import "fmt"

type LoadPhase string

const (
	LoadPhaseRead     LoadPhase = "read"
	LoadPhaseParse    LoadPhase = "parse"
	LoadPhaseValidate LoadPhase = "validate"
)

type LoadError struct {
	Phase LoadPhase
	Err   error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("%s config: %v", e.Phase, e.Err)
}

func (e *LoadError) Unwrap() error { return e.Err }

func newLoadError(phase LoadPhase, err error) error {
	return &LoadError{Phase: phase, Err: err}
}
