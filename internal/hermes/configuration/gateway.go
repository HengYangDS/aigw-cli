package configuration

import (
	"path/filepath"
	"time"
)

// GatewayLockDirectory is the verification home's own host lock directory,
// which keeps a verification gateway out of the operator's host rendezvous.
// Hermes creates it owner-only when a gateway first needs it.
func GatewayLockDirectory(home string) string { return filepath.Join(home, "gateway-locks") }

// AwaitGatewayAbsence checks GatewayAbsent every poll until the home's gateway
// is gone or the bound passes, and reports whether it is absent.
func AwaitGatewayAbsence(home string, bound, poll time.Duration) bool {
	deadline := time.Now().Add(bound)
	for {
		if absent, err := GatewayAbsent(home); err == nil && absent {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(poll)
	}
}
