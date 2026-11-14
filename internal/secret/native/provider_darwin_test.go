//go:build darwin && cgo

package native

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestKeychainReadUsesPrivateFixtureWithoutAuthorizationUI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)

	const service, account, token = "aigw-private-test", "authorized", "synthetic\nvalue"
	for _, item := range []struct {
		account string
		stored  string
	}{
		{account, "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))},
		{"legacy-hex", "go-keyring-encoded:" + hex.EncodeToString([]byte(token))},
		{"legacy-plain", token},
	} {
		runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", item.account, "-w", item.stored, "-A", path)
		value, err := readCredentialFromKeychain(service, item.account, path)
		if err != nil || string(value) != token {
			t.Fatalf("private item %q read = %q, %v", item.account, value, err)
		}
	}
	if present, err := observeCredentialInKeychain(service, account, path); err != nil || !present {
		t.Fatalf("authorized item metadata = %t, %v", present, err)
	}
	if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
		t.Fatalf("native subprocess lost the retained Account identity: %v", err)
	}
	if present, err := observeCredentialInKeychain(service, "absent", path); err != nil || present {
		t.Fatalf("missing item metadata = %t, %v", present, err)
	}
	assertKeychainMutationPreservesDeniedItem(t, service, path, token)

	if value, err := readCredentialFromKeychain(service, "absent", path); len(value) != 0 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing private item = %q, %v", value, err)
	}
	for _, item := range []struct {
		account string
		stored  string
	}{
		{"invalid-base64", "go-keyring-base64:***"},
		{"invalid-hex", "go-keyring-encoded:ZZ"},
	} {
		runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", item.account, "-w", item.stored, "-A", path)
		if value, err := readCredentialFromKeychain(service, item.account, path); len(value) != 0 || !errors.Is(err, ErrUnavailable) {
			t.Fatalf("invalid private item %q = %q, %v", item.account, value, err)
		}
	}
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "denied", "-w", "synthetic", "-T", "", path)
	if present, err := observeCredentialInKeychain(service, "denied", path); err != nil || !present {
		t.Fatalf("denied item metadata = %t, %v", present, err)
	}
	if value, err := readPrivateKeychainFixture(t, service, "denied", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("unauthorized private item = %q, %v", value, err)
	} else {
		failure, ok := errors.AsType[*exec.ExitError](err)
		if !ok || !bytes.Contains(failure.Stderr, []byte("keychain_status")) {
			t.Fatal("native denied read lost its safe status diagnostic")
		}
	}
}

func TestNewNativeKeychainItemsHaveReadablePurposeLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labels.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)

	const service = "aigw-private-test"
	// security renders kSecLabelItemAttr with its legacy numeric tag.
	const labelAttribute = "0x00000007 <blob>="
	for _, item := range []struct {
		account string
		label   string
	}{
		{"team", "AIGW Account Token: team"},
		{"diagnostic@team", "AIGW Provider Diagnostic: team"},
		{"native@team", "AIGW Account Token: native@team"},
	} {
		t.Run(item.account, func(t *testing.T) {
			if err := writeCredentialToKeychain(service, item.account, path, []byte("synthetic")); err != nil {
				t.Fatal(err)
			}
			observed := ""
			for line := range strings.SplitSeq(keychainItemAttributes(t, path, service, item.account), "\n") {
				if strings.Contains(line, labelAttribute) {
					observed = strings.TrimSpace(line)
					break
				}
			}
			if observed != labelAttribute+`"`+item.label+`"` {
				t.Fatalf("new native item label = %q, want %q", observed, item.label)
			}
		})
	}
}

func TestPrivateKeychainCopiedReaderUsesExactExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "copied-reader.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	const service, account, token = "aigw-private-test", "copied-reader", "synthetic-copied-reader-token"
	if err := writeCredentialToKeychain(service, account, path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(t.TempDir(), "copied-aigw-test")
	if err := os.WriteFile(copied, data, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Run("missing copied reader", func(t *testing.T) {
		value, err := runPrivateKeychainChild(t, copied+".absent", service, account, path, "TestPrivateKeychainReadChild")
		if err == nil || len(value) != 0 {
			t.Fatal("missing copied reader silently used another executable")
		}
	})
	for _, executable := range []string{os.Args[0], copied} {
		t.Run(executable, func(t *testing.T) {
			got, err := os.ReadFile(executable)
			if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
				t.Fatalf("credential reader copy changed executable identity: %v", err)
			}
			value, err := runPrivateKeychainChild(t, executable, service, account, path, "TestPrivateKeychainReadChild")
			if err != nil || string(value) != token {
				t.Fatalf("same-byte native reader cannot read its creator-authorized item: %v", err)
			}
		})
	}
	if err := writeCredentialToKeychain(service, account, path, []byte("rotated-synthetic-token")); err != nil {
		t.Fatal(err)
	}
	value, err := runPrivateKeychainChild(t, copied, service, account, path, "TestPrivateKeychainReadChild")
	if err != nil || string(value) != "rotated-synthetic-token" {
		t.Fatalf("copied reader lost native item rotation: %v", err)
	}
	if err := deleteCredentialFromKeychain(service, account, path); err != nil {
		t.Fatal(err)
	}
	value, err = runPrivateKeychainChild(t, copied, service, account, path, "TestPrivateKeychainReadChild")
	if len(value) != 0 || fixtureExitCode(err) != missingExit {
		t.Fatalf("copied reader retained a deleted native item: %v", err)
	}
}

func TestLockedKeychainMetadataDoesNotPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	runKeychainFixtureCommand(t, "add-generic-password", "-s", "aigw-private-test", "-a", "locked", "-w", "synthetic", "-A", path)
	runKeychainFixtureCommand(t, "lock-keychain", path)

	value, err := observePrivateKeychainFixture(t, "aigw-private-test", "locked", path)
	if err == nil && string(value) == "1" || fixtureExitCode(err) == failureExit && len(value) == 0 {
		return
	}
	t.Fatalf("locked Keychain metadata was reported absent or stalled: %q, %v", value, err)
}

func assertKeychainMutationPreservesDeniedItem(t *testing.T, service, path, token string) {
	t.Helper()
	// The worker neither reads nor replaces an item that trusts only another
	// application.
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "foreign-writer", "-w", "go-keyring-base64:"+base64.StdEncoding.EncodeToString([]byte(token)), "-T", "/usr/bin/true", path)
	foreign := keychainItemAttributes(t, path, service, "foreign-writer")
	if value, err := readPrivateKeychainFixture(t, service, "foreign-writer", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("foreign item unexpectedly bypassed authorization: %q, %v", value, err)
	}
	if err := writeCredentialToKeychain(service, "foreign-writer", path, []byte("replacement")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unauthorized foreign item accepted a replacement: %v", err)
	}
	const nativeAccount = "native-writer"
	for _, value := range [][]byte{nil, make([]byte, maxStoredValue+1)} {
		if err := writeCredentialToKeychain(service, nativeAccount, path, value); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("invalid native value was accepted: %v", err)
		}
	}
	if err := writeCredentialToKeychain(service, nativeAccount, path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, nativeAccount, path); err != nil || string(value) != token {
		t.Fatalf("native writer's item = %q, %v", value, err)
	}
	if err := writeCredentialToKeychain(service, nativeAccount, path, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, nativeAccount, path); err != nil || string(value) != "replacement" {
		t.Fatalf("updated native item = %q, %v", value, err)
	}
	if err := deleteCredentialFromKeychain(service, nativeAccount, path); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, nativeAccount, path); len(value) != 0 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted native item = %q, %v", value, err)
	}
	if value, err := readPrivateKeychainFixture(t, service, "foreign-writer", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("foreign item changed during native-slot lifecycle: %q, %v", value, err)
	}
	if after := keychainItemAttributes(t, path, service, "foreign-writer"); after != foreign {
		t.Fatalf("foreign item changed during native-slot lifecycle:\n%s\nwas:\n%s", after, foreign)
	}
}

func readPrivateKeychainFixture(t *testing.T, service, account, path string) ([]byte, error) {
	return runPrivateKeychainChild(t, os.Args[0], service, account, path, "TestPrivateKeychainReadChild")
}

func observePrivateKeychainFixture(t *testing.T, service, account, path string) ([]byte, error) {
	return runPrivateKeychainChild(t, os.Args[0], service, account, path, "TestPrivateKeychainObserveChild")
}

// runPrivateKeychainChild runs one bounded operation on a private keychain in
// a child, which environment can give more than the item's identity.
func runPrivateKeychainChild(t *testing.T, executable, service, account, path, testName string, environment ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), keychainFixtureTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^"+testName+"$")
	command.Env = append(privateKeychainChildEnvironment(service, account, path), environment...)
	output, err := command.Output()
	if ctx.Err() != nil {
		t.Fatalf("private Keychain operation did not return without UI: %v", ctx.Err())
	}
	return output, err
}

func privateKeychainChildEnvironment(service, account, path string) []string {
	return append(os.Environ(),
		"AIGW_TEST_KEYCHAIN_PATH="+path,
		"AIGW_TEST_KEYCHAIN_SERVICE="+service,
		"AIGW_TEST_KEYCHAIN_ACCOUNT="+account,
	)
}

func TestPrivateKeychainObserveChild(t *testing.T) {
	path := os.Getenv("AIGW_TEST_KEYCHAIN_PATH")
	if path == "" {
		return
	}
	present, err := observeCredentialInKeychain(
		os.Getenv("AIGW_TEST_KEYCHAIN_SERVICE"),
		os.Getenv("AIGW_TEST_KEYCHAIN_ACCOUNT"), path,
	)
	if err != nil {
		os.Exit(failureExit)
	}
	if present {
		_, _ = os.Stdout.WriteString("1")
	} else {
		_, _ = os.Stdout.WriteString("0")
	}
	os.Exit(0)
}

func fixtureExitCode(err error) int {
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode()
	}
	return 0
}

func TestPrivateKeychainReadChild(t *testing.T) {
	path := os.Getenv("AIGW_TEST_KEYCHAIN_PATH")
	if path == "" {
		return
	}
	value, err := readCredentialFromKeychain(
		os.Getenv("AIGW_TEST_KEYCHAIN_SERVICE"),
		os.Getenv("AIGW_TEST_KEYCHAIN_ACCOUNT"), path,
	)
	exitPrivateKeychainChild(value, err)
}

// exitPrivateKeychainChild ends a child as the worker ends: the value on
// standard output, a missing item and a failure as their exit codes, and a
// Keychain status as the diagnostic.
func exitPrivateKeychainChild(value []byte, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		os.Exit(missingExit)
	case err != nil:
		if failure, ok := errors.AsType[*KeychainError](err); ok {
			_ = json.NewEncoder(os.Stderr).Encode(failure)
		}
		os.Exit(failureExit)
	}
	_, _ = os.Stdout.Write(value)
	os.Exit(0)
}

func TestNativePresenceCommandClassifiesExactOutput(t *testing.T) {
	if _, err := queryCredential("unknown", "aigw-private-test", "synthetic", nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown credential operation = %v", err)
	}
	for _, test := range []struct {
		output  string
		present bool
		wantErr error
	}{
		{output: "1", present: true},
		{output: "0"},
		{output: "?", wantErr: ErrUnavailable},
	} {
		t.Run(test.output, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), "synthetic-presence")
			if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf '"+test.output+"'"), 0o700); err != nil {
				t.Fatal(err)
			}
			present, err := Exists(program, "aigw-private-test", "synthetic")
			if present != test.present || !errors.Is(err, test.wantErr) {
				t.Fatalf("presence = %t, %v", present, err)
			}
		})
	}
}

// keychainFixtureTimeout bounds a test's own /usr/bin/security calls and the
// children it waits for, so that it stops only a hang. A loaded shared CI host
// slowed create-keychain past five seconds (GitLab job 69467, 2026-11-13).
// AIGW's own native operations keep operationTimeout.
const keychainFixtureTimeout = 30 * time.Second

func runKeychainFixtureCommand(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), keychainFixtureTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/security", args...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("private Keychain fixture command %q failed: %v: %s", args[0], err, output)
	}
}

// newPrivateKeychain creates an unlocked keychain file of the test's own,
// which never locks by itself, and deletes it with the test.
func newPrivateKeychain(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "set-keychain-settings", path)
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	return path
}

func TestNativeEnvironmentRetainsOnlyKeychainHome(t *testing.T) {
	values := map[string]string{"HOME": "/Users/runner", "PATH": "/untrusted"}
	want := []string{"HOME=/Users/runner"}
	if got := nativeEnvironment(func(name string) string { return values[name] }); !slices.Equal(got, want) {
		t.Fatalf("native environment = %q, want %q", got, want)
	}
}
