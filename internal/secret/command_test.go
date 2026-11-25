package secret

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandQuotesOneExecutableAndScopesCredentialLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aigw client")
	command, err := Command(path, "hermes", "projection")
	if err != nil || command != "'"+path+"' credential hermes projection" {
		t.Fatalf("command = %q, %v", command, err)
	}
	for _, path := range []string{"relative", "/bad\npath", " /leading-space"} {
		if _, err := Command(path, "hermes", "projection"); err == nil {
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
}

func TestCommandRejectsUnsafeCredentialIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aigw")
	for _, tc := range []struct {
		name, client, scope string
	}{
		{"client", "claude;foreign", "projection"},
		{"scope", "claude", "projection;foreign"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, err := Command(path, tc.client, tc.scope)
			if err == nil || command != "" {
				t.Fatalf("unsafe credential identity produced an invocation: %q, %v", command, err)
			}
		})
	}
}

func TestCommandExecutableRoundTripsOnlyTheExactOwnedInvocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projection", "aigw's client")
	command, err := Command(path, "claude", "projection")
	if err != nil {
		t.Fatal(err)
	}
	observed, err := ExecutableFromCommand(command, "claude", "projection")
	if err != nil || observed != path {
		t.Fatalf("round-trip executable = %q, %v", observed, err)
	}
	for _, changed := range []string{
		command + " && echo foreign",
		strings.TrimSuffix(command, " projection") + " other-scope",
		strings.TrimPrefix(command, command[:1]),
		" credential claude projection",
		"'' credential claude projection",
		"'" + path + "' credential claude projection",
	} {
		if _, err := ExecutableFromCommand(changed, "claude", "projection"); err == nil {
			t.Fatal("changed credential invocation was accepted")
		}
	}
}
