//go:build darwin

package secrets

import (
	"fmt"

	"aigw-cli/internal/secrets/native"
)

// RestorePredecessorCredentials returns each Account's Token and diagnostic
// items to the predecessor's macOS reader before a portable rollback activates
// that program. Only the native keyring authorizes the reading executable.
func RestorePredecessorCredentials(store Store, executable string, accounts []string) error {
	keyring, err := selectedKeyring(store)
	if err != nil || !keyring {
		return err
	}
	for _, account := range accounts {
		for _, kind := range []Kind{APIToken, ProviderDiagnostic} {
			if err := native.Restore(executable, Service, slotName(kind, account)); err != nil {
				return fmt.Errorf("restore %s credential for the retained program: %w", account, err)
			}
		}
	}
	return nil
}
