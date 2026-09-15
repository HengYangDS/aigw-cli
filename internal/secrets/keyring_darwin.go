//go:build darwin

package secrets

import "aigw-cli/internal/secrets/keychain"

func observeKeyringItem(service, slot string) (bool, error) {
	return keychain.Exists(service, slot)
}
