//go:build !windows

package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

type syncWriterStub struct {
	writeErr error
	syncErr  error
}

func (writer syncWriterStub) Write([]byte) (int, error) {
	if writer.writeErr != nil {
		return 0, writer.writeErr
	}
	return 1, nil
}

func (writer syncWriterStub) Sync() error { return writer.syncErr }

type syncerStub struct{ err error }

func (syncer syncerStub) Sync() error { return syncer.err }

type readRootStub struct {
	info os.FileInfo
	err  error
}

func (root readRootStub) Lstat(string) (os.FileInfo, error) { return root.info, nil }
func (root readRootStub) Open(string) (*os.File, error)     { return nil, root.err }

type writeRootStub struct{ file *os.File }

func (root writeRootStub) OpenFile(string, int, os.FileMode) (*os.File, error) {
	return root.file, nil
}
func (writeRootStub) Remove(string) error         { return nil }
func (writeRootStub) Rename(string, string) error { return nil }
func (writeRootStub) Open(string) (*os.File, error) {
	return nil, errors.New("unexpected directory sync")
}

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
	if store.Has("missing") {
		t.Fatal("Has() reported a missing Token")
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
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
	if !store.Has("present") {
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

func TestBackendChoiceLifecycleAndValidation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	choice := newBackendChoice(root)
	if _, err := choice.Read(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read() missing error = %v", err)
	}
	if err := choice.Write("unknown"); err == nil {
		t.Fatal("Write() accepted an unknown backend")
	}
	if err := choice.Write("file"); err != nil {
		t.Fatalf("Write() error = %v", err)
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
}

func TestBackendChoiceReportsAtomicWriteFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "backend"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := newBackendChoice(root).Write("file"); err == nil {
		t.Fatal("Write() replaced a backend directory")
	}
}

func TestSecureFileHelpersRejectInvalidResources(t *testing.T) {
	parent := t.TempDir()
	regular := filepath.Join(parent, "regular")
	if err := os.WriteFile(regular, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openSecureRoot(regular, false); err == nil {
		t.Fatal("openSecureRoot() accepted a regular file")
	}
	if _, err := openSecureRoot(filepath.Join(regular, "child"), true); err == nil {
		t.Fatal("openSecureRoot() created beneath a regular file")
	}
	if _, err := openSecureRoot(filepath.Join(regular, "child"), false); err == nil {
		t.Fatal("openSecureRoot() inspected beneath a regular file")
	}

	rootPath := filepath.Join(parent, "root")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rootPath, "occupied"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeSecureFile(root, "occupied", []byte("token")); err == nil {
		t.Fatal("writeSecureFile() replaced a directory")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecureFile(root, "missing"); err == nil {
		t.Fatal("readSecureFile() used a closed root")
	}
	if err := writeSecureFile(root, "alpha", []byte("token")); err == nil {
		t.Fatal("writeSecureFile() used a closed root")
	}
	if err := syncRoot(root); err == nil {
		t.Fatal("syncRoot() used a closed root")
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

func TestSecureFilePrimitiveFailuresAreExplicit(t *testing.T) {
	want := errors.New("injected failure")
	if err := writeAndSync(syncWriterStub{writeErr: want}, []byte("token")); !errors.Is(err, want) {
		t.Fatalf("writeAndSync() write error = %v", err)
	}
	if err := writeAndSync(syncWriterStub{syncErr: want}, []byte("token")); !errors.Is(err, want) {
		t.Fatalf("writeAndSync() sync error = %v", err)
	}
	if err := syncDirectory(syncerStub{err: want}); !errors.Is(err, want) {
		t.Fatalf("syncDirectory() error = %v", err)
	}
}

func TestSecureFilePropagatesOpenAndWriteFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alpha")
	if err := os.WriteFile(path, []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("injected open failure")
	if _, err := readSecureFile(readRootStub{info: info, err: want}, "alpha"); !errors.Is(err, want) {
		t.Fatalf("readSecureFile() error = %v, want %v", err, want)
	}
	readOnly, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSecureFile(writeRootStub{file: readOnly}, "alpha", []byte("replacement")); err == nil {
		t.Fatal("writeSecureFile() ignored a write failure")
	}
}

func TestSecureRootAndFileIdentityAreStable(t *testing.T) {
	parent := t.TempDir()
	first := filepath.Join(parent, "first")
	second := filepath.Join(parent, "second")
	if err := os.Mkdir(first, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(second, 0o700); err != nil {
		t.Fatal(err)
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openVerifiedRoot(filepath.Join(parent, "missing"), firstInfo); err == nil {
		t.Fatal("openVerifiedRoot() accepted a missing root")
	}
	if _, err := openVerifiedRoot(second, firstInfo); err == nil {
		t.Fatal("openVerifiedRoot() accepted a changed root")
	}

	root, err := os.OpenRoot(first)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if _, err := openVerifiedFile(root, "missing", firstInfo); err == nil {
		t.Fatal("openVerifiedFile() accepted a missing file")
	}
	for _, name := range []string{"alpha", "beta"} {
		if err := os.WriteFile(filepath.Join(first, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	alpha, err := os.Stat(filepath.Join(first, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openVerifiedFile(root, "beta", alpha); err == nil {
		t.Fatal("openVerifiedFile() accepted a changed file")
	}
	if _, err := readSecureFile(root, "beta"); err != nil {
		t.Fatalf("readSecureFile() error = %v", err)
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
