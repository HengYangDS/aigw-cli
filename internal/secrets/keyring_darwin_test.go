//go:build darwin

package secrets

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

func TestKeychainMetadataCommandDoesNotRequestPasswordData(t *testing.T) {
	command := keychainMetadataCommand("AIGW_TOKEN", "team")
	want := []string{
		"/usr/bin/security",
		"find-generic-password",
		"-s", "AIGW_TOKEN",
		"-a", "team",
	}
	if !slices.Equal(command.Args, want) {
		t.Fatalf("Keychain metadata command = %q, want %q", command.Args, want)
	}
}

func TestAutomaticSelectionObservesNativeKeychainWithoutPersisting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{GOOS: "darwin", Root: root})
	if err != nil {
		t.Fatal(err)
	}

	got, err := Inspect(store)
	if err != nil {
		t.Fatal(err)
	}
	want := BackendSelection{
		Kind:         "keyring",
		Availability: "available",
		Mutability:   "read_write",
		Persistence:  "deferred",
	}
	if got != want {
		t.Fatalf("Inspect() = %#v, want %#v", got, want)
	}
	if _, err := newBackendChoice(root).Read(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("value-free inspection persisted backend selection: %v", err)
	}
}

func TestClassifyKeychainObservation(t *testing.T) {
	absent := keychainMetadataCommand("aigw-test-absent", "aigw-test-absent")
	absentOutput, absentErr := absent.CombinedOutput()
	if absentErr == nil {
		t.Fatal("absent Keychain fixture unexpectedly exists")
	}
	tests := []struct {
		name    string
		output  []byte
		err     error
		present bool
		wantErr bool
	}{
		{name: "present", present: true},
		{name: "absent", output: absentOutput, err: absentErr},
		{name: "failure", err: errors.New("security failed"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			present, err := classifyKeychainObservation(test.output, test.err)
			if present != test.present || (err != nil) != test.wantErr {
				t.Fatalf("classifyKeychainObservation() = %v, %v", present, err)
			}
		})
	}
}
