//go:build darwin && cgo

package native

import (
	"encoding/base64"
	"errors"
	"slices"
	"testing"
)

// macOS refuses with one status an item that this program may not use and
// every item of a locked keychain, and the worker tells them apart by the lock
// state of the keychain that holds the item. With two keychains in the search
// list that is not the first one: an item in a locked second keychain is a
// locked keychain's, whatever the first is, and another program's item in an
// unlocked second keychain is that item's refusal. A slot that holds no item
// takes the state of the first keychain, which would receive it.
func TestALockStateIsThatOfTheKeychainThatHoldsTheItem(t *testing.T) {
	const service, account, token = "aigw-private-test", "listed", "synthetic-listed-token"
	foreign := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))
	for _, test := range []struct {
		name string
		// store puts the slot's item, if any, into the second keychain.
		store func(t *testing.T, second string)
		// lockFirst locks the first keychain, and the second otherwise.
		lockFirst bool
		want      KeychainError
		// absent says that the slot holds no item, which a read then reports.
		absent bool
	}{
		{name: "this program's item in a locked second keychain", want: KeychainError{Status: -25293},
			store: func(t *testing.T, second string) {
				if err := writeCredentialToKeychain(service, account, second, []byte(token)); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "another program's item in an unlocked second keychain", lockFirst: true, want: KeychainError{Status: -25293, Unlocked: true},
			store: func(t *testing.T, second string) {
				runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", account, "-w", foreign, second)
			}},
		{name: "no item and a locked first keychain", lockFirst: true, want: KeychainError{Status: -25293}, absent: true,
			store: func(*testing.T, string) {}},
	} {
		t.Run(test.name, func(t *testing.T) {
			first, second := newPrivateKeychain(t), newPrivateKeychain(t)
			test.store(t, second)
			locked := second
			if test.lockFirst {
				locked = first
			}
			runKeychainFixtureCommand(t, "lock-keychain", locked)
			list := first + ":" + second

			value, err := readCredentialFromKeychain(service, account, list)
			if test.absent {
				if len(value) != 0 || !errors.Is(err, ErrNotFound) {
					t.Errorf("read of the absent slot = %q, %v", value, err)
				}
			} else if failure := keychainFailure(err); len(value) != 0 || failure != test.want {
				t.Errorf("read through the list = %q, %v, %+v; want %+v", value, err, failure, test.want)
			}
			err = writeCredentialToKeychain(service, account, list, []byte("synthetic-replacement"))
			if failure := keychainFailure(err); failure != test.want {
				t.Errorf("write through the list = %v, %+v; want %+v", err, failure, test.want)
			}

			runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", locked)
			if names := listedKeychainAccounts(t, service, first); len(names) != 0 {
				t.Errorf("items of the first keychain after the refusals = %q", names)
			}
			holds := []string{account}
			if test.absent {
				holds = nil
			}
			if names := listedKeychainAccounts(t, service, second); !slices.Equal(names, holds) {
				t.Errorf("items of the second keychain after the refusals = %q, want %q", names, holds)
			}
		})
	}
}

// A keychain that was locked when an operation began may have refused it for
// that reason alone. When another process unlocks the keychain before the
// worker reads the lock state again, the refusal therefore stays a locked
// keychain's, and never becomes one that tells the operator to delete the
// item. The worker does not run the refused operation again.
func TestARefusalOfAKeychainThatWasLockedStaysALockedKeychains(t *testing.T) {
	const service, account, token = "aigw-private-test", "own", "synthetic-own-token"
	for _, operation := range []string{"read", "write"} {
		t.Run(operation, func(t *testing.T) {
			path := newPrivateKeychain(t)
			if err := writeCredentialToKeychain(service, account, path, []byte(token)); err != nil {
				t.Fatal(err)
			}
			before := keychainItemAttributes(t, path, service, account)
			runKeychainFixtureCommand(t, "lock-keychain", path)
			reached := interleaveKeychainSteps(t, func(point string, _ int) {
				if point == "refused" {
					runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
				}
			})

			var err error
			lookups := 0
			if operation == "write" {
				err = writeCredentialToKeychain(service, account, path, []byte("synthetic-replacement"))
				lookups = 1
			} else {
				_, err = readCredentialFromKeychain(service, account, path)
			}
			if failure := keychainFailure(err); failure != (KeychainError{Status: -25293}) || failure.ItemRefused() {
				t.Errorf("the operation on a keychain that was locked = %v, %+v", err, failure)
			}
			if refusals, looked := reached("refused"), reached("looked-up"); refusals != 1 || looked != lookups {
				t.Errorf("the worker classified %d refusals after %d lookups, want 1 after %d", refusals, looked, lookups)
			}

			if after := keychainItemAttributes(t, path, service, account); after != before {
				t.Errorf("the refused item changed:\n%s\nwas:\n%s", after, before)
			}
			if value, err := readCredentialFromKeychain(service, account, path); err != nil || string(value) != token {
				t.Errorf("the item after the keychain was unlocked = %q, %v", value, err)
			}
		})
	}
}

// A keychain that another process locks while a write runs refuses that write.
// It was unlocked when the write began, and the refusal is still a locked
// keychain's: the worker reads the lock state again after the refusal.
func TestARefusalOfAKeychainThatIsLockedMeanwhileIsALockedKeychains(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "own", "synthetic-own-token"
	if err := writeCredentialToKeychain(service, account, path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	reached := interleaveKeychainSteps(t, func(point string, _ int) {
		if point == "looked-up" {
			runKeychainFixtureCommand(t, "lock-keychain", path)
		}
	})
	err := writeCredentialToKeychain(service, account, path, []byte("synthetic-replacement"))
	if failure := keychainFailure(err); failure != (KeychainError{Status: -25293}) || reached("looked-up") != 1 || reached("refused") != 1 {
		t.Fatalf("write to a keychain that was locked meanwhile = %v, %+v", err, failure)
	}
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	if value, err := readCredentialFromKeychain(service, account, path); err != nil || string(value) != token {
		t.Fatalf("the item after the keychain was unlocked = %q, %v", value, err)
	}
}

// A keychain name that is not an absolute path is refused before any Keychain
// call, by every operation: macOS would resolve it in the user's own keychain
// folder, which no test may touch.
func TestARelativeKeychainNameIsRefusedByEveryOperation(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "own"
	if err := writeCredentialToKeychain(service, account, path, []byte("synthetic-own-token")); err != nil {
		t.Fatal(err)
	}
	for _, list := range []string{"relative.keychain-db", path + ":relative.keychain-db", path + ":", ":" + path} {
		_, readErr := readCredentialFromKeychain(service, account, list)
		_, observeErr := observeCredentialInKeychain(service, account, list)
		_, listErr := listAccountsInKeychain(service, list)
		for operation, err := range map[string]error{
			"read": readErr, "observe": observeErr, "list": listErr,
			"write":  writeCredentialToKeychain(service, account, list, []byte("synthetic-replacement")),
			"delete": deleteCredentialFromKeychain(service, account, list),
		} {
			if failure := keychainFailure(err); failure != (KeychainError{Status: -50}) {
				t.Errorf("%s through %q = %v", operation, list, err)
			}
		}
	}
	if value, err := readCredentialFromKeychain(service, account, path); err != nil || string(value) != "synthetic-own-token" {
		t.Fatalf("the item after the refused names = %q, %v", value, err)
	}
}

// keychainFailure returns the typed failure of an operation that this process
// ran, or none.
func keychainFailure(err error) KeychainError {
	if failure, typed := errors.AsType[*KeychainError](err); typed {
		return *failure
	}
	return KeychainError{}
}
