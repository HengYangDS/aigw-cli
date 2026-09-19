//go:build darwin

package native

import (
	"errors"
	"slices"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestProviderPreservesLogicalValueAcrossLifecycle(t *testing.T) {
	keyring.MockInit()
	const service, account, token = "AIGW_TOKEN", "provider-test", "line one\nline two"

	if _, err := queryCredential(writeCommand, service, account, []byte(token)); err != nil {
		t.Fatal(err)
	}
	if stored, err := keyring.Get(service, account); err != nil || stored != token {
		t.Fatalf("stored value = %q, %v", stored, err)
	}
	if value, err := queryCredential(readCommand, service, account, nil); err != nil || string(value) != token {
		t.Fatalf("read value = %q, %v", value, err)
	}
	if _, err := queryCredential(deleteCommand, service, account, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := queryCredential(deleteCommand, service, account, nil); err != nil {
		t.Fatalf("repeated delete = %v", err)
	}
	if _, err := queryCredential(readCommand, service, account, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing read = %v", err)
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
