package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

type readRootStub struct {
	info os.FileInfo
	err  error
}

func (root readRootStub) Lstat(string) (os.FileInfo, error) { return root.info, nil }
func (root readRootStub) Open(string) (*os.File, error)     { return nil, root.err }

type failingDirectorySyncRoot struct {
	*os.Root
	remainingFailures int
	onFailure         func()
	recoveryObserved  bool
}

type faultRoot struct {
	*os.Root
	lstatErr    error
	openFileErr error
	openFile    *os.File
	removeErr   error
	renameErr   error
	openErrors  map[string]error
	openedFile  *os.File
}

func (root *faultRoot) Lstat(name string) (os.FileInfo, error) {
	if root.lstatErr != nil {
		return nil, root.lstatErr
	}
	return root.Root.Lstat(name)
}

func (root *faultRoot) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if root.openFileErr != nil {
		return nil, root.openFileErr
	}
	if root.openFile != nil {
		return root.openFile, nil
	}
	return root.Root.OpenFile(name, flag, perm)
}

func (root *faultRoot) Remove(name string) error {
	if root.removeErr != nil {
		return root.removeErr
	}
	return root.Root.Remove(name)
}

func (root *faultRoot) Rename(oldName, newName string) error {
	if root.renameErr != nil {
		return root.renameErr
	}
	return root.Root.Rename(oldName, newName)
}

func (root *faultRoot) Open(name string) (*os.File, error) {
	if err := root.openErrors[name]; err != nil {
		return nil, err
	}
	if root.openedFile != nil {
		return root.openedFile, nil
	}
	return root.Root.Open(name)
}

func (root *failingDirectorySyncRoot) Open(name string) (*os.File, error) {
	if name == "." && !root.recoveryObserved {
		root.recoveryObserved = true
		return root.Root.Open(name)
	}
	if name == "." && root.remainingFailures > 0 {
		root.remainingFailures--
		if root.onFailure != nil {
			root.onFailure()
		}
		return nil, errors.New("directory sync failed")
	}
	return root.Root.Open(name)
}

func TestSecureFileHelpersRejectInvalidResources(t *testing.T) {
	if _, err := openSecureRoot("", false); err == nil {
		t.Fatal("openSecureRoot() accepted an empty storage root")
	}
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

func TestSecureFileTransactionsReportOwnedFilesystemFailures(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	want := errors.New("injected filesystem failure")

	if _, _, err := replaceSecureFile(&faultRoot{Root: opened, openFileErr: want}, "alpha", []byte("token")); !errors.Is(err, want) {
		t.Fatalf("replaceSecureFile() open error = %v, want %v", err, want)
	}
	if _, _, err := replaceSecureFile(&faultRoot{Root: opened, renameErr: want}, "alpha", []byte("token")); !errors.Is(err, want) {
		t.Fatalf("replaceSecureFile() rename error = %v, want %v", err, want)
	}
	if err := os.WriteFile(filepath.Join(directory, "alpha"), []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := deleteSecureFile(&faultRoot{Root: opened, removeErr: want}, "alpha"); !errors.Is(err, want) {
		t.Fatalf("deleteSecureFile() remove error = %v, want %v", err, want)
	}
	if _, err := secureFileExists(&faultRoot{Root: opened, lstatErr: want}, "alpha"); !errors.Is(err, want) {
		t.Fatalf("secureFileExists() inspection error = %v, want %v", err, want)
	}
	if _, err := captureOptionalSecureFile(&faultRoot{Root: opened, openErrors: map[string]error{"alpha": want}}, "alpha"); !errors.Is(err, want) {
		t.Fatalf("captureOptionalSecureFile() read error = %v, want %v", err, want)
	}
}

func TestGuardedDeleteReportsObservationRemovalAndSyncFailures(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	expected, err := captureOptionalSecureFile(opened, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(expected.value)
	want := errors.New("injected filesystem failure")

	if err := deleteSecureFileIf(&faultRoot{Root: opened, lstatErr: want}, "alpha", expected); !errors.Is(err, want) {
		t.Fatalf("deleteSecureFileIf() observation error = %v, want %v", err, want)
	}
	if err := deleteSecureFileIf(&faultRoot{Root: opened, removeErr: want}, "alpha", expected); !errors.Is(err, want) {
		t.Fatalf("deleteSecureFileIf() removal error = %v, want %v", err, want)
	}
	root := &failingDirectorySyncRoot{Root: opened, remainingFailures: 1}
	if err := deleteSecureFileIf(root, "alpha", expected); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("deleteSecureFileIf() sync error = %v", err)
	}
	value, err := os.ReadFile(path)
	if err != nil || string(value) != "token" {
		t.Fatalf("Token after guarded delete compensation = %q, %v", value, err)
	}
}

func TestRestoreSecureFileReportsCompensationFailures(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("postimage"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	postimage, err := captureOptionalSecureFile(opened, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(postimage.value)
	cause := errors.New("commit failed")
	want := errors.New("injected compensation failure")

	if err := restoreSecureFile(&faultRoot{Root: opened, lstatErr: want}, "alpha", credentialFileSnapshot{}, postimage, cause); err == nil || !strings.Contains(err.Error(), "inspect Token postimage") {
		t.Fatalf("restoreSecureFile() observation error = %v", err)
	}
	if err := restoreSecureFile(&faultRoot{Root: opened, removeErr: want}, "alpha", credentialFileSnapshot{}, postimage, cause); err == nil || !strings.Contains(err.Error(), "remove uncommitted Token") {
		t.Fatalf("restoreSecureFile() removal error = %v", err)
	}
	if err := restoreSecureFile(&faultRoot{Root: opened, openErrors: map[string]error{".": want}}, "alpha", credentialFileSnapshot{}, postimage, cause); err == nil || !strings.Contains(err.Error(), "sync restored Token state") {
		t.Fatalf("restoreSecureFile() sync error = %v", err)
	}
	if err := os.WriteFile(path, []byte("postimage"), 0o600); err != nil {
		t.Fatal(err)
	}
	postimage, err = captureOptionalSecureFile(opened, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	preimage := credentialFileSnapshot{value: []byte("preimage"), exists: true}
	if err := restoreSecureFile(&faultRoot{Root: opened, openFileErr: want}, "alpha", preimage, postimage, cause); err == nil || !strings.Contains(err.Error(), "restore previous Token") {
		t.Fatalf("restoreSecureFile() replacement error = %v", err)
	}
}

func TestCredentialSnapshotComparisonSupportsContentOnlyExpectations(t *testing.T) {
	left := credentialFileSnapshot{value: []byte("token"), exists: true}
	right := credentialFileSnapshot{value: []byte("token"), exists: true}
	if !sameCredentialFileSnapshot(left, right) {
		t.Fatal("sameCredentialFileSnapshot() rejected equal content-only snapshots")
	}
}

func TestSecureFilePropagatesOpenAndWriteFailures(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "alpha")
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
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if err := writeSecureFile(&faultRoot{Root: root, openFile: readOnly}, "alpha", []byte("replacement")); err == nil {
		t.Fatal("writeSecureFile() ignored a write failure")
	}
}

func TestWriteSecureFileRestoresPreimageWhenDirectorySyncFails(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("old-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	root := &failingDirectorySyncRoot{Root: opened, remainingFailures: 1}
	if err := writeSecureFile(root, "alpha", []byte("new-token")); err == nil {
		t.Fatal("writeSecureFile() ignored a post-rename directory sync failure")
	}
	value, err := os.ReadFile(path)
	if err != nil || string(value) != "old-token" {
		t.Fatalf("Token after failed replacement = %q, %v; want old-token", value, err)
	}
	temporary, err := filepath.Glob(filepath.Join(directory, ".token-*"))
	if err != nil || len(temporary) != 0 {
		t.Fatalf("temporary files after failed replacement = %#v, %v", temporary, err)
	}
}

func TestWriteSecureFileRemovesCreatedPostimageWhenDirectorySyncFails(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	root := &failingDirectorySyncRoot{Root: opened, remainingFailures: 1}
	if err := writeSecureFile(root, "alpha", []byte("new-token")); err == nil {
		t.Fatal("writeSecureFile() ignored a post-rename directory sync failure")
	}
	if _, err := os.Stat(filepath.Join(directory, "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new Token remains after failed write: %v", err)
	}
}

func TestDeleteSecureFileRestoresPreimageWhenDirectorySyncFails(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("old-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	root := &failingDirectorySyncRoot{Root: opened, remainingFailures: 1}
	if err := deleteSecureFile(root, "alpha"); err == nil {
		t.Fatal("deleteSecureFile() ignored a post-removal directory sync failure")
	}
	value, err := os.ReadFile(path)
	if err != nil || string(value) != "old-token" {
		t.Fatalf("Token after failed deletion = %q, %v; want old-token", value, err)
	}
}

func TestWriteSecureFilePreservesNewerValueWhenCompensationObservesDrift(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("old-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	root := &failingDirectorySyncRoot{
		Root:              opened,
		remainingFailures: 1,
		onFailure: func() {
			if writeErr := os.WriteFile(path, []byte("newer-token"), 0o600); writeErr != nil {
				t.Fatalf("inject newer Token: %v", writeErr)
			}
		},
	}
	err = writeSecureFile(root, "alpha", []byte("transaction-token"))
	if err == nil || !strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("writeSecureFile() error = %v, want postimage drift", err)
	}
	value, readErr := os.ReadFile(path)
	if readErr != nil || string(value) != "newer-token" {
		t.Fatalf("Token after guarded compensation = %q, %v; want newer-token", value, readErr)
	}
}

func TestDeleteSecureFilePreservesRecreatedValueWhenCompensationObservesDrift(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("old-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	root := &failingDirectorySyncRoot{
		Root:              opened,
		remainingFailures: 1,
		onFailure: func() {
			if writeErr := os.WriteFile(path, []byte("newer-token"), 0o600); writeErr != nil {
				t.Fatalf("inject recreated Token: %v", writeErr)
			}
		},
	}
	err = deleteSecureFile(root, "alpha")
	if err == nil || !strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("deleteSecureFile() error = %v, want postimage drift", err)
	}
	value, readErr := os.ReadFile(path)
	if readErr != nil || string(value) != "newer-token" {
		t.Fatalf("Token after guarded compensation = %q, %v; want newer-token", value, readErr)
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

func TestWriteAndSyncFailuresAreExplicit(t *testing.T) {
	want := errors.New("injected failure")
	if err := writeAndSync(syncWriterStub{writeErr: want}, []byte("token")); !errors.Is(err, want) {
		t.Fatalf("writeAndSync() write error = %v", err)
	}
	if err := writeAndSync(syncWriterStub{syncErr: want}, []byte("token")); !errors.Is(err, want) {
		t.Fatalf("writeAndSync() sync error = %v", err)
	}
}

type boundedObservationRoot struct {
	*os.Root
	lstatCalls int
}

func (root *boundedObservationRoot) Lstat(name string) (os.FileInfo, error) {
	root.lstatCalls++
	if root.lstatCalls > 2 {
		return nil, errors.New("credential state was observed more than once")
	}
	return root.Root.Lstat(name)
}

func TestGuardedDeleteUsesOneValidatedSnapshot(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, backendChoiceName)
	if err := os.WriteFile(path, []byte("file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	root := &boundedObservationRoot{Root: opened}
	expected, err := captureOptionalSecureFile(opened, backendChoiceName)
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteSecureFileIf(root, backendChoiceName, expected); err != nil {
		t.Fatalf("deleteSecureFileIf() error = %v", err)
	}
	if root.lstatCalls != 2 {
		t.Fatalf("Lstat calls = %d, want one validated snapshot", root.lstatCalls)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backend selection remains after guarded deletion: %v", err)
	}
}
