//go:build darwin

package native

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "security-writer", "-w", "go-keyring-base64:"+base64.StdEncoding.EncodeToString([]byte(token)), path)
	if value, err := readPrivateKeychainFixture(t, service, "security-writer", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("old writer's item unexpectedly bypassed authorization: %q, %v", value, err)
	}
	if err := writeCredentialToKeychain(service, "security-writer", path, []byte("replacement")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unauthorized legacy item accepted a replacement: %v", err)
	}
	for _, value := range [][]byte{nil, make([]byte, maxStoredValue+1)} {
		if err := writeCredentialToKeychain(service, "native-writer", path, value); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("invalid native value was accepted: %v", err)
		}
	}
	if err := writeCredentialToKeychain(service, "native-writer", path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, "native-writer", path); err != nil || string(value) != token {
		t.Fatalf("native writer's item = %q, %v", value, err)
	}
	if err := writeCredentialToKeychain(service, "native-writer", path, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, "native-writer", path); err != nil || string(value) != "replacement" {
		t.Fatalf("updated native item = %q, %v", value, err)
	}
	if err := deleteCredentialFromKeychain(service, "native-writer", path); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, "native-writer", path); len(value) != 0 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted native item = %q, %v", value, err)
	}

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
	if value, err := readPrivateKeychainFixture(t, service, "denied", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("unauthorized private item = %q, %v", value, err)
	}
}

func readPrivateKeychainFixture(t *testing.T, service, account, path string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrivateKeychainReadChild$")
	command.Env = append(os.Environ(),
		"AIGW_TEST_KEYCHAIN_PATH="+path,
		"AIGW_TEST_KEYCHAIN_SERVICE="+service,
		"AIGW_TEST_KEYCHAIN_ACCOUNT="+account,
	)
	output, err := command.Output()
	if ctx.Err() != nil {
		t.Fatalf("private Keychain read did not return without UI: %v", ctx.Err())
	}
	return output, err
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
	switch {
	case errors.Is(err, ErrNotFound):
		os.Exit(missingExit)
	case err != nil:
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

func runKeychainFixtureCommand(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/security", args...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("private Keychain fixture command %q failed: %v: %s", args[0], err, output)
	}
}

func TestKeychainMetadataCommandDoesNotRequestPasswordData(t *testing.T) {
	command := keychainMetadataCommand("AIGW_TOKEN", "team")
	want := []string{"/usr/bin/security", "find-generic-password", "-s", "AIGW_TOKEN", "-a", "team"}
	if !slices.Equal(command.Args, want) {
		t.Fatalf("Keychain metadata command = %q, want %q", command.Args, want)
	}
}

func TestNativeEnvironmentRetainsOnlyKeychainHome(t *testing.T) {
	values := map[string]string{"HOME": "/Users/runner", "PATH": "/untrusted"}
	want := []string{"HOME=/Users/runner"}
	if got := nativeEnvironment(func(name string) string { return values[name] }); !slices.Equal(got, want) {
		t.Fatalf("native environment = %q, want %q", got, want)
	}
}
