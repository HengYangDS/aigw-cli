package hermes

import "errors"

// CreateVerificationHome refuses an unsupported native launch before effects.
func CreateVerificationHome(_ string) (string, func() error, error) {
	return "", nil, errors.New("Hermes verification cannot isolate native launcher publication on Windows; no client process was started")
}

// GatewayAbsent cannot observe a Hermes gateway on Windows, where no
// verification home or gateway is started.
func GatewayAbsent(string) (bool, error) {
	return false, errors.New("Hermes gateway locks are not observed on Windows")
}
