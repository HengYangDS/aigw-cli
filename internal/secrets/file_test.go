//go:build !windows

package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type fileInfoStub struct {
	mode os.FileMode
	stat *syscall.Stat_t
}

func (info fileInfoStub) Name() string       { return "stub" }
func (info fileInfoStub) Size() int64        { return 0 }
func (info fileInfoStub) Mode() os.FileMode  { return info.mode }
func (info fileInfoStub) ModTime() time.Time { return time.Time{} }
func (info fileInfoStub) IsDir() bool        { return info.mode.IsDir() }
func (info fileInfoStub) Sys() any           { return info.stat }

type syncerStub struct{ err error }

func (syncer syncerStub) Sync() error { return syncer.err }

func TestFileStoreCRUDUsesOwnerOnlyFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store := newFileStore(root)
	if err := store.Set("alpha", "token"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, err := store.Get("alpha")
	if err != nil || value != "token" {
		t.Fatalf("Get() = %q, %v", value, err)
	}
	directory, err := os.Stat(root)
	if err != nil || directory.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %v, err = %v", directory.Mode().Perm(), err)
	}
	file, err := os.Stat(filepath.Join(root, "alpha"))
	if err != nil || file.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, err = %v", file.Mode().Perm(), err)
	}
	if err := store.Delete("alpha"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get("alpha"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after delete error = %v", err)
	}
}

func TestFileStoreRejectsSymlink(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "alpha")); err != nil {
		t.Fatal(err)
	}
	store := newFileStore(root)
	if _, err := store.Get("alpha"); err == nil {
		t.Fatal("Get() accepted a symlink")
	}
	if present, err := store.Exists("alpha"); err == nil || present {
		t.Fatal("Exists() accepted a symlink")
	}
	if err := store.Set("alpha", "replacement"); err == nil {
		t.Fatal("Set() replaced a symlink")
	}
}

func TestFileStoreRejectsUnsafePermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "alpha")
	if err := os.WriteFile(file, []byte("token"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newFileStore(root).Get("alpha"); err == nil {
		t.Fatal("Get() accepted group/world-readable Token")
	}
	if present, err := newFileStore(root).Exists("alpha"); err == nil || present {
		t.Fatal("Exists() accepted group/world-readable Token")
	}
}

func TestFileStoreRejectsHardLink(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "alpha")
	if err := os.WriteFile(file, []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(file, filepath.Join(t.TempDir(), "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := newFileStore(root).Get("alpha"); err == nil {
		t.Fatal("Get() accepted a multiply-linked Token")
	}
}

func TestFileStoreRejectsUnsafeDirectoryPermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := newFileStore(root).Get("alpha"); err == nil {
		t.Fatal("Get() accepted group/world-accessible Token directory")
	}
	if present, err := newFileStore(root).Exists("alpha"); err == nil || present {
		t.Fatal("Exists() accepted group/world-accessible Token directory")
	}
}

func TestFileStoreRejectsSymlinkedRoot(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(parent, "linked")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	if err := newFileStore(linkedRoot).Set("alpha", "token"); err == nil {
		t.Fatal("Set() accepted a symlinked Token directory")
	}
}

func TestBackendChoiceRejectsUnsafePermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend"), []byte("file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newBackendChoice(root).Read(); err == nil {
		t.Fatal("Read() accepted unsafe backend marker permissions")
	}
}

func TestFileStoreReadAndDeleteBoundaries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store := newFileStore(root)
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if err := store.Delete("missing"); err != nil {
		t.Fatalf("Delete() missing error = %v", err)
	}
	if mustExist(t, store, "missing") {
		t.Fatal("Has() reported a missing Token")
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if mustExist(t, store, "still-missing") {
		t.Fatal("Exists() reported an absent Token file")
	}
	if err := store.Delete("still-missing"); err != nil {
		t.Fatalf("Delete() absent Token error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("empty"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() empty error = %v, want ErrNotFound", err)
	}
	if err := store.Set("present", "token"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("present", "replacement"); err != nil {
		t.Fatalf("replace Token error = %v", err)
	}
	if !mustExist(t, store, "present") {
		t.Fatal("Has() did not report the stored Token")
	}
}

func TestFileStoreDeleteRejectsUnsafeStorage(t *testing.T) {
	parent := t.TempDir()
	unsafeRoot := filepath.Join(parent, "unsafe-root")
	if err := os.Mkdir(unsafeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := newFileStore(unsafeRoot).Delete("alpha"); err == nil {
		t.Fatal("Delete() accepted an unsafe Token directory")
	}

	root := filepath.Join(parent, "safe-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alpha"), []byte("token"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := newFileStore(root).Delete("alpha"); err == nil {
		t.Fatal("Delete() accepted an unsafe Token file")
	}
}

func TestFileStoreDeleteRejectsDirectoryAtTokenPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := newFileStore(root).Delete("alpha"); err == nil {
		t.Fatal("Delete() accepted a directory at the Token path")
	}
}

func TestFileStoreRejectsInvalidAccountNames(t *testing.T) {
	store := newFileStore(filepath.Join(t.TempDir(), "secrets"))
	if _, err := store.Get("invalid account"); err == nil {
		t.Fatal("Get() accepted an invalid Account ID")
	}
	if err := store.Set("invalid account", "token"); err == nil {
		t.Fatal("Set() accepted an invalid Account ID")
	}
	if err := store.Set("alpha", ""); err == nil {
		t.Fatal("Set() accepted an empty Token")
	}
	if err := store.Delete("invalid account"); err == nil {
		t.Fatal("Delete() accepted an invalid Account ID")
	}
}

func TestOwnedFileValidationRejectsAmbiguousOrForeignMetadata(t *testing.T) {
	uid := uint32(os.Geteuid())
	if err := validateOwnedFile(fileInfoStub{mode: 0o600, stat: &syscall.Stat_t{Uid: uid, Nlink: 2}}); err == nil {
		t.Fatal("validateOwnedFile() accepted a multiply-linked file")
	}
	if err := validateOwnedFile(fileInfoStub{mode: 0o600, stat: &syscall.Stat_t{Uid: uid + 1, Nlink: 1}}); err == nil {
		t.Fatal("validateOwnedFile() accepted a foreign owner")
	}
	if err := validateOwnedFile(fileInfoStub{mode: 0o600}); err == nil {
		t.Fatal("validateOwnedFile() accepted missing native metadata")
	}
}

func TestUnixDirectorySyncFailureIsExplicit(t *testing.T) {
	want := errors.New("injected failure")
	if err := syncDirectory(syncerStub{err: want}); !errors.Is(err, want) {
		t.Fatalf("syncDirectory() error = %v", err)
	}
}

func TestSecureRootRejectsForeignOwnerMetadata(t *testing.T) {
	info := fileInfoStub{
		mode: os.ModeDir | 0o700,
		stat: &syscall.Stat_t{Uid: uint32(os.Geteuid() + 1), Nlink: 1},
	}
	if err := validateSecureRoot(info); err == nil {
		t.Fatal("validateSecureRoot() accepted a foreign owner")
	}
}
