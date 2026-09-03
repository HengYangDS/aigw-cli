package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type probeStore struct{ err error }

func (store probeStore) Get(string) (string, error) { return "", store.err }
func (probeStore) Set(string, string) error         { return nil }
func (probeStore) Delete(string) error              { return nil }
func (store probeStore) Exists(string) (bool, error) {
	if errors.Is(store.err, ErrNotFound) {
		return false, nil
	}
	return false, store.err
}

func TestAutomaticSelectionFallsBackWhenWindowsKeyringIsUnavailable(t *testing.T) {
	store, err := Select(Selection{
		GOOS:         "windows",
		Root:         filepath.Join(t.TempDir(), "secrets"),
		Getenv:       func(string) string { return "" },
		KeyringProbe: func(Store) error { return errors.New("unavailable") },
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	automatic, ok := store.(*automaticStore)
	if !ok {
		t.Fatalf("Select() = %T, want *automaticStore", store)
	}
	selected, err := automatic.resolve(false)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if _, ok := selected.(*fileStore); !ok {
		t.Fatalf("resolve() = %T, want *fileStore", selected)
	}
}

func TestExplicitKeyringFailureDoesNotFallback(t *testing.T) {
	_, err := Select(Selection{
		Backend:      "keyring",
		GOOS:         "linux",
		Root:         filepath.Join(t.TempDir(), "secrets"),
		Getenv:       func(string) string { return "" },
		KeyringProbe: func(Store) error { return errors.New("unavailable") },
	})
	if err == nil {
		t.Fatal("Select() succeeded with unavailable explicit keyring")
	}
}

func TestExplicitFileBackendSupportsWindows(t *testing.T) {
	store, err := Select(Selection{Backend: "file", GOOS: "windows", Root: t.TempDir()})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if _, ok := store.(*fileStore); !ok {
		t.Fatalf("Select() = %T, want *fileStore", store)
	}
}

func TestSelectionRejectsUnsupportedAutomaticAndFilePlatforms(t *testing.T) {
	for _, selection := range []Selection{
		{GOOS: "plan9", Root: t.TempDir()},
		{Backend: "file", GOOS: "plan9", Root: t.TempDir()},
	} {
		if _, err := Select(selection); err == nil || !strings.Contains(err.Error(), "operating system") {
			t.Fatalf("Select(%+v) error = %v", selection, err)
		}
	}
}

func TestFileSelectionRequiresStorageRoot(t *testing.T) {
	for _, selection := range []Selection{
		{Backend: "file", GOOS: "linux"},
		{GOOS: "linux"},
	} {
		if _, err := Select(selection); err == nil || !strings.Contains(err.Error(), "storage root") {
			t.Fatalf("Select(%+v) error = %v", selection, err)
		}
	}
}

func TestEnvironmentSelectionDefaultsToEmptyProcessEnvironment(t *testing.T) {
	store, err := Select(Selection{Backend: "env"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("alpha"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestDefaultKeyringProbeRecognizesReachableAndFailedStores(t *testing.T) {
	if err := probeKeyring(probeStore{err: ErrNotFound}, nil); err != nil {
		t.Fatalf("probeKeyring() error = %v", err)
	}
	want := errors.New("service unavailable")
	if err := probeKeyring(probeStore{err: want}, nil); !errors.Is(err, want) {
		t.Fatalf("probeKeyring() error = %v, want %v", err, want)
	}
}

func TestAutomaticBackendSelectionRollbackRemovesOnlyTheTransactionSelection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := Select(Selection{
		GOOS:         runtime.GOOS,
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	rollback, err := PrepareBackendSelectionRollback(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backend selection remains after rollback: %v", err)
	}
}

func TestAutomaticBackendSelectionRollbackRemovesSelectionPersistedByRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := newFileStore(filepath.Join(root, "tokens")).Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{
		GOOS:         runtime.GOOS,
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	rollback, err := PrepareBackendSelectionRollback(store)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := store.Get("team"); err != nil || value != "token" {
		t.Fatalf("Get() = %q, %v", value, err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backend selection remains after rollback: %v", err)
	}
}

func TestAutomaticBackendSelectionRollbackPreservesExistingSelection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	if err := choice.Write("file"); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	rollback, err := PrepareBackendSelectionRollback(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if selected, err := choice.Read(); err != nil || selected != "file" {
		t.Fatalf("backend selection = %q, %v; want file", selected, err)
	}
}

func TestAutomaticBackendSelectionRollbackPreservesDrift(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	store, err := Select(Selection{
		GOOS:         runtime.GOOS,
		Root:         root,
		KeyringProbe: func(Store) error { return errors.New("unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	rollback, err := PrepareBackendSelectionRollback(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := choice.Write("keyring"); err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("rollback error = %v; want drift rejection", err)
	}
	if selected, err := choice.Read(); err != nil || selected != "keyring" {
		t.Fatalf("backend selection = %q, %v; want newer keyring", selected, err)
	}
}

func TestAutomaticBackendSelectionRollbackPreservesSelectionCreatedByAnotherWriter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	store, err := Select(Selection{GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	rollback, err := PrepareBackendSelectionRollback(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := choice.Write("file"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if selected, err := choice.Read(); err != nil || selected != "file" {
		t.Fatalf("backend selection = %q, %v; want externally created file selection", selected, err)
	}
}

func TestExplicitBackendSelectionRollbackIsNoOp(t *testing.T) {
	rollback, err := PrepareBackendSelectionRollback(NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
}
