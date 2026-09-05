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
	selected, err := automatic.resolve()
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

func TestAutomaticBackendSelectionReadNeedsNoRollback(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(root, backendChoiceName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only Get() persisted backend selection: %v", err)
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
	if _, _, err := choice.Persist("file"); err != nil {
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

func TestAutomaticBackendSelectionRollbackRejectsInvalidSelectionState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, backendChoiceName), []byte("unknown\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareBackendSelectionRollback(store); err == nil || !strings.Contains(err.Error(), "observe automatic credential backend selection") {
		t.Fatalf("PrepareBackendSelectionRollback() error = %v", err)
	}
}

func TestAutomaticBackendSelectionRollbackPreservesDrift(t *testing.T) {
	for _, replacement := range []string{"keyring", "file"} {
		t.Run(replacement, func(t *testing.T) {
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
			if err := replaceBackendChoice(choice, replacement); err != nil {
				t.Fatal(err)
			}
			if err := rollback(); err == nil || !strings.Contains(err.Error(), "changed") {
				t.Fatalf("rollback error = %v; want drift rejection", err)
			}
			if selected, err := choice.Read(); err != nil || selected != replacement {
				t.Fatalf("backend selection = %q, %v; want %s", selected, err, replacement)
			}
		})
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
	if _, _, err := choice.Persist("file"); err != nil {
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

func TestAutomaticMutationFailurePreservesExistingBackendSelection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	if _, _, err := choice.Persist("file"); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential mutation failed")
	automatic := store.(*automaticStore)
	if err := automatic.mutate(func(typedStore) error { return want }); !errors.Is(err, want) {
		t.Fatalf("mutate() error = %v, want %v", err, want)
	}
	if selected, err := choice.Read(); err != nil || selected != "file" {
		t.Fatalf("backend selection = %q, %v; want retained file selection", selected, err)
	}
}

func TestAutomaticFileMutationFailureRemovesItsBackendSelection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.MkdirAll(filepath.Join(root, "tokens", "team"), 0o700); err != nil {
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
	if err := store.Set("team", "token"); err == nil {
		t.Fatal("Set() replaced a directory at the credential slot")
	}
	if _, err := os.Stat(filepath.Join(root, backendChoiceName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backend selection remains after failed credential mutation: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "tokens", "team")); err != nil || !info.IsDir() {
		t.Fatalf("credential slot = %#v, %v; want preserved directory", info, err)
	}
}

func TestAutomaticMutationFailureReportsBackendRollbackConflict(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	automatic := &automaticStore{
		selection:       Selection{GOOS: runtime.GOOS, Root: root},
		choice:          choice,
		selected:        NewMemoryStore(),
		selectedBackend: "file",
	}
	want := errors.New("credential mutation failed")
	err := automatic.mutate(func(typedStore) error {
		if replaceErr := replaceBackendChoice(choice, "keyring"); replaceErr != nil {
			t.Fatal(replaceErr)
		}
		return want
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restore automatic credential backend selection") {
		t.Fatalf("mutate() error = %v; want mutation and rollback conflict", err)
	}
	if selected, readErr := choice.Read(); readErr != nil || selected != "keyring" {
		t.Fatalf("backend selection = %q, %v; want external keyring selection", selected, readErr)
	}
}

func replaceBackendChoice(choice backendChoice, backend string) error {
	root, err := openSecureRoot(choice.root, true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeSecureFile(root, backendChoiceName, []byte(backend+"\n"))
}
