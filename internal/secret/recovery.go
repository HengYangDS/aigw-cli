package secret

import (
	"errors"
	"fmt"

	"aigw-cli/internal/secret/native"
)

// TokenRecovery names the executable recovery owned by the selected secret
// backend and reports whether the backend can persist a replacement Token.
func TokenRecovery(store Store, account string) (instruction string, writable bool) {
	if IsReadOnly(store) {
		return "set environment variable " + EnvironmentKey(account), false
	}
	return "run `aigw provider rotate " + account + "`", true
}

// KeychainItemRefusedError is a macOS Keychain item that macOS refuses AIGW
// while its keychain is unlocked: another program stored the item, or its
// access list does not admit AIGW. It names the credential, carries no value
// and owns the fix.
type KeychainItemRefusedError struct {
	Kind     Kind
	Provider string
	// Status is the Keychain status with which macOS refused the item.
	Status int32
	// lastStep is the last step of the fix when the command that met the
	// refusal stores the credential itself, which AfterDeletionRun sets.
	lastStep string
}

// Error names the credential and the native status, without a value.
func (e *KeychainItemRefusedError) Error() string {
	return fmt.Sprintf("macOS refused AIGW provider %s's %s while its keychain was unlocked: %v", e.Provider, e.credential(), e.Unwrap())
}

// Unwrap keeps the native failure, with its status, behind the refusal.
func (e *KeychainItemRefusedError) Unwrap() error {
	return &native.KeychainError{Status: e.Status, Unlocked: true}
}

// Title says what happened, as a problem's title.
func (e *KeychainItemRefusedError) Title() string {
	return "Another program stored provider " + e.Provider + "'s " + e.credential() + " in the Keychain"
}

// Evidence says what macOS answered.
func (e *KeychainItemRefusedError) Evidence() string {
	return fmt.Sprintf("macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus %d).", e.Status)
}

// Fix is the one fix: have the credential at hand, delete the item with the
// system tool, then store the credential again, with its own command unless
// the command that met the refusal stores it. AIGW does not delete the item
// itself, because the other program may still use it. The credential comes
// first, because AIGW cannot read back the copy that the deletion removes.
func (e *KeychainItemRefusedError) Fix() string {
	last := e.lastStep
	if last == "" {
		last = "run `aigw provider rotate " + e.Provider + "`"
		if e.Kind == ProviderDiagnostic {
			last = "run `aigw provider edit " + e.Provider + " --balance-token-stdin --balance-user-id <user-id>`"
		}
	}
	return "Have the " + e.credential() + " at hand, delete the item with `security delete-generic-password -s " + Service +
		" -a " + slotName(e.Kind, e.Provider) + "`, then " + last + "."
}

// AfterDeletionRun makes command the last step of the fix of a refusal that
// err wraps, and returns err. It serves a command that stores the credential
// itself and adds its provider: until that command succeeds, the credential's
// own command does not know the provider. command is the command's plain
// line, or "" when the line would need quoting, which the fix then names as
// the same command.
func AfterDeletionRun(err error, command string) error {
	if refusal, refused := errors.AsType[*KeychainItemRefusedError](err); refused {
		refusal.lastStep = "run the same command again"
		if command != "" {
			refusal.lastStep = "run `" + command + "`"
		}
	}
	return err
}

// credential names the kind of credential that the item holds.
func (e *KeychainItemRefusedError) credential() string {
	if e.Kind == ProviderDiagnostic {
		return "balance credentials"
	}
	return "Token"
}

// refusedItem makes err, a failed read or write of one credential, the
// refusal of its item when macOS refused it while the keychain was unlocked.
// Any other failure, a locked keychain's among them, returns unchanged.
func refusedItem(kind Kind, provider string, err error) error {
	if failure, ok := errors.AsType[*native.KeychainError](err); ok && failure.ItemRefused() {
		return &KeychainItemRefusedError{Kind: kind, Provider: provider, Status: failure.Status}
	}
	return err
}
