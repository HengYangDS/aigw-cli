//go:build darwin && cgo

package native

import (
	"errors"
	"testing"
)

func TestVersionedReaderReadsAPredecessorItemWithoutMigrating(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "shared", "synthetic-shared-token"
	addPredecessorItem(t, path, service, account, token)
	t.Setenv("AIGW_TEST_KEYCHAIN_ROLE", "reader")
	for range 2 {
		if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
			t.Fatalf("versioned reader read = %q, %v", value, err)
		}
		requirePredecessorItem(t, path, service, account, token)
	}
	t.Setenv("AIGW_TEST_KEYCHAIN_ROLE", "")
	if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
		t.Fatalf("installed program read = %q, %v", value, err)
	}
	requireWorkerOwnedItem(t, path, service, account)
}

func TestVersionedReaderLeavesAnInterruptedMigration(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "interrupted", "synthetic-staged-token"
	if err := writeCredentialToKeychain(service, stagedAccount(account), path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_TEST_KEYCHAIN_ROLE", "reader")
	if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
		t.Fatalf("versioned reader read = %q, %v", value, err)
	}
	if present, err := observeCredentialInKeychain(service, account, path); err != nil || present {
		t.Fatalf("versioned reader completed the migration: %t, %v", present, err)
	}
	if present, err := observeCredentialInKeychain(service, stagedAccount(account), path); err != nil || !present {
		t.Fatalf("versioned reader removed the staged Token: %t, %v", present, err)
	}
}

func TestVersionedReaderNeverConvertsOwnership(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "retained", "synthetic-retained-token"
	addPredecessorItem(t, path, service, account, token)
	if err := writeCredentialWithMigration(service, account, path, []byte("synthetic-rotated-token"), VersionedReader); err == nil {
		t.Fatal("versioned reader replaced a predecessor item")
	}
	requirePredecessorItem(t, path, service, account, token)
	for _, operation := range []string{restoreCommand, reclaimCommand} {
		if _, err := queryCredential(operation, service, account, nil, VersionedReader); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("versioned reader %s = %v", operation, err)
		}
	}
}

func TestReclaimReturnsARestoredItemToAIGW(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "reclaimed", "synthetic-reclaimed-token"
	addPredecessorItem(t, path, service, account, token)
	if err := reclaimCredentialFromPredecessor(service, account, path); err != nil {
		t.Fatalf("reclaim predecessor item: %v", err)
	}
	requireWorkerOwnedItem(t, path, service, account)
	if err := restoreCredentialForPredecessor(service, account, path); err != nil {
		t.Fatalf("restore before reclaim: %v", err)
	}
	if err := reclaimCredentialFromPredecessor(service, account, path); err != nil {
		t.Fatalf("reclaim restored item: %v", err)
	}
	requireWorkerOwnedItem(t, path, service, account)
	if value, err := readCredentialFromKeychain(service, account, path); err != nil || string(value) != token {
		t.Fatalf("reclaimed item = %q, %v", value, err)
	}
	if err := reclaimCredentialFromPredecessor(service, "absent", path); err != nil {
		t.Fatalf("reclaim absent slot: %v", err)
	}
}
