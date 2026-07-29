package recovery

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func prepareAirRecoveryCrashForTest(t *testing.T) (airRecoveryFixture, AirRecoveryPlan) {
	t.Helper()
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.write(f.store.airQuarantinePath(plan.CaseID), plan.quarantineBefore, plan.removal.Preimage.Data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.write(f.store.airLedgerPath(), plan.ledgerBefore, plan.preparedLedger, 0o600); err != nil {
		t.Fatal(err)
	}
	return f, plan
}

func settledAirRecoveryForTest(t *testing.T) (airRecoveryFixture, AirRecoveryPlan) {
	t.Helper()
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.SettleAir(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}); err != nil {
		t.Fatal(err)
	}
	return f, plan
}

func assertLifecycleForTest(t *testing.T, store Store, airPath, standalonePath, state, health, reason string) AirLifecycleStatus {
	t.Helper()
	status, err := store.InspectAirLifecycle(airPath, standalonePath)
	if err != nil {
		t.Fatal(err)
	}
	if status.RecoveryState != state || status.RecoveryHealth != health || status.RecoveryReasonCode != reason {
		t.Fatalf("status = %#v, want state=%q health=%q reason=%q", status, state, health, reason)
	}
	return status
}

func TestInspectAirLifecycleAdditionalActiveClassifications(t *testing.T) {
	t.Run("ledger capture failure", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		f.store.captureRecovery = func(string) (transaction.FileSnapshot, error) {
			return transaction.FileSnapshot{}, errors.New("injected ledger read failure")
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, "unknown", AirRecoveryHealthInvalid, AirRecoveryReasonLedgerUnreadable)
	})

	t.Run("non-regular quarantine", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		path := f.store.airQuarantinePath(plan.CaseID)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthInvalid, AirRecoveryReasonQuarantineUnreadable)
	})

	t.Run("prepared recovery", func(t *testing.T) {
		f, _ := prepareAirRecoveryCrashForTest(t)
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStatePrepared, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
	})

	t.Run("missing current configuration", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(f.air); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
	})

	t.Run("malformed current configuration", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.air, []byte("["), 0o640); err != nil {
			t.Fatal(err)
		}
		status := assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
		if status.DerivedState != "" {
			t.Fatalf("derived state = %q", status.DerivedState)
		}
	})

	t.Run("external current configuration", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.air, []byte("model_provider = \"jetbrains\"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		status := assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
		if status.DerivedState != "" {
			t.Fatalf("derived state = %q", status.DerivedState)
		}
	})

	t.Run("reappeared exact orphan", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.air, f.orphan, 0o640); err != nil {
			t.Fatal(err)
		}
		status := assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
		if status.DerivedState != "reappeared-after-recovery" {
			t.Fatalf("derived state = %q", status.DerivedState)
		}
	})

	t.Run("storage changes after inventory", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		originalCapture := f.store.captureRecovery
		changed := false
		f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
			snapshot, err := originalCapture(path)
			if err == nil && path == f.store.airQuarantinePath(plan.CaseID) && !changed {
				changed = true
				if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			return snapshot, err
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryHealthInvalid, AirRecoveryReasonStoragePermission)
	})
}

func TestInspectAirLifecycleAdditionalSettledClassifications(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		f, _ := settledAirRecoveryForTest(t)
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateSettled, AirRecoveryHealthHealthy, AirRecoveryReasonOK)
	})

	t.Run("quarantine permission", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), f.orphan, 0o644); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateSettled, AirRecoveryHealthInvalid, AirRecoveryReasonQuarantinePermission)
	})

	t.Run("quarantine digest", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateSettled, AirRecoveryHealthInvalid, AirRecoveryReasonQuarantineInvalid)
	})

	t.Run("tracked quarantine remains", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), f.orphan, 0o600); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateSettled, AirRecoveryHealthInvalid, AirRecoveryReasonQuarantineUnexpected)
	})

	t.Run("untracked quarantine remains", func(t *testing.T) {
		f, _ := settledAirRecoveryForTest(t)
		path := f.store.airQuarantinePath("air-999999-deadbeefcafe")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("untracked"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateSettled, AirRecoveryHealthInvalid, AirRecoveryReasonQuarantineUnexpected)
	})

	t.Run("directory changes after inventory", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		originalCapture := f.store.captureRecovery
		changed := false
		f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
			if path == f.store.airLedgerPath() && !changed {
				changed = true
				if err := os.Chmod(filepath.Dir(f.store.airQuarantinePath(plan.CaseID)), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			return originalCapture(path)
		}
		assertLifecycleForTest(t, f.store, f.air, f.standalone, AirRecoveryStateSettled, AirRecoveryHealthInvalid, AirRecoveryReasonStoragePermission)
	})
}

func TestPlanAirOrphanRecoveryAdditionalFailuresAndGeneration(t *testing.T) {
	t.Run("missing option", func(t *testing.T) {
		if _, err := NewStore(t.TempDir()).PlanAirOrphanRecovery(AirRecoverOptions{}); err == nil {
			t.Fatal("accepted unavailable recovery surfaces")
		}
	})

	t.Run("missing Air configuration", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		if err := os.Remove(f.air); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("planned recovery without Air configuration")
		}
	})

	t.Run("inputs change during classification", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		originalCapture := f.store.capture
		captures := 0
		f.store.capture = func(path string) (transaction.FileSnapshot, error) {
			if path == f.air {
				captures++
				if captures == 2 {
					if err := os.WriteFile(path, append(append([]byte(nil), f.orphan...), []byte("changed = true\n")...), 0o640); err != nil {
						return transaction.FileSnapshot{}, err
					}
				}
			}
			return originalCapture(path)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("planned recovery across changing inputs")
		}
	})

	t.Run("quarantine inspection failure", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		originalCapture := f.store.captureRecovery
		f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
			if path != f.store.airLedgerPath() {
				return transaction.FileSnapshot{}, errors.New("injected quarantine inspection failure")
			}
			return originalCapture(path)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("planned recovery without inspecting quarantine")
		}
	})

	t.Run("conflicting quarantine", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		path := f.store.airQuarantinePath(plan.CaseID)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("accepted conflicting quarantine")
		}
	})

	t.Run("unsafe matching quarantine", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		path := f.store.airQuarantinePath(plan.CaseID)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, f.orphan, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("accepted matching quarantine in unsafe storage")
		}
	})

	t.Run("unrepresentable ledger timestamp", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		f.store.now = func() time.Time { return time.Date(10_000, 1, 1, 0, 0, 0, 0, time.UTC) }
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("encoded an unrepresentable ledger timestamp")
		}
	})

	t.Run("second generation", func(t *testing.T) {
		f, first := settledAirRecoveryForTest(t)
		if err := os.WriteFile(f.air, f.orphan, 0o640); err != nil {
			t.Fatal(err)
		}
		second, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if err != nil {
			t.Fatal(err)
		}
		if second.RecoveryGeneration != 2 || second.CaseID == first.CaseID {
			t.Fatalf("second plan = %#v", second)
		}
	})

	t.Run("settled quarantine inspection failure", func(t *testing.T) {
		f, plan := settledAirRecoveryForTest(t)
		originalCapture := f.store.captureRecovery
		f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
			if path == f.store.airQuarantinePath(plan.CaseID) {
				return transaction.FileSnapshot{}, errors.New("injected settled quarantine inspection failure")
			}
			return originalCapture(path)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("planned a new generation without inspecting settled quarantine")
		}
	})
}

func TestCaptureAirRecoveryInputsRejectsUnavailableSurfaces(t *testing.T) {
	for _, surface := range []string{"air", "air-sidecar", "standalone", "standalone-sidecar"} {
		t.Run(surface, func(t *testing.T) {
			f := newAirRecoveryFixture(t)
			var path string
			switch surface {
			case "air":
				path = f.air
			case "air-sidecar":
				path = codexSidecarPath(f.air)
			case "standalone":
				path = f.standalone
			case "standalone-sidecar":
				path = codexSidecarPath(f.standalone)
			}
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if surface == "air-sidecar" || surface == "standalone-sidecar" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.store.captureAirRecoveryInputs(f.recoverOptions("")); err == nil {
				t.Fatalf("captured unavailable %s", surface)
			}
		})
	}
}

func TestGuardedMutationHandlesInaccurateOperationSnapshots(t *testing.T) {
	t.Run("write committed with stale observation", func(t *testing.T) {
		base := t.TempDir()
		store := NewStore(filepath.Join(base, "recovery"))
		path := filepath.Join(base, "config.toml")
		if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}
		expected, _ := transaction.CaptureFileSnapshot(path)
		originalWrite := store.write
		store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			if _, err := originalWrite(path, expected, data, mode); err != nil {
				return transaction.FileSnapshot{}, err
			}
			return expected, nil
		}
		post, written, err := store.guardedWrite(path, expected, []byte("after"), 0o600)
		if err != nil || !written || !bytes.Equal(post.Data, []byte("after")) {
			t.Fatalf("guarded write = %#v, %v, %v", post, written, err)
		}
	})

	t.Run("write reports success without commit", func(t *testing.T) {
		base := t.TempDir()
		store := NewStore(filepath.Join(base, "recovery"))
		path := filepath.Join(base, "config.toml")
		if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}
		expected, _ := transaction.CaptureFileSnapshot(path)
		store.write = func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
			return expected, nil
		}
		if _, written, err := store.guardedWrite(path, expected, []byte("after"), 0o600); err == nil || written {
			t.Fatalf("guarded write accepted missing commit: written=%v err=%v", written, err)
		}
	})

	t.Run("remove missing expected", func(t *testing.T) {
		store := NewStore(t.TempDir())
		if post, removed, err := store.guardedRemove(filepath.Join(t.TempDir(), "missing"), transaction.FileSnapshot{}); err != nil || removed || post.Exists {
			t.Fatalf("guarded remove = %#v, %v, %v", post, removed, err)
		}
	})

	t.Run("remove committed with stale observation", func(t *testing.T) {
		base := t.TempDir()
		store := NewStore(filepath.Join(base, "recovery"))
		path := filepath.Join(base, "config.toml")
		if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}
		expected, _ := transaction.CaptureFileSnapshot(path)
		originalRemove := store.remove
		store.remove = func(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
			if _, err := originalRemove(path, expected); err != nil {
				return transaction.FileSnapshot{}, err
			}
			return expected, nil
		}
		if post, removed, err := store.guardedRemove(path, expected); err != nil || !removed || post.Exists {
			t.Fatalf("guarded remove = %#v, %v, %v", post, removed, err)
		}
	})

	t.Run("remove reports success without commit", func(t *testing.T) {
		base := t.TempDir()
		store := NewStore(filepath.Join(base, "recovery"))
		path := filepath.Join(base, "config.toml")
		if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}
		expected, _ := transaction.CaptureFileSnapshot(path)
		store.remove = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
			return expected, nil
		}
		if _, removed, err := store.guardedRemove(path, expected); err == nil || removed {
			t.Fatalf("guarded remove accepted missing commit: removed=%v err=%v", removed, err)
		}
	})
}
