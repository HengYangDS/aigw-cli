//go:build !darwin && !linux

package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func privateRecoveryRootForTest(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "recovery")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFallbackRecoveryMutationsEnforceMissingAndChangedSnapshots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "recovery")
	path := filepath.Join(root, "air", "ledger.json")
	missing := transaction.FileSnapshot{}
	desired := desiredSnapshot([]byte("ledger\n"), 0o600)

	if got, err := removeRecoveryFileIfUnchanged(root, path, missing); err != nil || got.Exists {
		t.Fatalf("remove missing file = %#v, %v", got, err)
	}
	if _, err := removeRecoveryFileIfUnchanged(root, path, desired); err == nil {
		t.Fatal("remove accepted a missing file as the expected postimage")
	}
	if err := restoreRecoveryFileAtomicIfPostimage(root, path, missing, missing); err != nil {
		t.Fatalf("restore missing to missing: %v", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing rollback created recovery root: %v", err)
	}
	if err := restoreRecoveryFileAtomicIfPostimage(root, path, missing, desired); err == nil {
		t.Fatal("restore accepted a missing file as a present postimage")
	}

	written, err := writeRecoveryFileAtomicIfUnchanged(root, path, missing, desired.Data, desired.Mode)
	if err != nil || !sameRecoverySnapshot(written, desired) {
		t.Fatalf("initial write = %#v, %v", written, err)
	}
	if _, err := writeRecoveryFileAtomicIfUnchanged(root, path, missing, []byte("new\n"), 0o600); err == nil {
		t.Fatal("write accepted a changed preimage")
	}
	if _, err := removeRecoveryFileIfUnchanged(root, path, missing); err == nil {
		t.Fatal("remove accepted a changed preimage")
	}
	if err := restoreRecoveryFileAtomicIfPostimage(root, path, missing, desiredSnapshot([]byte("other\n"), 0o600)); err == nil {
		t.Fatal("restore accepted a changed postimage")
	}
}

func TestFallbackRecoveryMutationsRejectNonRegularTargets(t *testing.T) {
	for _, operation := range []string{"write", "remove", "restore"} {
		t.Run(operation, func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			path := filepath.Join(root, "air", "ledger.json")
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "write":
				_, err = writeRecoveryFileAtomicIfUnchanged(root, path, transaction.FileSnapshot{}, []byte("data"), 0o600)
			case "remove":
				_, err = removeRecoveryFileIfUnchanged(root, path, transaction.FileSnapshot{})
			case "restore":
				err = restoreRecoveryFileAtomicIfPostimage(root, path, transaction.FileSnapshot{}, transaction.FileSnapshot{})
			}
			if err == nil {
				t.Fatalf("%s accepted a directory target", operation)
			}
		})
	}
}

func TestFallbackRecoveryMutationsRejectPathEscape(t *testing.T) {
	for _, operation := range []string{"write", "remove", "restore"} {
		t.Run(operation, func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			outside := filepath.Join(filepath.Dir(root), "outside.json")
			path := filepath.Join(root, "..", "outside.json")
			var err error
			switch operation {
			case "write":
				_, err = writeRecoveryFileAtomicIfUnchanged(root, path, transaction.FileSnapshot{}, []byte("data"), 0o600)
			case "remove":
				if writeErr := os.WriteFile(outside, []byte("data"), 0o600); writeErr != nil {
					t.Fatal(writeErr)
				}
				_, err = removeRecoveryFileIfUnchanged(root, path, desiredSnapshot([]byte("data"), 0o600))
			case "restore":
				if writeErr := os.WriteFile(outside, []byte("data"), 0o600); writeErr != nil {
					t.Fatal(writeErr)
				}
				err = restoreRecoveryFileAtomicIfPostimage(root, path, transaction.FileSnapshot{}, desiredSnapshot([]byte("data"), 0o600))
			}
			if err == nil {
				t.Fatalf("%s followed a path that escapes its root", operation)
			}
			_, statErr := os.Stat(outside)
			switch operation {
			case "write":
				if !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("escaping write created content outside its root: %v", statErr)
				}
			default:
				if statErr != nil {
					t.Fatalf("escaping mutation removed content outside its root: %v", statErr)
				}
			}
		})
	}
}

func TestValidateRecoveryMutationParentsCreatesMissingDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "recovery")
	path := filepath.Join(root, "air", "quarantine", "case", "config.toml")
	if err := validateRecoveryMutationParents(root, path, true); err != nil {
		t.Fatalf("validateRecoveryMutationParents: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, "air", "quarantine", "case"))
	if err != nil || !info.IsDir() {
		t.Fatalf("did not create the missing parent chain: %v", err)
	}
}

func TestValidateRecoveryMutationParentsRejectsRegularFileComponent(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run("", func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			blocked := filepath.Join(root, "blocked")
			if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(blocked, "ledger.json")
			if err := validateRecoveryMutationParents(root, path, create); err == nil {
				t.Fatalf("create=%v: accepted a regular file as a recovery parent", create)
			}
		})
	}
}

func TestValidateRecoveryMutationParentsRejectsLinkedComponent(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "recovery")
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	createRecoveryLinkForTest(t, outside, link)
	for _, create := range []bool{false, true} {
		if err := validateRecoveryMutationParents(root, filepath.Join(link, "ledger.json"), create); err == nil {
			t.Fatalf("create=%v: followed a linked recovery parent", create)
		}
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("linked target changed: %#v, %v", entries, err)
	}
}

func TestValidateRecoveryMutationParentsMissingWithoutCreateFails(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	path := filepath.Join(root, "missing", "ledger.json")
	if err := validateRecoveryMutationParents(root, path, false); err == nil {
		t.Fatal("accepted a missing parent without creating it")
	}
}
