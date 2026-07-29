package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestNewStoreRemovesExternalFileThroughTransaction(t *testing.T) {
	base := t.TempDir()
	store := NewStore(filepath.Join(base, "recovery"))
	path := filepath.Join(base, "external.toml")
	if err := os.WriteFile(path, []byte("external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	post, err := store.remove(path, expected)
	if err != nil || post.Exists {
		t.Fatalf("remove external file = %#v, %v", post, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("external file remains: %v", err)
	}
}

func TestCaptureAirLedgerRejectsSymlinkWithoutFollowingIt(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := os.MkdirAll(filepath.Dir(store.airLedgerPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte("private target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, store.airLedgerPath()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.captureAirLedger(); err == nil || !strings.Contains(err.Error(), "read Air recovery ledger") {
		t.Fatalf("capture error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "private target" {
		t.Fatalf("symlink target changed: %q, %v", got, err)
	}
}

func TestDecodeAirLedgerRejectsTrailingJSON(t *testing.T) {
	data, err := encodeAirLedger(validPreparedAirLedgerForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAirLedger(append(data, []byte("{}\n")...)); err == nil || !strings.Contains(err.Error(), "trailing content") {
		t.Fatalf("decode error = %v", err)
	}
}

func TestValidateAirRecoveryStorageRejectsUnsafeFiles(t *testing.T) {
	store := NewStore(t.TempDir())
	quarantine := desiredSnapshot([]byte("quarantine"), 0o600)
	if err := store.validateAirRecoveryStorage("air-000001-deadbeefcafe", desiredSnapshot([]byte("ledger"), 0o644), quarantine); err == nil {
		t.Fatal("accepted an unsafe ledger mode")
	}
	if err := store.validateAirRecoveryStorage("air-000001-deadbeefcafe", transaction.FileSnapshot{}, transaction.FileSnapshot{}); err == nil {
		t.Fatal("accepted a missing quarantine")
	}
}

func TestInspectAirRecoveryStorageRejectsSpecialEntries(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, root string)
	}{
		{
			name: "Air root is regular file",
			prepare: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "air"), []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "ledger is directory",
			prepare: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "air", "ledger.json"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "quarantine root is regular file",
			prepare: func(t *testing.T, root string) {
				if err := os.Mkdir(filepath.Join(root, "air"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "air", "quarantine"), []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.prepare(t, root)
			inventory, err := NewStore(root).inspectAirRecoveryStorage()
			if err != nil {
				t.Fatal(err)
			}
			if !inventory.permissionInvalid || !inventory.unsafeTraversal {
				t.Fatalf("inventory = %#v", inventory)
			}
		})
	}
}

func TestAirRecoveryStorageInventoryHelpersRejectUnexpectedEntries(t *testing.T) {
	inventory := airRecoveryStorageInventory{quarantineEntries: []airRecoveryQuarantineEntry{
		{name: "other-case", directory: true},
		{name: "active-case", directory: true, files: []airRecoveryQuarantineFile{{name: "extra", regular: true}}},
	}}
	if exists, regular := inventory.quarantineFile("active-case", "config.toml"); exists || regular {
		t.Fatalf("missing config reported as %v, %v", exists, regular)
	}
	if !inventory.hasUnexpectedQuarantine("active-case", true) {
		t.Fatal("unexpected quarantine entries were accepted")
	}
	configOnly := airRecoveryStorageInventory{quarantineEntries: []airRecoveryQuarantineEntry{
		{name: "active-case", directory: true, files: []airRecoveryQuarantineFile{{name: "config.toml", regular: true}}},
	}}
	if !configOnly.hasUnexpectedQuarantine("active-case", false) {
		t.Fatal("settled inventory accepted a remaining config")
	}
}

func TestValidateAirLedgerRejectsAdditionalInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*airLedger)
	}{
		{name: "unsupported identity", mutate: func(ledger *airLedger) { ledger.SurfaceID = "foreign" }},
		{name: "invalid digest", mutate: func(ledger *airLedger) { ledger.ProjectionFingerprintSHA256 = "not-a-digest" }},
		{name: "missing creation time", mutate: func(ledger *airLedger) { ledger.CreatedAt = time.Time{} }},
		{name: "invalid roundtrip digest", mutate: func(ledger *airLedger) { ledger.ObservedRoundtripSHA256 = "short" }},
		{name: "unsupported state", mutate: func(ledger *airLedger) { ledger.State = "foreign" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := validPreparedAirLedgerForTest()
			tt.mutate(&ledger)
			if err := validateAirLedger(ledger); err == nil {
				t.Fatalf("accepted ledger %#v", ledger)
			}
		})
	}
}

func TestInspectAirRecoveryStorageHandlesUnreadableDirectories(t *testing.T) {
	tests := []struct {
		name string
		path func(root string) string
	}{
		{name: "Air root", path: func(root string) string { return filepath.Join(root, "air") }},
		{name: "quarantine root", path: func(root string) string { return filepath.Join(root, "air", "quarantine") }},
		{name: "case root", path: func(root string) string { return filepath.Join(root, "air", "quarantine", "case") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := tt.path(root)
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
			inventory, err := NewStore(root).inspectAirRecoveryStorage()
			if err != nil {
				t.Fatal(err)
			}
			if !inventory.unsafeTraversal {
				t.Fatalf("inventory = %#v", inventory)
			}
		})
	}
}

func TestEncodeAirLedgerRejectsUnrepresentableTimestamp(t *testing.T) {
	ledger := validPreparedAirLedgerForTest()
	ledger.CreatedAt = time.Date(10_000, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := encodeAirLedger(ledger); err == nil || !strings.Contains(err.Error(), "encode Air recovery ledger") {
		t.Fatalf("encode error = %v", err)
	}
}

func TestIsSHA256HexRejectsWrongLengthAndEncoding(t *testing.T) {
	if isSHA256Hex("abc") {
		t.Fatal("accepted a short digest")
	}
	if isSHA256Hex(strings.Repeat("z", 64)) {
		t.Fatal("accepted a non-hex digest")
	}
}
