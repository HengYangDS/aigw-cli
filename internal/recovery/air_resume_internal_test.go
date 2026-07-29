package recovery

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func assertAirFileForTest(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func TestPlanExistingAndRecoverAlreadyActive(t *testing.T) {
	t.Run("wrong active case", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		wrong := "air-999999-deadbeefcafe"
		if wrong == plan.CaseID {
			t.Fatal("test case unexpectedly matched active case")
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(wrong)); err == nil {
			t.Fatal("accepted a different active recovery case")
		}
	})

	t.Run("already active", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		receipt, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Action != "already-active" || receipt.State != AirRecoveryStateAwaitingHostRoundtrip {
			t.Fatalf("receipt = %#v", receipt)
		}
	})
}

func TestRecoverAirOrphanRollsBackWhenInputsChangeAfterPreparation(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	originalWrite := f.store.write
	changed := false
	f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		post, err := originalWrite(path, expected, data, mode)
		if err == nil && path == f.store.airLedgerPath() && bytes.Equal(data, plan.preparedLedger) && !changed {
			changed = true
			if writeErr := os.WriteFile(f.standalone, append(append([]byte(nil), expected.Data...), []byte("changed = true\n")...), 0o600); writeErr != nil {
				return post, writeErr
			}
		}
		return post, err
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
		t.Fatal("recovered across a changed reference input")
	}
	assertAirFileForTest(t, f.air, f.orphan)
	for _, path := range []string{f.store.airLedgerPath(), f.store.airQuarantinePath(plan.CaseID)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rollback left %s: %v", path, err)
		}
	}
}

func TestResumePreparedAirRecoveryRejectsUnavailableOrChangedInputs(t *testing.T) {
	t.Run("missing quarantine", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		if err := os.Remove(f.store.airQuarantinePath(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed without quarantine")
		}
	})

	t.Run("changed quarantine", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed with changed quarantine")
		}
	})

	t.Run("missing Air input", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		if err := os.Remove(f.air); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed without Air input")
		}
	})

	t.Run("Air sidecar appears", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		if err := os.WriteFile(codexSidecarPath(f.air), []byte("foreign sidecar"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed with an Air sidecar")
		}
	})

	t.Run("inputs change during inspection", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
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
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed across changing inputs")
		}
	})

	t.Run("preimage changed", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		if err := os.WriteFile(f.air, []byte("model_provider = \"foreign\"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed with changed preimage")
		}
	})

	t.Run("projection changed", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		ledger, present, err := f.store.loadAirLedger()
		if err != nil || !present {
			t.Fatalf("load prepared ledger: present=%v err=%v", present, err)
		}
		ledger.ProjectionFingerprintSHA256 = strings.Repeat("f", 64)
		data, err := encodeAirLedger(ledger)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.store.airLedgerPath(), data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed against changed projection reference")
		}
	})
}

func TestResumePreparedAirRecoveryVerifiesInputsBeforeCommit(t *testing.T) {
	for _, currentState := range []string{"preimage", "cleaned"} {
		t.Run(currentState, func(t *testing.T) {
			f, plan := prepareAirRecoveryCrashForTest(t)
			if currentState == "cleaned" {
				if err := os.WriteFile(f.air, plan.removal.Cleaned.Data, plan.removal.Cleaned.Mode); err != nil {
					t.Fatal(err)
				}
			}
			originalCapture := f.store.capture
			captures := 0
			f.store.capture = func(path string) (transaction.FileSnapshot, error) {
				if path == f.standalone {
					captures++
					if captures == 3 {
						if err := os.WriteFile(path, []byte("model_provider = \"changed\"\n"), 0o600); err != nil {
							return transaction.FileSnapshot{}, err
						}
					}
				}
				return originalCapture(path)
			}
			if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
				t.Fatal("resumed after verified inputs changed")
			}
			if currentState == "preimage" {
				assertAirFileForTest(t, f.air, f.orphan)
			} else {
				assertAirFileForTest(t, f.air, plan.removal.Cleaned.Data)
			}
		})
	}
}

func TestResumePreparedAirRecoveryHandlesCleanupWriteFailures(t *testing.T) {
	t.Run("before commit", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			if path == f.air {
				return transaction.FileSnapshot{}, errors.New("injected cleanup write failure")
			}
			return originalWrite(path, expected, data, mode)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed after cleanup write failure")
		}
		assertAirFileForTest(t, f.air, f.orphan)
	})

	t.Run("after commit", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			post, err := originalWrite(path, expected, data, mode)
			if err == nil && path == f.air {
				return post, errors.New("injected cleanup post-commit failure")
			}
			return post, err
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed after cleanup post-commit failure")
		}
		assertAirFileForTest(t, f.air, f.orphan)
	})
}

func TestResumePreparedAirRecoveryRollsBackFinalizationFailures(t *testing.T) {
	t.Run("unrepresentable timestamp", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		f.store.now = func() time.Time { return time.Date(10_000, 1, 1, 0, 0, 0, 0, time.UTC) }
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed with unrepresentable final timestamp")
		}
		assertAirFileForTest(t, f.air, f.orphan)
	})

	t.Run("ledger failure before commit", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			if path == f.store.airLedgerPath() {
				return transaction.FileSnapshot{}, errors.New("injected ledger write failure")
			}
			return originalWrite(path, expected, data, mode)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed after ledger write failure")
		}
		assertAirFileForTest(t, f.air, f.orphan)
	})

	t.Run("ledger failure after commit", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			post, err := originalWrite(path, expected, data, mode)
			if err == nil && path == f.store.airLedgerPath() {
				return post, errors.New("injected ledger post-commit failure")
			}
			return post, err
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
			t.Fatal("resumed after ledger post-commit failure")
		}
		assertAirFileForTest(t, f.air, f.orphan)
		ledger, present, err := f.store.loadAirLedger()
		if err != nil || !present || ledger.State != AirRecoveryStatePrepared {
			t.Fatalf("ledger = %#v, present=%v, err=%v", ledger, present, err)
		}
	})

	t.Run("Air rollback failure", func(t *testing.T) {
		f, plan := prepareAirRecoveryCrashForTest(t)
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			if path == f.store.airLedgerPath() {
				return transaction.FileSnapshot{}, errors.New("injected ledger write failure")
			}
			return originalWrite(path, expected, data, mode)
		}
		originalRestore := f.store.restore
		f.store.restore = func(path string, preimage, postimage transaction.FileSnapshot) error {
			if path == f.air {
				return errors.New("injected Air rollback failure")
			}
			return originalRestore(path, preimage, postimage)
		}
		_, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
		if err == nil || !strings.Contains(err.Error(), "rollback also failed") {
			t.Fatalf("resume error = %v", err)
		}
		assertAirFileForTest(t, f.air, plan.removal.Cleaned.Data)
	})
}
