package diagnostic

import (
	"errors"
	"fmt"
)

// StatusError is a platform API response whose status is not a success.
// Its message is the status; a probe adds which API answered and what it
// said.
type StatusError struct {
	Status int
}

// Error names the status.
func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d", e.Status) }

// UnreachableError is a platform API request that received no response, such
// as one that the network or the probe's deadline stopped.
type UnreachableError struct {
	Err error
}

// Error says that the request received no response, and why.
func (e *UnreachableError) Error() string { return "no response: " + e.Err.Error() }

// Unwrap returns the transport failure.
func (e *UnreachableError) Unwrap() error { return e.Err }

// RefusedError is a platform API that answered a query and refused it, which
// the balance credentials did not grant. Text is the probe's redacted account
// of the refusal.
type RefusedError struct {
	Text string
}

// Error is the probe's account of the refusal.
func (e *RefusedError) Error() string { return e.Text }

// ErrTokenNotListed reports that the provider's account lists no Token that
// matches the one that AIGW holds for the provider.
var ErrTokenNotListed = errors.New("current API Token was not found")
