package configuration

import "errors"

// CreateVerificationHome refuses an unsupported native launch before effects.
func CreateVerificationHome(_ string) (string, func() error, error) {
	return "", nil, errors.New("Hermes verification cannot isolate native launcher publication on Windows; no client process was started")
}
