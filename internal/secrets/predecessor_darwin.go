//go:build darwin

package secrets

import (
	"fmt"
	"strings"

	"aigw-cli/internal/secrets/native"

	"github.com/Masterminds/semver/v3"
)

// nativeOwnerRelease is the first release whose macOS worker owns the Keychain
// items it uses; earlier releases read every item through /usr/bin/security.
var nativeOwnerRelease = semver.MustParse("0.4.0")

// RestorePredecessorCredentials returns each Account's Token and diagnostic
// items to the predecessor's macOS reader before a rollback activates the
// program at version. A release that reads AIGW-owned items needs no
// conversion; an unknown version, as before a Homebrew downgrade, is converted.
// Only the native keyring authorizes the reading executable.
func RestorePredecessorCredentials(store Store, executable string, accounts []string, version string) error {
	if release, err := semver.StrictNewVersion(strings.TrimPrefix(version, "v")); err == nil && !release.LessThan(nativeOwnerRelease) {
		return nil
	}
	return convertKeychainSlots(store, accounts, func(account, slot string) error {
		if err := native.Restore(executable, Service, slot); err != nil {
			return fmt.Errorf("restore %s credential for the retained program: %w", account, err)
		}
		return nil
	})
}

// ReclaimPredecessorCredentials returns the items that a rollback gave to the
// predecessor's reader to AIGW ownership when that rollback is not activated.
func ReclaimPredecessorCredentials(store Store, executable string, accounts []string) error {
	return convertKeychainSlots(store, accounts, func(account, slot string) error {
		if err := native.Reclaim(executable, Service, slot); err != nil {
			return fmt.Errorf("reclaim %s credential for the active program: %w", account, err)
		}
		return nil
	})
}

// convertKeychainSlots visits each Account's Token and diagnostic slots when
// the native keyring is selected.
func convertKeychainSlots(store Store, accounts []string, convert func(account, slot string) error) error {
	keyring, err := selectedKeyring(store)
	if err != nil || !keyring {
		return err
	}
	for _, account := range accounts {
		for _, kind := range []Kind{APIToken, ProviderDiagnostic} {
			if err := convert(account, slotName(kind, account)); err != nil {
				return err
			}
		}
	}
	return nil
}
