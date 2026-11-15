//go:build darwin && cgo

package native

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// interleaveKeychainSteps runs act at every step of this process's Keychain
// operations until the test ends, where a test lets a child act as another
// writer of the slot would. act receives the step's point and how often the
// operations reached that point before. The returned function counts how
// often they reached a point.
func interleaveKeychainSteps(t *testing.T, act func(point string, earlier int)) func(point string) int {
	t.Helper()
	reached := map[string]int{}
	product := keychainStep
	t.Cleanup(func() { keychainStep = product })
	keychainStep = func(point string) {
		reached[point]++
		act(point, reached[point]-1)
	}
	return func(point string) int { return reached[point] }
}

// A write or a delete acts on the item that its lookup found. When another
// writer changes the slot between that lookup and the operation, macOS refuses
// the operation through the reference that the lookup returned, and the worker
// looks the slot up again: the operation then acts on the slot as it is. The
// other writer is a child of this binary, which acts after the first lookup.
func TestAMutationLooksItsSlotUpAgainAfterAnotherWriterChangedIt(t *testing.T) {
	const service, account, neighbor = "aigw-private-test", "contended", "neighbor"
	const ours, theirs = "synthetic-our-value", "synthetic-other-writer"
	for _, test := range []struct {
		name string
		// stored says that the slot holds an item before the operation.
		stored bool
		// other is what the other writer does after the first lookup.
		other func(t *testing.T, path string)
		// remove selects a deletion; the operation is a write otherwise.
		remove  bool
		wantErr error
		// kept says that the slot holds our value afterwards, and no item
		// otherwise.
		kept bool
	}{
		{name: "a replacement after another writer's replacement", stored: true, kept: true,
			other: func(t *testing.T, path string) { otherWriterStores(t, service, account, path, theirs) }},
		{name: "a deletion after another writer's replacement", stored: true, remove: true,
			other: func(t *testing.T, path string) { otherWriterStores(t, service, account, path, theirs) }},
		{name: "a creation after another writer's creation", kept: true,
			other: func(t *testing.T, path string) { otherWriterStores(t, service, account, path, theirs) }},
		{name: "a replacement after another writer's deletion and creation", stored: true, kept: true,
			other: func(t *testing.T, path string) {
				otherWriterDeletes(t, service, account, path)
				otherWriterStores(t, service, account, path, theirs)
			}},
		{name: "a deletion after another writer's deletion", stored: true, remove: true, wantErr: ErrNotFound,
			other: func(t *testing.T, path string) { otherWriterDeletes(t, service, account, path) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := newPrivateKeychain(t)
			if err := writeCredentialToKeychain(service, neighbor, path, []byte("synthetic-neighbor-token")); err != nil {
				t.Fatal(err)
			}
			if test.stored {
				if err := writeCredentialToKeychain(service, account, path, []byte("synthetic-original")); err != nil {
					t.Fatal(err)
				}
			}
			untouched := keychainItemAttributes(t, path, service, neighbor)
			reached := interleaveKeychainSteps(t, func(point string, earlier int) {
				if point == "looked-up" && earlier == 0 {
					test.other(t, path)
				}
			})

			var err error
			if test.remove {
				err = deleteCredentialFromKeychain(service, account, path)
			} else {
				err = writeCredentialToKeychain(service, account, path, []byte(ours))
			}
			if lookups := reached("looked-up"); !errors.Is(err, test.wantErr) || lookups != 2 {
				t.Errorf("the operation = %v after %d lookups, want %v after 2", err, lookups, test.wantErr)
			}

			value, err := readCredentialFromKeychain(service, account, path)
			remaining := []string{neighbor}
			if test.kept {
				remaining = []string{account, neighbor}
				if err != nil || string(value) != ours {
					t.Errorf("the slot = %q, %v; want this writer's value", value, err)
				}
				if attributes := keychainItemAttributes(t, path, service, account); !strings.Contains(attributes, `"`+keychainItemLabel(account)+`"`) {
					t.Errorf("the slot's item lost its label:\n%s", attributes)
				}
			} else if len(value) != 0 || !errors.Is(err, ErrNotFound) {
				t.Errorf("the slot after its deletion = %q, %v", value, err)
			}
			if names := listedKeychainAccounts(t, service, path); !slices.Equal(names, remaining) {
				t.Errorf("items after the operation = %q, want %q", names, remaining)
			}
			if after := keychainItemAttributes(t, path, service, neighbor); after != untouched {
				t.Errorf("the neighboring item changed:\n%s\nwas:\n%s", after, untouched)
			}
		})
	}
}

// A slot that another writer replaces at every lookup ends the operation after
// its last lookup. The status of that refusal is the one that macOS gives for
// a missing item, and the worker reports it as a failure: it found an item at
// every lookup, and the other writer's last value stays.
func TestAMutationStopsAfterItsLastLookupOfASlotThatKeepsChanging(t *testing.T) {
	const service, account = "aigw-private-test", "contended"
	for _, operation := range []string{"write", "delete"} {
		t.Run(operation, func(t *testing.T) {
			path := newPrivateKeychain(t)
			if err := writeCredentialToKeychain(service, account, path, []byte("synthetic-original")); err != nil {
				t.Fatal(err)
			}
			last := ""
			reached := interleaveKeychainSteps(t, func(point string, earlier int) {
				if point == "looked-up" {
					last = "synthetic-other-writer-" + strconv.Itoa(earlier+1)
					otherWriterStores(t, service, account, path, last)
				}
			})
			var err error
			if operation == "delete" {
				err = deleteCredentialFromKeychain(service, account, path)
			} else {
				err = writeCredentialToKeychain(service, account, path, []byte("synthetic-our-value"))
			}
			if lookups := reached("looked-up"); keychainFailure(err) != (KeychainError{Status: -25300}) || errors.Is(err, ErrNotFound) || lookups != 5 {
				t.Errorf("the operation = %v after %d lookups, want OSStatus -25300 after 5", err, lookups)
			}
			if value, err := readCredentialFromKeychain(service, account, path); err != nil || string(value) != last || last != "synthetic-other-writer-5" {
				t.Errorf("the slot = %q, %v; want the other writer's last value %q", value, err, last)
			}
			if names := listedKeychainAccounts(t, service, path); !slices.Equal(names, []string{account}) {
				t.Errorf("items after the operation = %q", names)
			}
			if attributes := keychainItemAttributes(t, path, service, account); !strings.Contains(attributes, `"`+keychainItemLabel(account)+`"`) {
				t.Errorf("the slot's item lost its label:\n%s", attributes)
			}
		})
	}
}

// Three workers that replace one slot's value at once collide: macOS refuses
// each writer whose item another has just replaced. Such a writer looks the
// slot up again, so all three succeed, and the slot holds, in its one labelled
// item, the value of one of them.
func TestConcurrentWritersOfAnExistingSlotAllSucceed(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "contended"
	if err := writeCredentialToKeychain(service, account, path, []byte("synthetic-original")); err != nil {
		t.Fatal(err)
	}
	values := []string{"synthetic-first-writer", "synthetic-second-writer", "synthetic-third-writer"}
	writers := make([]*gatedKeychainWriter, len(values))
	for index, value := range values {
		writers[index] = startGatedKeychainWriter(t, service, account, path, value)
	}
	for _, writer := range writers {
		writer.release()
	}
	for index, writer := range writers {
		if err := writer.wait(); err != nil {
			t.Errorf("writer %d failed with OSStatus %d: %v", index, fixtureKeychainRefusal(err).Status, err)
		}
	}
	value, err := readCredentialFromKeychain(service, account, path)
	if err != nil || !slices.Contains(values, string(value)) {
		t.Fatalf("contended slot = %q, %v; want one of %q", value, err, values)
	}
	t.Logf("the slot holds the value of writer %d", slices.Index(values, string(value)))
	if names := listedKeychainAccounts(t, service, path); !slices.Equal(names, []string{account}) {
		t.Fatalf("items after the contended write = %q", names)
	}
	if attributes := keychainItemAttributes(t, path, service, account); !strings.Contains(attributes, `"`+keychainItemLabel(account)+`"`) {
		t.Fatalf("the contended item lost its label:\n%s", attributes)
	}
}

// otherWriterStores stores value in the slot as another process of this
// binary, which macOS admits to the items that this binary wrote.
func otherWriterStores(t *testing.T, service, account, path, value string) {
	t.Helper()
	if err := writePrivateKeychainFixture(t, service, account, path, value); err != nil {
		t.Errorf("the other writer's write failed with OSStatus %d: %v", fixtureKeychainRefusal(err).Status, err)
	}
}

// otherWriterDeletes deletes the slot's item as another process of this binary.
func otherWriterDeletes(t *testing.T, service, account, path string) {
	t.Helper()
	if err := deletePrivateKeychainFixture(t, service, account, path); err != nil {
		t.Errorf("the other writer's deletion failed with OSStatus %d: %v", fixtureKeychainRefusal(err).Status, err)
	}
}
