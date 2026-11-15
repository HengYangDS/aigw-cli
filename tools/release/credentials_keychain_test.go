package main

import (
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// storeKeychainItemOfAnotherProgram stores an item for account through the
// system tool, in keychain or, for "", in the default keychain. macOS then
// lets that tool alone use the item, as it does for a Token that a release up
// to 0.3.5 stored. The slot must be unoccupied, and the item is deleted with
// the test, also after an early exit.
func storeKeychainItemOfAnotherProgram(t *testing.T, account, keychain string) {
	t.Helper()
	if keychainTool(t, "find-generic-password", account, keychain) {
		t.Fatalf("native credential test requires an unoccupied %q slot", account)
	}
	t.Cleanup(func() {
		keychainTool(t, "delete-generic-password", account, keychain)
		if keychainTool(t, "find-generic-password", account, keychain) {
			t.Errorf("credential slot %q remains after cleanup", account)
		}
	})
	args := []string{"add-generic-password", "-s", secret.Service, "-a", account, "-w", "synthetic-item-of-another-program"}
	if keychain != "" {
		args = append(args, keychain)
	}
	runKeychainTool(t, args...)
}

// requireRefusedKeychainItemOfAnotherProgram requires that store, which runs
// the candidate's credential worker, neither reads nor replaces an item that
// another program stored for account: each answers with the refusal of that
// item, whose fix the commands then name. The item stays as it was until the
// tool that stored it deletes it.
func requireRefusedKeychainItemOfAnotherProgram(t *testing.T, store secret.Store, account, keychain string) {
	t.Helper()
	storeKeychainItemOfAnotherProgram(t, account, keychain)
	before := keychainItemAttributes(t, account, keychain)
	if _, err := store.Get(account); !keychainItemRefused(err) {
		t.Fatalf("the candidate's read of another program's Keychain item = %v", err)
	}
	if err := store.Set(account, "synthetic-replacement"); !keychainItemRefused(err) {
		t.Fatalf("the candidate's replacement of another program's Keychain item = %v", err)
	}
	if after := keychainItemAttributes(t, account, keychain); after != before {
		t.Fatalf("the refused Keychain item changed:\n%s\nwas:\n%s", after, before)
	}
	if !keychainTool(t, "delete-generic-password", account, keychain) || keychainTool(t, "find-generic-password", account, keychain) {
		t.Fatalf("the tool that stored the %q item did not delete it", account)
	}
	t.Logf("the candidate's store refused the read and the replacement of another program's Keychain item %q with OSStatus -25293 and left it unchanged; the system tool deleted it", account)
}

// keychainItemRefused reports that err is the refusal of a Keychain item that
// macOS refuses AIGW while its keychain is unlocked, with the status that
// macOS gives for an item of another program.
func keychainItemRefused(err error) bool {
	refusal, refused := errors.AsType[*secret.KeychainItemRefusedError](err)
	return refused && refusal.Status == -25293
}

// keychainFixtureTimeout bounds this file's own /usr/bin/security calls. It
// stops only a hang: a shared CI host can take longer than five seconds to
// create a keychain, as GitLab job 69467 did on 2026-11-13.
const keychainFixtureTimeout = 30 * time.Second

// keychainTool runs the system tool's find or delete of account's item, in
// keychain or, for "", in the default keychain, and reports whether the tool
// found the item.
func keychainTool(t *testing.T, operation, account, keychain string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), keychainFixtureTimeout)
	defer cancel()
	args := []string{operation, "-s", secret.Service, "-a", account}
	if keychain != "" {
		args = append(args, keychain)
	}
	_, err := exec.CommandContext(ctx, "/usr/bin/security", args...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Keychain %s timed out for %q: %v", operation, account, ctx.Err())
	}
	if err == nil {
		return true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 44 {
		return false
	}
	t.Fatalf("Keychain %s failed for %q: %v", operation, account, err)
	return false
}

// keychainItemAttributes returns the attributes of account's item, which
// include its modification time, as the system tool prints them without the
// item's value.
func keychainItemAttributes(t *testing.T, account, keychain string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), keychainFixtureTimeout)
	defer cancel()
	args := []string{"find-generic-password", "-s", secret.Service, "-a", account}
	if keychain != "" {
		args = append(args, keychain)
	}
	output, err := exec.CommandContext(ctx, "/usr/bin/security", args...).Output()
	if err != nil {
		t.Fatalf("read the attributes of the %q Keychain item: %v", account, err)
	}
	return string(output)
}

// refusingStore answers a read and a write as the candidate's store answers
// for a Keychain item that macOS refuses AIGW, and counts them.
type refusingStore struct {
	secret.Store
	reads, writes int
}

func (store *refusingStore) refusal(account string) error {
	return fmt.Errorf("read %s/%s from system keyring: %w", secret.Service, account,
		&secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: account, Status: -25293})
}

func (store *refusingStore) Get(account string) (string, error) {
	store.reads++
	return "", store.refusal(account)
}

func (store *refusingStore) Set(account, _ string) error {
	store.writes++
	return store.refusal(account)
}

// A test that stores a Keychain item through the system tool owns that item:
// it is gone after the test, also after an early exit. The journey's step
// stores one, requires the refusal of a read and of a replacement, finds the
// item unchanged and deletes it with the tool. The keychain is the test's own.
func TestKeychainItemOfAnotherProgramIsRefusedAndDeletedWithItsTest(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the system tool stores Keychain items only on macOS")
	}
	keychain := filepath.Join(t.TempDir(), "private.keychain-db")
	runKeychainTool(t, "create-keychain", "-p", "", keychain)
	t.Cleanup(func() { runKeychainTool(t, "delete-keychain", keychain) })
	runKeychainTool(t, "unlock-keychain", "-p", "", keychain)
	const account = "item-of-another-program"
	t.Run("early exit", func(t *testing.T) {
		storeKeychainItemOfAnotherProgram(t, account, keychain)
		if !keychainTool(t, "find-generic-password", account, keychain) {
			t.Fatal("the system tool stored no item")
		}
		t.SkipNow()
	})
	if keychainTool(t, "find-generic-password", account, keychain) {
		t.Fatal("early exit left the item of another program behind")
	}
	store := &refusingStore{}
	requireRefusedKeychainItemOfAnotherProgram(t, store, account, keychain)
	if store.reads != 1 || store.writes != 1 || keychainTool(t, "find-generic-password", account, keychain) {
		t.Fatalf("the step read %d times, wrote %d times, or left its item behind", store.reads, store.writes)
	}
}

// Only the refusal of an item in an unlocked keychain, with the status that
// macOS gives for it, is the answer that the journey's step requires: a locked
// keychain, a missing item, another failure and a success are not.
func TestKeychainRefusalIsToldFromEveryOtherAnswer(t *testing.T) {
	refusal := &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: "probe", Status: -25293}
	for name, test := range map[string]struct {
		err  error
		want bool
	}{
		"the refusal":        {fmt.Errorf("write AIGW_TOKEN/probe to system keyring: %w", refusal), true},
		"another status":     {&secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: "probe", Status: -25308}, false},
		"a locked keychain":  {&native.KeychainError{Status: -25293}, false},
		"a missing item":     {secret.ErrNotFound, false},
		"a success":          {nil, false},
		"an unrelated error": {errors.New("backend failed"), false},
	} {
		if got := keychainItemRefused(test.err); got != test.want {
			t.Errorf("%s: refused = %t, want %t", name, got, test.want)
		}
	}
}

// runKeychainTool runs one command of the system tool that must succeed.
func runKeychainTool(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), keychainFixtureTimeout)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "/usr/bin/security", args...).CombinedOutput(); err != nil {
		t.Fatalf("Keychain fixture command %q failed: %v; %s", args[0], err, output)
	}
}
