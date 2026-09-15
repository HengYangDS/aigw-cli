//go:build darwin

package secrets

import (
	"errors"
	"path/filepath"
	"testing"
)

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
