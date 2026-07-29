//go:build !windows

package recovery

import "testing"

func TestValidateAirRecoveryStorageRejectsUnsafeFileModes(t *testing.T) {
	store := NewStore(t.TempDir())
	ledger := desiredSnapshot([]byte("ledger"), 0o644)
	quarantine := desiredSnapshot([]byte("quarantine"), 0o600)
	if err := store.validateAirRecoveryStorage("air-000001-deadbeefcafe", ledger, quarantine); err == nil {
		t.Fatal("accepted an unsafe ledger mode")
	}
}
