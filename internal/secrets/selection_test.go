//go:build !windows

package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomaticSelectionFallsBackAndPersistsOnLinux(t *testing.T) {
	data := t.TempDir()
	root := filepath.Join(data, "secrets")
	store, err := Select(Selection{
		Backend:      "",
		GOOS:         "linux",
		Root:         root,
		Getenv:       func(string) string { return "" },
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Select() mutated storage before first use: %v", err)
	}
	if err := store.Set("alpha", "token"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	selected, err := Select(Selection{
		GOOS:         "linux",
		Root:         filepath.Join(data, "secrets"),
		Getenv:       func(string) string { return "" },
		KeyringProbe: func(Store) error { return nil },
	})
	if err != nil {
		t.Fatalf("second Select() error = %v", err)
	}
	value, err := selected.Get("alpha")
	if err != nil || value != "token" {
		t.Fatalf("persisted selection Get() = %q, %v", value, err)
	}
}

func TestAutomaticSelectionMissingTokenDoesNotPersist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only miss persisted backend selection: %v", err)
	}
}

func TestAutomaticSelectionSuccessfulGetDoesNotPersist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := newFileStore(filepath.Join(root, "tokens")).Set("alpha", "token"); err != nil {
		t.Fatalf("seed file Token: %v", err)
	}
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	value, err := store.Get("alpha")
	if err != nil || value != "token" {
		t.Fatalf("Get() = %q, %v", value, err)
	}
	if _, err := os.Stat(filepath.Join(root, backendChoiceName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only Get() persisted backend selection: %v", err)
	}
}

func TestAutomaticSelectionPersistedKeyringNeverFallsBack(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	if _, _, err := choice.Persist("keyring"); err != nil {
		t.Fatalf("persist keyring selection: %v", err)
	}
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if _, err := store.Get("alpha"); err == nil || !strings.Contains(err.Error(), "service unavailable") {
		t.Fatalf("Get() error = %v, want persisted keyring failure", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tokens")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("persisted keyring failure created fallback store: %v", err)
	}
}

func TestAutomaticSelectionFailedMutationDoesNotPersist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if err := store.Set("invalid profile", "token"); err == nil {
		t.Fatal("Set() accepted invalid profile")
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed mutation persisted backend selection: %v", err)
	}
}

func TestAutomaticSelectionCachesAndPersistsOneBackend(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.Set("alpha", "token"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := store.Set("beta", "token"); err != nil {
		t.Fatalf("second Set() error = %v", err)
	}
	if !mustExist(t, store, "alpha") {
		t.Fatal("Has() did not reuse selected backend")
	}
	if value, err := store.Get("beta"); err != nil || value != "token" {
		t.Fatalf("second Get() = %q, %v", value, err)
	}
	if err := store.Delete("alpha"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestAutomaticSelectionReportsInitialMarkerFailure(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "secrets")
	store, err := Select(Selection{
		GOOS: "linux",
		Root: root,
		KeyringProbe: func(Store) error {
			if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
				t.Fatalf("prepare marker failure: %v", err)
			}
			return errors.New("service unavailable")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("alpha", "token"); err == nil {
		t.Fatal("Set() ignored initial selection marker failure")
	}
}

func TestAutomaticSuccessfulReadDoesNotTouchSelectionStorage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	memory := NewMemoryStore()
	if err := memory.Set("alpha", "token"); err != nil {
		t.Fatal(err)
	}
	blockedRoot := filepath.Join(root, "blocked")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockedRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &automaticStore{
		choice:          newBackendChoice(filepath.Join(blockedRoot, "child")),
		selected:        memory,
		selectedBackend: "file",
	}
	value, err := store.Get("alpha")
	if err != nil || value != "token" {
		t.Fatalf("Get() = %q, %v; want token without a selection write", value, err)
	}
}

func TestAutomaticSelectionRejectsInvalidPersistedChoice(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend"), []byte("unknown\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{GOOS: "linux", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("alpha"); err == nil || !strings.Contains(err.Error(), "invalid persisted") {
		t.Fatalf("Get() error = %v", err)
	}
	if present, err := store.Exists("alpha"); err == nil || present || !strings.Contains(err.Error(), "invalid persisted") {
		t.Fatalf("Exists() = %v, %v", present, err)
	}
}

func TestAutomaticSelectionSuccessfulGetReportsMarkerFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := newFileStore(filepath.Join(root, "tokens")).Set("alpha", "token"); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("alpha"); err == nil || !strings.Contains(err.Error(), "owner-only directory") {
		t.Fatalf("Get() marker error = %v", err)
	}
}

func TestAutomaticDeleteRejectsInvalidAccountAndBrokenSelection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{GOOS: "linux", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("invalid account"); err == nil {
		t.Fatal("Delete() accepted an invalid Account ID")
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("alpha"); err == nil {
		t.Fatal("Delete() ignored invalid backend selection storage")
	}
}

func TestAutomaticSelectionReportsMarkerPersistenceFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{
		GOOS:         "linux",
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("alpha", "token"); err == nil {
		t.Fatal("Set() ignored unsafe selection storage")
	}
}
