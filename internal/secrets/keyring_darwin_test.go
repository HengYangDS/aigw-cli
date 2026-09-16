//go:build darwin && keychain_integration

package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticSelectionObservesNativeKeychainWithoutPersisting(t *testing.T) {
	if os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE") != "ephemeral-host" {
		t.Fatal("Keychain integration requires an explicitly admitted ephemeral-host")
	}
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
