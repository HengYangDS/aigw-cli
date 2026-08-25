//go:build windows

package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsAutomaticFallbackPersistsProtectedToken(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{
		GOOS:         "windows",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("credential manager unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	const token = "windows-dpapi-token"
	if err := store.Set("team", token); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	protected, err := os.ReadFile(filepath.Join(root, "tokens", "team"))
	if err != nil {
		t.Fatalf("read protected Token: %v", err)
	}
	if bytes.Contains(protected, []byte(token)) {
		t.Fatal("fallback store persisted plaintext Token bytes")
	}
	selected, err := os.ReadFile(filepath.Join(root, "backend"))
	if err != nil || string(selected) != "file\n" {
		t.Fatalf("persisted backend = %q, %v", selected, err)
	}

	reopened, err := Select(Selection{
		GOOS:         "windows",
		Root:         root,
		KeyringProbe: func(Store) error { return nil },
	})
	if err != nil {
		t.Fatalf("second Select() error = %v", err)
	}
	value, err := reopened.Get("team")
	if err != nil || value != token {
		t.Fatalf("Get() = %q, %v", value, err)
	}
	if err := reopened.Delete("team"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := reopened.Get("team"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
}

func TestWindowsExplicitFileBackendRoundTrip(t *testing.T) {
	store, err := Select(Selection{Backend: "file", GOOS: "windows", Root: t.TempDir()})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if value, err := store.Get("team"); err != nil || value != "token" {
		t.Fatalf("Get() = %q, %v", value, err)
	}
}
