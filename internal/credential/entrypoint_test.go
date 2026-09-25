package credential

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestEntrypointCopiesOnceAndRollsBackOnlyItsOwnBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("first-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	undo, err := EnsureEntrypoint(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("first-version")) {
		t.Fatalf("entrypoint bytes = %q, %v", got, err)
	}
	if err := os.WriteFile(source, []byte("second-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	noOpUndo, err := EnsureEntrypoint(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := noOpUndo(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("first-version")) {
		t.Fatalf("ordinary sync replaced the retained entrypoint: %q, %v", got, err)
	}
	if err := undo(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("compensation retained an unconsumed entrypoint: %v", err)
	}
}

func TestEntrypointRejectsUnknownOrChangedBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("foreign"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err == nil {
		t.Fatal("unknown executable was adopted as the AIGW credential entrypoint")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("changed-after-install"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EntrypointNeeded(target); err == nil {
		t.Fatal("changed credential executable passed its retained identity check")
	}
}

func TestEntrypointRejectsPermissiveDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ACLs are not represented by POSIX permission bits")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	parent := filepath.Join(root, "data", "credential")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, filepath.Join(parent, "aigw")); err == nil {
		t.Fatal("permissive credential directory was accepted")
	}
}

func TestEntrypointRejectsWritableDataDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows data-directory rights are represented by ACLs")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	data := filepath.Join(root, "data")
	target := filepath.Join(data, "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(data, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err == nil {
		t.Fatal("writable data directory was accepted as a credential entrypoint parent")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("rejected ancestor received credential executable: %v", err)
	}
}

func TestEntrypointRejectsMultiplyLinkedExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file identity is validated through ACL and reparse checks")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, "other-name")); err != nil {
		t.Fatal(err)
	}
	if _, err := EntrypointNeeded(target); err == nil {
		t.Fatal("multiply linked credential executable passed ownership validation")
	}
}

func TestEntrypointRemovalRejectsChangedBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("later-user-edit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEntrypoint(target); err == nil {
		t.Fatal("changed credential entrypoint was removed")
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("later-user-edit")) {
		t.Fatalf("changed credential entrypoint was not preserved: %q, %v", got, err)
	}
}

func TestReceiptWriteFailureRemovesOnlyNewCredentialEntrypoint(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := errors.New("receipt write failed")
	write := func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if path == target+".sha256" {
			return transaction.FileSnapshot{}, want
		}
		return transaction.WriteFileAtomicExactModeIfUnchanged(path, expected, data, mode)
	}
	if _, err := ensureEntrypoint(source, target, write); !errors.Is(err, want) {
		t.Fatalf("receipt failure = %v, want %v", err, want)
	}
	for _, path := range []string{target, target + ".sha256", filepath.Dir(target)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed receipt write retained %s: %v", path, err)
		}
	}
}

func TestExecutableWriteFailureLeavesNoCredentialEntrypoint(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := errors.New("executable write failed")
	write := func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
		return transaction.FileSnapshot{}, want
	}
	if _, err := ensureEntrypoint(source, target, write); !errors.Is(err, want) {
		t.Fatalf("executable failure = %v, want %v", err, want)
	}
	if _, err := os.Lstat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatalf("failed executable write retained credential directory: %v", err)
	}
}

func TestEntrypointRejectsIncompleteOrDriftedReceipt(t *testing.T) {
	for _, state := range []string{"missing", "drifted"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "data", "credential", "aigw")
			if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := EnsureEntrypoint(source, target); err != nil {
				t.Fatal(err)
			}
			receipt := target + ".sha256"
			if state == "missing" {
				if err := os.Remove(receipt); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(receipt, []byte("wrong digest\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := EntrypointNeeded(target); err == nil {
				t.Fatalf("%s receipt was admitted", state)
			}
		})
	}
}

func TestEntrypointVerificationFailureCompensatesItsOwnBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if path == target+".sha256" {
			data = []byte("wrong digest\n")
		}
		return transaction.WriteFileAtomicExactModeIfUnchanged(path, expected, data, mode)
	}
	if _, err := ensureEntrypoint(source, target, write); err == nil {
		t.Fatal("entrypoint verification accepted a mismatched receipt")
	}
	for _, path := range []string{target, target + ".sha256", filepath.Dir(target)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed verification retained %s: %v", path, err)
		}
	}
}

func TestEntrypointCompensationPreservesReceiptIfExecutableChanged(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	undo, err := EnsureEntrypoint(source, target)
	if err != nil {
		t.Fatal(err)
	}
	receipt := target + ".sha256"
	before, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("changed-after-creation"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := undo(); err == nil {
		t.Fatal("compensation removed a changed executable")
	}
	if got, err := os.ReadFile(receipt); err != nil || !bytes.Equal(got, before) {
		t.Fatalf("failed compensation lost the executable identity record: %q, %v", got, err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("changed-after-creation")) {
		t.Fatalf("failed compensation changed newer executable bytes: %q, %v", got, err)
	}
}

func TestEntrypointRejectsNonAbsoluteAndMissingSource(t *testing.T) {
	if _, err := EntrypointNeeded("relative/aigw"); err == nil {
		t.Fatal("relative credential entrypoint was accepted")
	}
	root := t.TempDir()
	target := filepath.Join(root, "data", "credential", "aigw")
	if _, err := EnsureEntrypoint(filepath.Join(root, "missing-source"), target); err == nil {
		t.Fatal("missing source was accepted")
	}
	source := filepath.Join(root, "empty-source")
	if err := os.WriteFile(source, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err == nil {
		t.Fatal("empty source was accepted")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("rejected source created credential executable: %v", err)
	}
}

func TestEntrypointPlansCreationBelowExistingDataDirectory(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if needed, err := EntrypointNeeded(filepath.Join(data, "credential", "aigw")); err != nil || !needed {
		t.Fatalf("missing credential directory plan = %v, %v; want creation", needed, err)
	}
}

func TestEntrypointRemovalIsNoOpWhenAbsent(t *testing.T) {
	if err := RemoveEntrypoint(""); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEntrypoint(filepath.Join(t.TempDir(), "data", "credential", "aigw")); err != nil {
		t.Fatal(err)
	}
}

func TestEntrypointCompensationPreservesChangedReceipt(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	undo, err := EnsureEntrypoint(source, target)
	if err != nil {
		t.Fatal(err)
	}
	receipt := target + ".sha256"
	if err := os.WriteFile(receipt, []byte("later edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := undo(); err == nil {
		t.Fatal("compensation removed a changed receipt")
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("verified-source")) {
		t.Fatalf("changed receipt caused executable deletion: %q, %v", got, err)
	}
	if got, err := os.ReadFile(receipt); err != nil || !bytes.Equal(got, []byte("later edit\n")) {
		t.Fatalf("compensation changed newer receipt bytes: %q, %v", got, err)
	}
}

func TestEntrypointFailurePreservesUnownedDirectoryContent(t *testing.T) {
	for _, phase := range []string{"preexisting", "concurrent"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "data", "credential", "aigw")
			parent := filepath.Dir(target)
			foreign := filepath.Join(parent, "user-note")
			if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
				t.Fatal(err)
			}
			if phase == "preexisting" {
				if err := os.MkdirAll(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(foreign, []byte("preserve me"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want := errors.New("executable write failed")
			write := func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
				if phase == "concurrent" {
					if err := os.WriteFile(foreign, []byte("preserve me"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return transaction.FileSnapshot{}, want
			}
			if _, err := ensureEntrypoint(source, target, write); !errors.Is(err, want) {
				t.Fatalf("entrypoint failure = %v, want %v", err, want)
			}
			if got, err := os.ReadFile(foreign); err != nil || !bytes.Equal(got, []byte("preserve me")) {
				t.Fatalf("failed installation changed unrelated content: %q, %v", got, err)
			}
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				t.Fatalf("failed installation created credential executable: %v", err)
			}
		})
	}
}

func TestEntrypointRejectsNonregularPaths(t *testing.T) {
	tests := []struct {
		name   string
		create func(string) error
	}{
		{"executable", func(target string) error {
			if err := os.Mkdir(target, 0o700); err != nil {
				return err
			}
			return os.WriteFile(target+".sha256", []byte("digest"), 0o600)
		}},
		{"receipt", func(target string) error {
			if err := os.WriteFile(target, []byte("executable"), 0o700); err != nil {
				return err
			}
			return os.Mkdir(target+".sha256", 0o700)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "data", "credential", "aigw")
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := test.create(target); err != nil {
				t.Fatal(err)
			}
			if _, err := EntrypointNeeded(target); err == nil {
				t.Fatalf("nonregular %s was accepted", test.name)
			}
		})
	}
}

func TestEntrypointRejectsPermissiveFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows executable rights are represented by ACLs")
	}
	for _, subject := range []string{"executable", "receipt"} {
		t.Run(subject, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "data", "credential", "aigw")
			if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := EnsureEntrypoint(source, target); err != nil {
				t.Fatal(err)
			}
			path := target
			if subject == "receipt" {
				path += ".sha256"
			}
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := EntrypointNeeded(target); err == nil {
				t.Fatalf("permissive %s mode was accepted", subject)
			}
		})
	}
}

func TestEntrypointRemovalPreservesUnrelatedDirectoryContent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("verified-source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(filepath.Dir(target), "user-note")
	if err := os.WriteFile(foreign, []byte("preserve me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEntrypoint(target); err != nil {
		t.Fatalf("removing owned files should preserve foreign content: %v", err)
	}
	for _, path := range []string{target, target + ".sha256"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("uninstall retained owned file %s: %v", path, err)
		}
	}
	if got, err := os.ReadFile(foreign); err != nil || !bytes.Equal(got, []byte("preserve me")) {
		t.Fatalf("uninstall changed unrelated content: %q, %v", got, err)
	}
}
