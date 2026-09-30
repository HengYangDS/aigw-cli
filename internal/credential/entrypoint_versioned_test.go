package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestVersionedEntrypointFailureRemovesCreatedDirectoryChain(t *testing.T) {
	for _, tc := range []struct {
		name            string
		preexistingData bool
		failedWrite     string
	}{
		{"new-data-executable", false, "executable"},
		{"new-data-receipt", false, "receipt"},
		{"existing-data-executable", true, "executable"},
		{"existing-data-receipt", true, "receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			data := filepath.Join(root, "data")
			if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.preexistingData {
				if err := os.Mkdir(data, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			target, err := VersionedEntrypointPath(data, source, "aigw")
			if err != nil {
				t.Fatal(err)
			}
			want := errors.New(tc.failedWrite + " write failed")
			write := func(path string, expected transaction.FileSnapshot, content []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
				if tc.failedWrite == "executable" || path == target+".sha256" {
					return transaction.FileSnapshot{}, want
				}
				return transaction.WriteFileAtomicExactModeIfUnchanged(path, expected, content, mode)
			}
			if _, err := ensureEntrypoint(source, target, write); !errors.Is(err, want) {
				t.Fatalf("entrypoint failure = %v, want %v", err, want)
			}
			for _, path := range []string{filepath.Dir(target), filepath.Dir(filepath.Dir(target))} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed installation retained created directory %s: %v", path, err)
				}
			}
			_, err = os.Lstat(data)
			if tc.preexistingData && err != nil || !tc.preexistingData && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("data directory after failure = %v; preexisting = %t", err, tc.preexistingData)
			}
		})
	}
}

func TestVersionedEntrypointRejectsWritableDataDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows data-directory rights are represented by ACLs")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	data := filepath.Join(root, "data")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	target, err := VersionedEntrypointPath(data, source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(data, 0o777); err != nil {
		t.Fatal(err)
	}
	if needed, err := EntrypointNeeded(target); err == nil {
		t.Fatalf("writable data directory was accepted as a reader plan: needed = %t", needed)
	}
	if _, err := EnsureEntrypoint(source, target); err == nil {
		t.Fatal("writable data directory received a credential reader")
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected data directory received executable bytes: %v", err)
	}
}

func TestVersionedEntrypointFailurePreservesReplacedDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	data := filepath.Join(root, "data")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	target, err := VersionedEntrypointPath(data, source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(target)
	want := errors.New("executable write failed")
	write := func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
		if err := os.Rename(parent, parent+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatal(err)
		}
		return transaction.FileSnapshot{}, want
	}
	if _, err := ensureEntrypoint(source, target, write); !errors.Is(err, want) {
		t.Fatalf("entrypoint failure = %v, want %v", err, want)
	}
	if info, err := os.Lstat(parent); err != nil || !info.IsDir() {
		t.Fatalf("replacement directory was removed: %v", err)
	}
}

func TestVersionedEntrypointUndoRemovesCreatedDirectoryChain(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	data := filepath.Join(root, "data")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	target, err := VersionedEntrypointPath(data, source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	undo, err := EnsureEntrypoint(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := undo(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(data); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback retained its created data directory: %v", err)
	}
}

func TestVersionedEntrypointRejectsChangedSourceBeforeCreatingBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	first := sha256.Sum256([]byte("first-version"))
	target := filepath.Join(root, "data", "credential", hex.EncodeToString(first[:]), "aigw")
	if err := os.WriteFile(source, []byte("second-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err == nil {
		t.Fatal("changed executable was installed under the predecessor digest")
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected successor left executable bytes: %v", err)
	}
}

func TestFixedEntrypointCannotAdoptSuccessorWithoutReplacingActiveBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	dataDir := filepath.Join(root, "data")
	fixed := filepath.Join(dataDir, "credential", "aigw")
	predecessor := []byte("first-version")
	if err := os.WriteFile(source, predecessor, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, fixed); err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(fixed + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	successor := []byte("second-version")
	if err := os.WriteFile(source, successor, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, fixed); err == nil {
		t.Fatal("fixed path silently claimed a newer reader without installing its bytes")
	}
	if current, err := os.ReadFile(fixed); err != nil || string(current) != string(predecessor) {
		t.Fatalf("active predecessor changed: %q, %v", current, err)
	}
	if current, err := os.ReadFile(fixed + ".sha256"); err != nil || string(current) != string(receipt) {
		t.Fatalf("active predecessor receipt changed: %q, %v", current, err)
	}
	versioned, err := VersionedEntrypointPath(dataDir, source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	if versioned == fixed {
		t.Fatal("successor reused the fixed path")
	}
	if _, err := EnsureEntrypoint(source, versioned); err != nil {
		t.Fatal(err)
	}
	if current, err := os.ReadFile(versioned); err != nil || string(current) != string(successor) {
		t.Fatalf("successor bytes were not prepared: %q, %v", current, err)
	}
}

func TestVersionedEntrypointRejectsBytesThatMatchOnlyTheirReceipt(t *testing.T) {
	root := t.TempDir()
	first := sha256.Sum256([]byte("first-version"))
	second := sha256.Sum256([]byte("second-version"))
	target := filepath.Join(root, "data", "credential", hex.EncodeToString(first[:]), "aigw")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("second-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".sha256", []byte(hex.EncodeToString(second[:])+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EntrypointNeeded(target); err == nil {
		t.Fatal("digest directory accepted different executable bytes")
	}
}

func TestVersionedEntrypointsKeepPredecessorBytesThroughSuccessorRemoval(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	data := filepath.Join(root, "data")
	var predecessor string
	for _, version := range []string{"first-version", "second-version"} {
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		path, err := VersionedEntrypointPath(data, source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureEntrypoint(source, path); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != version {
			t.Fatalf("installed version = %q, %v; want %q", got, err, version)
		}
		if version == "first-version" {
			predecessor = path
		} else if err := RemoveEntrypoint(path); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := os.ReadFile(predecessor); err != nil || string(got) != "first-version" {
		t.Fatalf("predecessor changed after successor removal: %q, %v", got, err)
	}
}

func TestRetainedEntrypointRequiresTheSameIntactVersionedNamespace(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	paths := make([]string, 0, 2)
	for _, version := range []string{"predecessor", "successor"} {
		source := filepath.Join(root, version)
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		path, err := VersionedEntrypointPath(data, source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	retained, current := paths[0], paths[1]
	if _, err := EnsureEntrypoint(filepath.Join(root, "predecessor"), retained); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRetainedEntrypoint(current, retained); err != nil {
		t.Fatalf("intact predecessor reader was rejected: %v", err)
	}
	foreign := filepath.Join(root, "foreign", "credential", filepath.Base(filepath.Dir(current)), "aigw")
	for _, path := range []string{foreign, filepath.Join(data, "credential", "aigw")} {
		if err := ValidateRetainedEntrypoint(path, retained); err == nil {
			t.Fatalf("reader in a different or unversioned namespace was accepted: %s", path)
		}
	}
	if err := os.WriteFile(retained+".sha256", []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRetainedEntrypoint(current, retained); err == nil {
		t.Fatal("drifted predecessor receipt was accepted")
	}
}

func TestRetainedEntrypointRejectsRedirectedCurrentNamespace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows native reparse-point fixtures require their own OS contract")
	}
	root := t.TempDir()
	paths := make([]string, 0, 2)
	for _, version := range []string{"predecessor", "successor"} {
		source := filepath.Join(root, version)
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		reader, err := VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, reader)
	}
	retained, current := paths[0], paths[1]
	if _, err := EnsureEntrypoint(filepath.Join(root, "predecessor"), retained); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, "foreign")
	if err := os.Mkdir(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	redirect := filepath.Join(foreign, "credential")
	if err := os.Symlink(filepath.Dir(filepath.Dir(retained)), redirect); err != nil {
		t.Fatal(err)
	}
	current = filepath.Join(redirect, filepath.Base(filepath.Dir(current)), "aigw")
	if err := ValidateRetainedEntrypoint(current, retained); err == nil {
		t.Fatal("a redirected current namespace was accepted as the owned installation")
	}
}
