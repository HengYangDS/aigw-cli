//go:build !windows

package recovery

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedAirRecoveryRejectsUnsafeModes(t *testing.T) {
	tests := []string{
		"ledger file", "quarantine file", "config file",
		"recovery root", "air state directory", "quarantine directory", "case directory",
	}
	for _, changed := range tests {
		t.Run(changed, func(t *testing.T) {
			f, plan := prepareAirRecoveryCrashForTest(t)
			paths := map[string]string{
				"ledger file":          f.store.airLedgerPath(),
				"quarantine file":      f.store.airQuarantinePath(plan.CaseID),
				"config file":          f.air,
				"recovery root":        f.store.root,
				"air state directory":  filepath.Join(f.store.root, "air"),
				"quarantine directory": filepath.Join(f.store.root, "air", "quarantine"),
				"case directory":       filepath.Dir(f.store.airQuarantinePath(plan.CaseID)),
			}
			mode := os.FileMode(0o755)
			if changed == "ledger file" || changed == "quarantine file" {
				mode = 0o644
			}
			if changed == "config file" {
				mode = 0o600
			}
			if err := os.Chmod(paths[changed], mode); err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
				t.Fatal("prepared recovery resumed with unsafe or mismatched mode")
			}
			current, err := os.ReadFile(f.air)
			if err != nil || !bytes.Equal(current, f.orphan) {
				t.Fatalf("Air changed after rejection: %q, %v", current, err)
			}
			ledger, present, err := f.store.loadAirLedger()
			if changed == "ledger file" {
				if err == nil {
					t.Fatalf("unsafe ledger mode remained readable: %#v, %v", ledger, present)
				}
			} else if err != nil || !present || ledger.State != AirRecoveryStatePrepared {
				t.Fatalf("ledger = %#v, %v, %v", ledger, present, err)
			}
		})
	}
}
