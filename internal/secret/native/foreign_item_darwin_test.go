//go:build darwin && cgo

package native

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// An item that another program stored, as releases up to 0.3.5 did through
// /usr/bin/security, is refused and stays as it was: the worker reads and
// replaces nothing, adds no item beside it, and the program that stored the
// item still reads it. The refusal is errSecAuthFailed, and its diagnostic
// says that the keychain was unlocked, which a locked keychain's does not.
func TestAnotherProgramsItemStaysDeniedAndUnchanged(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, neighbor = "aigw-private-test", "foreign", "neighbor"
	stored := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte("synthetic-foreign-token"))
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", account, "-w", stored, path)
	if err := writeCredentialToKeychain(service, neighbor, path, []byte("synthetic-neighbor-token")); err != nil {
		t.Fatal(err)
	}
	before := keychainItemAttributes(t, path, service, account)

	value, err := readPrivateKeychainFixture(t, service, account, path)
	if refusal := fixtureKeychainRefusal(err); len(value) != 0 || fixtureExitCode(err) != failureExit || refusal != (KeychainError{Status: -25293, Unlocked: true}) || !refusal.ItemRefused() {
		t.Fatalf("read of another program's item = %q, %v, %+v", value, err, refusal)
	}
	err = writePrivateKeychainFixture(t, service, account, path, "synthetic-replacement")
	if refusal := fixtureKeychainRefusal(err); fixtureExitCode(err) != failureExit || refusal != (KeychainError{Status: -25293, Unlocked: true}) || !refusal.ItemRefused() {
		t.Fatalf("replacement of another program's item = %v, %+v", err, refusal)
	}

	if after := keychainItemAttributes(t, path, service, account); after != before {
		t.Fatalf("another program's item changed:\n%s\nwas:\n%s", after, before)
	}
	if got := storedKeychainValue(t, path, service, account); got != stored {
		t.Fatal("the program that stored the item no longer reads its value")
	}
	if value, err := readCredentialFromKeychain(service, neighbor, path); err != nil || string(value) != "synthetic-neighbor-token" {
		t.Fatalf("neighboring item = %q, %v", value, err)
	}
	if names := listedKeychainAccounts(t, service, path); !slices.Equal(names, []string{account, neighbor}) {
		t.Fatalf("items after the refusals = %q", names)
	}
}

// macOS lets this program delete an item that it may not read, in an unlocked
// and in a locked keychain alike. The deletion removes that item and no other,
// and a repeated one finds nothing.
func TestDeletingAnotherProgramsItemRemovesIt(t *testing.T) {
	for _, locked := range []bool{false, true} {
		path := newPrivateKeychain(t)
		const service, account, neighbor = "aigw-private-test", "foreign", "neighbor"
		stored := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte("synthetic-foreign-token"))
		runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", account, "-w", stored, path)
		if err := writeCredentialToKeychain(service, neighbor, path, []byte("synthetic-neighbor-token")); err != nil {
			t.Fatal(err)
		}
		if locked {
			runKeychainFixtureCommand(t, "lock-keychain", path)
		}
		if err := deletePrivateKeychainFixture(t, service, account, path); err != nil {
			t.Fatalf("locked %t: deletion of another program's item = %v, %+v", locked, err, fixtureKeychainRefusal(err))
		}
		if err := deletePrivateKeychainFixture(t, service, account, path); fixtureExitCode(err) != missingExit {
			t.Fatalf("locked %t: repeated deletion = %v", locked, err)
		}
		runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
		if names := listedKeychainAccounts(t, service, path); !slices.Equal(names, []string{neighbor}) {
			t.Fatalf("locked %t: items after the deletion = %q", locked, names)
		}
		if value, err := readCredentialFromKeychain(service, neighbor, path); err != nil || string(value) != "synthetic-neighbor-token" {
			t.Fatalf("locked %t: neighboring item = %q, %v", locked, value, err)
		}
	}
}

// A locked keychain refuses a read and a write of every item with
// errSecAuthFailed, without waiting for a person, and its diagnostic does not
// say that the keychain was unlocked. It holds the same items once it is.
func TestLockedKeychainRefusesReadsAndWritesAndKeepsItsItems(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, own, foreign, token = "aigw-private-test", "own", "foreign", "synthetic-locked-token"
	stored := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", foreign, "-w", stored, path)
	if err := writeCredentialToKeychain(service, own, path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	before := keychainItemAttributes(t, path, service, own) + keychainItemAttributes(t, path, service, foreign)
	runKeychainFixtureCommand(t, "lock-keychain", path)

	for _, account := range []string{own, foreign} {
		value, err := readPrivateKeychainFixture(t, service, account, path)
		if refusal := fixtureKeychainRefusal(err); len(value) != 0 || fixtureExitCode(err) != failureExit || refusal != (KeychainError{Status: -25293}) || refusal.ItemRefused() {
			t.Fatalf("locked read of %q = %q, %v, %+v", account, value, err, refusal)
		}
	}
	for _, account := range []string{own, foreign, "absent"} {
		err := writePrivateKeychainFixture(t, service, account, path, "synthetic-replacement")
		if refusal := fixtureKeychainRefusal(err); fixtureExitCode(err) != failureExit || refusal != (KeychainError{Status: -25293}) || refusal.ItemRefused() {
			t.Fatalf("locked write of %q = %v, %+v", account, err, refusal)
		}
	}

	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	if after := keychainItemAttributes(t, path, service, own) + keychainItemAttributes(t, path, service, foreign); after != before {
		t.Fatalf("a locked keychain's items changed:\n%s\nwere:\n%s", after, before)
	}
	if value, err := readCredentialFromKeychain(service, own, path); err != nil || string(value) != token {
		t.Fatalf("own item after unlocking = %q, %v", value, err)
	}
	if names := listedKeychainAccounts(t, service, path); !slices.Equal(names, []string{foreign, own}) {
		t.Fatalf("items after unlocking = %q", names)
	}
}

// Two workers that create one new slot at once collide: one adds an item that
// the other has just added, and macOS answers with a duplicate. That writer
// updates the item it now finds, so both succeed, and the slot holds, in one
// labelled item, the value of one of them.
func TestConcurrentWritersOfANewSlotBothSucceed(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "contended"
	values := []string{"synthetic-first-writer", "synthetic-second-writer"}
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

// gatedKeychainWriter is a child that has started and waits, with standard
// input open, to write one value.
type gatedKeychainWriter struct {
	command *exec.Cmd
	gate    io.Closer
	cancel  context.CancelFunc
	stderr  bytes.Buffer
}

// startGatedKeychainWriter starts a writer and returns once it is ready, so
// that releasing several of them starts their writes together.
func startGatedKeychainWriter(t *testing.T, service, account, path, value string) *gatedKeychainWriter {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), keychainFixtureTimeout)
	writer := &gatedKeychainWriter{cancel: cancel}
	t.Cleanup(cancel)
	writer.command = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrivateKeychainMutateChild$")
	writer.command.Env = append(privateKeychainChildEnvironment(service, account, path),
		"AIGW_TEST_KEYCHAIN_VALUE="+value, "AIGW_TEST_KEYCHAIN_GATE=1")
	writer.command.Stderr = &writer.stderr
	gate, err := writer.command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	writer.gate = gate
	ready, err := writer.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.command.Start(); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(ready).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("gated writer did not start: %q, %v", line, err)
	}
	return writer
}

func (writer *gatedKeychainWriter) release() { _ = writer.gate.Close() }

// wait returns the writer's result with its diagnostics, as Output does.
func (writer *gatedKeychainWriter) wait() error {
	defer writer.cancel()
	err := writer.command.Wait()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		exit.Stderr = writer.stderr.Bytes()
	}
	return err
}

func writePrivateKeychainFixture(t *testing.T, service, account, path, value string) error {
	t.Helper()
	_, err := runPrivateKeychainChild(t, os.Args[0], service, account, path, "TestPrivateKeychainMutateChild", "AIGW_TEST_KEYCHAIN_VALUE="+value)
	return err
}

func deletePrivateKeychainFixture(t *testing.T, service, account, path string) error {
	t.Helper()
	_, err := runPrivateKeychainChild(t, os.Args[0], service, account, path, "TestPrivateKeychainMutateChild")
	return err
}

// TestPrivateKeychainMutateChild writes the value that its environment names
// to one slot of a private keychain, or deletes the slot when it names none. A
// gated child says that it is ready and starts when its standard input closes.
func TestPrivateKeychainMutateChild(t *testing.T) {
	path := os.Getenv("AIGW_TEST_KEYCHAIN_PATH")
	if path == "" {
		return
	}
	service, account := os.Getenv("AIGW_TEST_KEYCHAIN_SERVICE"), os.Getenv("AIGW_TEST_KEYCHAIN_ACCOUNT")
	if os.Getenv("AIGW_TEST_KEYCHAIN_GATE") != "" {
		_, _ = os.Stdout.WriteString("ready\n")
		_, _ = io.Copy(io.Discard, os.Stdin)
	}
	var err error
	if value, write := os.LookupEnv("AIGW_TEST_KEYCHAIN_VALUE"); write {
		err = writeCredentialToKeychain(service, account, path, []byte(value))
	} else {
		err = deleteCredentialFromKeychain(service, account, path)
	}
	exitPrivateKeychainChild(nil, err)
}

// fixtureKeychainRefusal reads the diagnostic that a failed child wrote, as
// the parent of a worker reads it, and nothing from a child that wrote none or
// more than the diagnostic's own fields.
func fixtureKeychainRefusal(err error) KeychainError {
	var refusal KeychainError
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		decoder := json.NewDecoder(bytes.NewReader(exit.Stderr))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&refusal) != nil {
			return KeychainError{}
		}
	}
	return refusal
}

// keychainItemAttributes returns a private fixture item's attributes, which
// include its creation and modification times, without its secret data.
func keychainItemAttributes(t *testing.T, path, service, account string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), keychainFixtureTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", service, "-a", account, path).Output()
	if err != nil {
		t.Fatalf("read private Keychain item attributes: %v", err)
	}
	return string(output)
}

// storedKeychainValue reads a private fixture item as the program that stored
// it, which its access list trusts.
func storedKeychainValue(t *testing.T, path, service, account string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), keychainFixtureTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", service, "-a", account, "-w", path).Output()
	if err != nil {
		t.Fatalf("read private Keychain item as the program that stored it: %v", err)
	}
	return strings.TrimSpace(string(output))
}

// listedKeychainAccounts names, sorted, the accounts of a service's items in a
// private keychain.
func listedKeychainAccounts(t *testing.T, service, path string) []string {
	t.Helper()
	names, err := listAccountsInKeychain(service, path)
	if err != nil {
		t.Fatal(err)
	}
	names = slices.DeleteFunc(names, func(name string) bool { return name == "" })
	slices.Sort(names)
	return names
}
