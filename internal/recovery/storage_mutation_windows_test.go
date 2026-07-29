//go:build windows

package recovery

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestWindowsRecoverySnapshotModeMatchesFilesystem(t *testing.T) {
	for _, mode := range []os.FileMode{0o600, 0o400, 0o020} {
		t.Run(mode.String(), func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			path := filepath.Join(root, "ledger.json")
			data := []byte("ledger\n")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			actual, err := captureRecoveryFileNoFollow(root, path)
			if err != nil {
				t.Fatal(err)
			}
			want := desiredSnapshot(data, mode)
			if !sameRecoverySnapshot(actual, want) {
				t.Fatalf("snapshot = %#v, want %#v", actual, want)
			}
		})
	}
}

func TestWindowsFallbackRecoveryMutationsRejectLockedTarget(t *testing.T) {
	for _, operation := range []string{"write", "remove", "restore"} {
		t.Run(operation, func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			path := filepath.Join(root, "ledger.json")
			data := []byte("original\n")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			expected, err := captureRecoveryFileNoFollow(root, path)
			if err != nil {
				t.Fatal(err)
			}
			release := makeRecoveryPathUnreadableForTest(t, path)
			switch operation {
			case "write":
				_, err = writeRecoveryFileAtomicIfUnchanged(root, path, expected, []byte("changed\n"), 0o600)
			case "remove":
				_, err = removeRecoveryFileIfUnchanged(root, path, expected)
			case "restore":
				err = restoreRecoveryFileAtomicIfPostimage(root, path, transaction.FileSnapshot{}, expected)
			}
			if err == nil {
				t.Fatalf("%s mutated a target held without sharing", operation)
			}
			release()
			current, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(current, data) {
				t.Fatalf("locked target changed: %q, %v", current, readErr)
			}
		})
	}
}

func TestValidateRecoveryMutationParentsAcceptsWindowsDirectoryModes(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	parent := filepath.Join(root, "air")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateRecoveryMutationParents(root, filepath.Join(parent, "ledger.json"), false); err != nil {
		t.Fatalf("rejected a regular Windows recovery directory: %v", err)
	}
}

func TestReadRecoveryDirectoryNoFollowRejectsWindowsSharingViolation(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	directory := filepath.Join(root, "inventory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	release := makeRecoveryPathUnreadableForTest(t, directory)
	entries, info, err := readRecoveryDirectoryNoFollow(root, directory)
	if err == nil || entries != nil || info != nil {
		t.Fatalf("locked directory = %#v, %v, %v", entries, info, err)
	}
	release()
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("locked directory was changed: %v", err)
	}
}
