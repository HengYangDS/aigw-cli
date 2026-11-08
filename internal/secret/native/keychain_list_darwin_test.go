//go:build darwin && cgo

package native

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A private keychain's accounts are named from their attributes alone, even
// for an item whose access list denies this program, with no UI, and without
// another service's items. The test never lists the default keychains.
func TestKeychainListNamesAccountsWithoutReadingValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)

	const service, token = "aigw-private-list", "synthetic-listing-value"
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "team", "-w", token, "-A", path)
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "denied", "-w", token, "-T", "", path)
	runKeychainFixtureCommand(t, "add-generic-password", "-s", "aigw-other-service", "-a", "foreign", "-w", token, "-A", path)

	names, err := listAccountsInKeychain(service, path)
	if err != nil {
		t.Fatal(err)
	}
	names = slices.DeleteFunc(names, func(name string) bool { return name == "" })
	slices.Sort(names)
	if !slices.Equal(names, []string{"denied", "team"}) {
		t.Fatalf("listed accounts = %q", names)
	}
	if slices.ContainsFunc(names, func(name string) bool { return strings.Contains(name, token) }) {
		t.Fatal("a listing returned a value")
	}
	if names, err := listAccountsInKeychain("aigw-private-absent", path); err != nil || len(names) != 0 {
		t.Fatalf("a service without items = %q, %v", names, err)
	}
}

// A locked keychain is named or refused, and never waits for a person: the
// child that lists it is bounded.
func TestLockedKeychainListingDoesNotPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked-list.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	runKeychainFixtureCommand(t, "add-generic-password", "-s", "aigw-private-list", "-a", "locked", "-w", "synthetic", "-A", path)
	runKeychainFixtureCommand(t, "lock-keychain", path)

	output, err := runPrivateKeychainChild(t, os.Args[0], "aigw-private-list", "", path, "TestPrivateKeychainListChild")
	if err == nil && strings.TrimSpace(string(output)) == "locked" || fixtureExitCode(err) == failureExit && len(output) == 0 {
		return
	}
	t.Fatalf("a locked keychain's listing = %q, %v", output, err)
}

func TestPrivateKeychainListChild(t *testing.T) {
	path := os.Getenv("AIGW_TEST_KEYCHAIN_PATH")
	if path == "" {
		return
	}
	names, err := listAccountsInKeychain(os.Getenv("AIGW_TEST_KEYCHAIN_SERVICE"), path)
	if err != nil {
		os.Exit(failureExit)
	}
	_, _ = os.Stdout.WriteString(strings.Join(names, "\n"))
	os.Exit(0)
}
