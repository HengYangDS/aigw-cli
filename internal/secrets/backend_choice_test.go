package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackendChoiceLifecycleAndValidation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	if _, err := choice.Read(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read() missing error = %v", err)
	}
	if _, _, err := choice.Persist("unknown"); err == nil {
		t.Fatal("Persist() accepted an unknown backend")
	}
	if _, written, err := choice.Persist("file"); err != nil || !written {
		t.Fatalf("Persist() = written %v, error %v", written, err)
	}
	backend, err := choice.Read()
	if err != nil || backend != "file" {
		t.Fatalf("Read() = %q, %v", backend, err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend"), []byte("unknown\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := choice.Read(); err == nil || !strings.Contains(err.Error(), "invalid persisted") {
		t.Fatalf("Read() invalid marker error = %v", err)
	}
	if _, _, err := choice.Persist("file"); err == nil || !strings.Contains(err.Error(), "invalid persisted") {
		t.Fatalf("Persist() invalid marker error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, backendChoiceName), []byte("file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := choice.Persist("keyring"); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Persist() conflicting marker error = %v", err)
	}
}

func TestBackendChoiceRollbackRejectsReplacedPostimage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	postimage, written, err := choice.Persist("file")
	if err != nil || !written {
		t.Fatal(err)
	}
	if err := replaceBackendChoice(choice, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := choice.Rollback(postimage); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Rollback() mismatch error = %v, want changed postimage", err)
	}
	if backend, err := choice.Read(); err != nil || backend != "keyring" {
		t.Fatalf("backend after rejected rollback = %q, %v; want keyring", backend, err)
	}

	exact, created, err := choice.Persist("keyring")
	if err != nil || created || exact.exists {
		t.Fatalf("Persist(existing) = %#v, created %v, error %v", exact, created, err)
	}
	if err := choice.Rollback(postimage); err == nil {
		t.Fatal("Rollback() accepted a stale postimage")
	}
}

func TestBackendChoiceRollbackRequiresAnAvailableOwnedPostimage(t *testing.T) {
	choice := newBackendChoice(filepath.Join(t.TempDir(), "missing"))
	if err := choice.Rollback(credentialFileSnapshot{}); err == nil || !strings.Contains(err.Error(), "owned postimage") {
		t.Fatalf("Rollback() unowned postimage error = %v", err)
	}

	identityPath := filepath.Join(t.TempDir(), "identity")
	if err := os.WriteFile(identityPath, []byte("file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(identityPath)
	if err != nil {
		t.Fatal(err)
	}
	postimage := credentialFileSnapshot{
		value:    []byte("file\n"),
		identity: identifyFile(info),
		exists:   true,
	}
	if err := choice.Rollback(postimage); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Rollback() missing storage error = %v", err)
	}

	invalidRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(invalidRoot, []byte("file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := newBackendChoice(invalidRoot).Rollback(postimage); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("Rollback() invalid storage error = %v", err)
	}
}

func TestBackendChoiceRollbackRemovesItsOwnPostimage(t *testing.T) {
	choice := newBackendChoice(filepath.Join(t.TempDir(), "secrets"))
	postimage, written, err := choice.Persist("file")
	if err != nil || !written {
		t.Fatalf("Persist() = written %v, error %v", written, err)
	}
	if err := choice.Rollback(postimage); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if _, err := choice.Read(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read() after Rollback() error = %v, want ErrNotFound", err)
	}
}

func TestBackendChoiceReportsAtomicWriteFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "backend"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := newBackendChoice(root).Persist("file"); err == nil {
		t.Fatal("Persist() replaced a backend directory")
	}
}
