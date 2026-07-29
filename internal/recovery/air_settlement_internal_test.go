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

func activeAirSettlementFixtureForTest(t *testing.T) (airRecoveryFixture, AirRecoveryPlan, AirSettleOptions) {
	t.Helper()
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	return f, plan, AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
}

func writeExternalAirForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestInspectAirLifecycleHandlesUnsafeLedgerCaptureAndVanishedConfig(t *testing.T) {
	t.Run("unsafe ledger symlink", func(t *testing.T) {
		root := t.TempDir()
		store := NewStore(root)
		if err := os.MkdirAll(filepath.Dir(store.airLedgerPath()), 0o700); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte("ledger"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, store.airLedgerPath()); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, store, filepath.Join(root, "config.toml"), filepath.Join(root, "standalone.toml"), "unknown", AirRecoveryHealthInvalid, AirRecoveryReasonStoragePermission)
	})

	t.Run("config vanishes before adapter inspection", func(t *testing.T) {
		f, _, _ := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		originalCapture := f.store.capture
		removed := false
		f.store.capture = func(path string) (transaction.FileSnapshot, error) {
			snapshot, err := originalCapture(path)
			if err == nil && path == f.air && !removed {
				removed = true
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			return snapshot, err
		}
		status := assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
		if status.DerivedState != "" {
			t.Fatalf("derived state = %q", status.DerivedState)
		}
	})
}

func TestPlanAirOrphanRecoveryRejectsSettledQuarantineResidue(t *testing.T) {
	f, plan := settledAirRecoveryForTest(t)
	if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), f.orphan, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
		t.Fatal("planned a new generation while settled quarantine remained")
	}
}

func TestPlanAirSettlementAdditionalFailures(t *testing.T) {
	validCase := "air-000001-deadbeefcafe"

	t.Run("invalid case", func(t *testing.T) {
		if _, err := NewStore(t.TempDir()).PlanAirSettlement(AirSettleOptions{}); err == nil {
			t.Fatal("accepted settlement without a case")
		}
	})

	t.Run("ledger capture failure", func(t *testing.T) {
		root := t.TempDir()
		store := NewStore(root)
		if err := os.MkdirAll(filepath.Dir(store.airLedgerPath()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing"), store.airLedgerPath()); err != nil {
			t.Fatal(err)
		}
		if _, err := store.PlanAirSettlement(AirSettleOptions{CaseID: validCase}); err == nil {
			t.Fatal("settled with unreadable ledger")
		}
	})

	t.Run("missing ledger", func(t *testing.T) {
		if _, err := NewStore(t.TempDir()).PlanAirSettlement(AirSettleOptions{CaseID: validCase}); err == nil {
			t.Fatal("settled without active ledger")
		}
	})

	t.Run("quarantine capture failure", func(t *testing.T) {
		f, plan, options := activeAirSettlementFixtureForTest(t)
		originalCapture := f.store.captureRecovery
		f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
			if path == f.store.airQuarantinePath(plan.CaseID) {
				return transaction.FileSnapshot{}, errors.New("injected quarantine capture failure")
			}
			return originalCapture(path)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled without inspecting quarantine")
		}
	})

	t.Run("changed settled quarantine", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirSettlement(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}); err == nil {
			t.Fatal("accepted changed settled quarantine")
		}
	})

	t.Run("prepared ledger", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		if _, err := f.store.PlanAirSettlement(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}); err == nil {
			t.Fatal("settled a prepared ledger")
		}
	})

	t.Run("missing active quarantine", func(t *testing.T) {
		f, plan, options := activeAirSettlementFixtureForTest(t)
		if err := os.Remove(f.store.airQuarantinePath(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled without active quarantine")
		}
	})

	t.Run("missing current config", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		if err := os.Remove(f.air); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled without current config")
		}
	})

	t.Run("unreadable Air sidecar", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		if err := os.Mkdir(codexSidecarPath(f.air), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled with unreadable Air sidecar")
		}
	})

	t.Run("missing standalone reference", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		if err := os.Remove(f.standalone); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled without standalone reference")
		}
	})

	t.Run("unreadable standalone sidecar", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		path := codexSidecarPath(f.standalone)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled with unreadable standalone sidecar")
		}
	})

	t.Run("config vanishes before inspection", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		originalCapture := f.store.capture
		removed := false
		f.store.capture = func(path string) (transaction.FileSnapshot, error) {
			snapshot, err := originalCapture(path)
			if err == nil && path == codexSidecarPath(f.standalone) && !removed {
				removed = true
				if err := os.Remove(f.air); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(f.air, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			return snapshot, err
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled after config vanished")
		}
	})

	t.Run("inputs change during inspection", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		originalCapture := f.store.capture
		captures := 0
		f.store.capture = func(path string) (transaction.FileSnapshot, error) {
			if path == f.air {
				captures++
				if captures == 2 {
					if err := os.WriteFile(path, []byte("model_provider = \"changed\"\n"), 0o640); err != nil {
						return transaction.FileSnapshot{}, err
					}
				}
			}
			return originalCapture(path)
		}
		if _, err := f.store.PlanAirSettlement(options); err == nil {
			t.Fatal("settled across changing inputs")
		}
	})

	t.Run("reappeared orphan has foreign fingerprint", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		if err := os.WriteFile(f.air, f.orphan, 0o640); err != nil {
			t.Fatal(err)
		}
		ledger, present, err := f.store.loadAirLedger()
		if err != nil || !present {
			t.Fatalf("load ledger: present=%v err=%v", present, err)
		}
		ledger.ProjectionFingerprintSHA256 = strings.Repeat("f", 64)
		data, err := encodeAirLedger(ledger)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.store.airLedgerPath(), data, 0o600); err != nil {
			t.Fatal(err)
		}
		plan, err := f.store.PlanAirSettlement(options)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Action != "quarantine" || plan.State != "partial-or-foreign-residue" {
			t.Fatalf("plan = %#v", plan)
		}
	})
}

func TestSettleAirAdditionalTransitionsAndFailures(t *testing.T) {
	t.Run("planning error", func(t *testing.T) {
		if _, err := NewStore(t.TempDir()).SettleAir(AirSettleOptions{}); err == nil {
			t.Fatal("settled without valid plan")
		}
	})

	t.Run("already settled", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		receipt, err := f.store.SettleAir(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID})
		if err != nil || receipt.Action != "already-settled" {
			t.Fatalf("receipt = %#v, %v", receipt, err)
		}
	})

	t.Run("remove settled quarantine", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		path := f.store.airQuarantinePath(plan.CaseID)
		if err := os.WriteFile(path, f.orphan, 0o600); err != nil {
			t.Fatal(err)
		}
		receipt, err := f.store.SettleAir(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID})
		if err != nil || receipt.Action != "completed-settled-cleanup" {
			t.Fatalf("receipt = %#v, %v", receipt, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("quarantine remains: %v", err)
		}
	})

	t.Run("inputs change before commit", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		originalCapture := f.store.capture
		captures := 0
		f.store.capture = func(path string) (transaction.FileSnapshot, error) {
			if path == f.air {
				captures++
				if captures == 3 {
					if err := os.WriteFile(path, []byte("model_provider = \"changed\"\n"), 0o640); err != nil {
						return transaction.FileSnapshot{}, err
					}
				}
			}
			return originalCapture(path)
		}
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("settled after inputs changed")
		}
	})

	t.Run("quarantine inputs change before commit", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		if err := os.WriteFile(f.air, []byte("# >>> AIGW managed provider >>>\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		originalCapture := f.store.capture
		captures := 0
		f.store.capture = func(path string) (transaction.FileSnapshot, error) {
			if path == f.air {
				captures++
				if captures == 3 {
					if err := os.WriteFile(path, []byte("changed = true\n"), 0o640); err != nil {
						return transaction.FileSnapshot{}, err
					}
				}
			}
			return originalCapture(path)
		}
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("quarantined after inputs changed")
		}
	})

	t.Run("settled timestamp cannot encode", func(t *testing.T) {
		f, _, options := activeAirSettlementFixtureForTest(t)
		writeExternalAirForTest(t, f.air)
		f.store.now = func() time.Time { return time.Date(10_000, 1, 1, 0, 0, 0, 0, time.UTC) }
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("encoded settled ledger with invalid timestamp")
		}
	})
}
