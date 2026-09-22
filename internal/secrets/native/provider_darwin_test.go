//go:build darwin

package native

import (
	"slices"
	"testing"
)

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
