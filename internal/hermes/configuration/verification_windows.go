//go:build windows

package configuration

import "errors"

// ErrNativeVerificationUnsupported preserves the operator's launcher when its
// platform has no qualified isolated publication boundary.
var ErrNativeVerificationUnsupported = errors.New("Hermes verification cannot isolate native launcher publication on Windows; no client process was started")

// CreateVerificationHome refuses an unqualified native launcher boundary.
func CreateVerificationHome(_ string) (string, func() error, error) {
	return "", nil, ErrNativeVerificationUnsupported
}
