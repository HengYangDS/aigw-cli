package recovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

type airRecoveryFixture struct {
	store      Store
	root       string
	air        string
	standalone string
	orphan     []byte
}

func newAirRecoveryFixture(t *testing.T) airRecoveryFixture {
	t.Helper()
	root := t.TempDir()
	standalone := filepath.Join(root, "standalone", "config.toml")
	air := filepath.Join(root, "air", "config.toml")
	for _, path := range []string{standalone, air} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(standalone, []byte("model_provider = \"native\"\nstandalone_only = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := domain.Runtime{ProfileID: "terra", ProfileLabel: "Terra", AccountID: "gateway", Client: domain.ClientCodex, Endpoint: "https://gateway.test/v1", Model: "gpt-5.6-terra"}
	if err := adapters.SyncCodexConfig(standalone, rt); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(standalone)
	if err != nil {
		t.Fatal(err)
	}
	orphan := bytes.Replace(projected, []byte("https://gateway.test/v1"), []byte("https://orphan.test/v1"), 1)
	orphan = append(orphan, []byte("\n[jetbrains]\nhost_only = true\n")...)
	if err := os.WriteFile(air, orphan, 0o640); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(root, "aigw-data", "recovery"))
	store.now = func() time.Time { return time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC) }
	return airRecoveryFixture{store: store, root: root, air: air, standalone: standalone, orphan: orphan}
}

func (f airRecoveryFixture) recoverOptions(caseID string) AirRecoverOptions {
	return AirRecoverOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: caseID}
}

func TestPlanAirOrphanRecoveryIsReadOnlyAndCaseStable(t *testing.T) {
	f := newAirRecoveryFixture(t)
	first, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if first.CaseID != second.CaseID || first.CaseID == "" || first.RecoveryGeneration != 1 {
		t.Fatalf("plans=%#v %#v", first, second)
	}
	if first.State != "orphaned-exact-full-selection" || first.Action != "quarantine-and-clean" {
		t.Fatalf("plan=%#v", first)
	}
	if !strings.HasPrefix(first.CaseID, "air-000001-") {
		t.Fatalf("case=%q", first.CaseID)
	}
	if _, err := os.Stat(filepath.Join(f.root, "aigw-data", "recovery")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote recovery root: %v", err)
	}
	got, _ := os.ReadFile(f.air)
	if !bytes.Equal(got, f.orphan) {
		t.Fatal("dry run changed Air")
	}
}

func TestRecoverAirOrphanWritesQuarantineAndAwaitingLedger(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != "awaiting-host-roundtrip" || receipt.CaseID != plan.CaseID {
		t.Fatalf("receipt=%#v", receipt)
	}
	cleaned, err := os.ReadFile(f.air)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"managed by AIGW", "[model_providers.aigw]", "orphan.test", "gpt-5.6-terra"} {
		if bytes.Contains(cleaned, []byte(forbidden)) {
			t.Fatalf("cleaned retains %q", forbidden)
		}
	}
	for _, want := range []string{"standalone_only = true", "[jetbrains]", "host_only = true"} {
		if !bytes.Contains(cleaned, []byte(want)) {
			t.Fatalf("cleaned lost %q", want)
		}
	}
	assertRecoveryFileModeForTest(t, f.air, 0o640)
	quarantine := f.store.airQuarantinePath(plan.CaseID)
	payload, err := os.ReadFile(quarantine)
	if err != nil || !bytes.Equal(payload, f.orphan) {
		t.Fatalf("quarantine=%q %v", payload, err)
	}
	assertRecoveryFileModeForTest(t, quarantine, 0o600)
	ledger, present, err := f.store.loadAirLedger()
	if err != nil || !present || ledger.State != "awaiting-host-roundtrip" {
		t.Fatalf("ledger=%#v present=%v err=%v", ledger, present, err)
	}
	raw, _ := os.ReadFile(f.store.airLedgerPath())
	for _, forbidden := range []string{f.air, f.standalone, "orphan.test", "gpt-5.6-terra", "model_provider"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ledger leaked %q: %s", forbidden, raw)
		}
	}
	var valid any
	if err := json.Unmarshal(raw, &valid); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverAirOrphanRejectsChangedPreimageAndWrongCase(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions("air-000001-wrong")); err == nil {
		t.Fatal("wrong case accepted")
	}
	if _, err := os.Stat(f.store.airLedgerPath()); !os.IsNotExist(err) {
		t.Fatalf("wrong case wrote ledger: %v", err)
	}
	if err := os.WriteFile(f.air, append(append([]byte(nil), f.orphan...), []byte("newer = true\n")...), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
		t.Fatal("changed preimage accepted")
	}
	if _, err := os.Stat(f.store.airLedgerPath()); !os.IsNotExist(err) {
		t.Fatalf("changed preimage wrote ledger: %v", err)
	}
}

func TestPlanAirOrphanRecoveryRefusesMirrorAndPartialResidue(t *testing.T) {
	t.Run("mirror", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		projected, _ := os.ReadFile(f.standalone)
		if err := os.WriteFile(f.air, projected, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("mirror accepted")
		}
	})
	t.Run("partial", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		if err := os.WriteFile(f.air, []byte("# >>> AIGW managed provider >>>\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
			t.Fatal("partial accepted")
		}
	})
}

func TestSettleAirRequiresHostRoundtripThenSettlesExternalRewrite(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	settle := AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
	preview, err := f.store.PlanAirSettlement(settle)
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "awaiting-host-roundtrip" || preview.Action != "wait" {
		t.Fatalf("preview=%#v", preview)
	}
	if _, err := f.store.SettleAir(settle); err == nil {
		t.Fatal("unchanged cleaned postimage settled")
	}
	if err := os.WriteFile(f.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	preview, err = f.store.PlanAirSettlement(settle)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Action != "settle" {
		t.Fatalf("preview=%#v", preview)
	}
	receipt, err := f.store.SettleAir(settle)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != "settled" {
		t.Fatalf("receipt=%#v", receipt)
	}
	if _, err := os.Stat(f.store.airQuarantinePath(plan.CaseID)); !os.IsNotExist(err) {
		t.Fatalf("quarantine remains: %v", err)
	}
	ledger, present, err := f.store.loadAirLedger()
	if err != nil || !present || ledger.State != "settled" || ledger.ObservedRoundtripSHA256 == "" {
		t.Fatalf("ledger=%#v %v %v", ledger, present, err)
	}
}

func TestSettleAirAcceptsReferenceMirrorAndQuarantinesUnexpectedResidue(t *testing.T) {
	t.Run("mirror", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		_, _ = f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
		projected, _ := os.ReadFile(f.standalone)
		if err := os.WriteFile(f.air, projected, 0o640); err != nil {
			t.Fatal(err)
		}
		receipt, err := f.store.SettleAir(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID})
		if err != nil || receipt.State != "settled" {
			t.Fatalf("receipt=%#v err=%v", receipt, err)
		}
	})
	t.Run("partial", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		_, _ = f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
		if err := os.WriteFile(f.air, []byte("# >>> AIGW managed provider >>>\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		receipt, err := f.store.SettleAir(AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID})
		if err != nil {
			t.Fatal(err)
		}
		if receipt.State != "quarantined" {
			t.Fatalf("receipt=%#v", receipt)
		}
		if _, err := os.Stat(f.store.airQuarantinePath(plan.CaseID)); err != nil {
			t.Fatalf("quarantine missing: %v", err)
		}
	})
}

func TestRecoverAirOrphanRollsBackEveryArtifactWhenFinalLedgerWriteFails(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	originalWrite := f.store.write
	ledgerWrites := 0
	f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if path == f.store.airLedgerPath() {
			ledgerWrites++
			if ledgerWrites == 2 {
				return transaction.FileSnapshot{}, errors.New("injected final-ledger failure")
			}
		}
		return originalWrite(path, expected, data, mode)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
		t.Fatal("injected final ledger failure unexpectedly succeeded")
	}
	got, err := os.ReadFile(f.air)
	if err != nil || !bytes.Equal(got, f.orphan) {
		t.Fatalf("Air after rollback = %q, %v", got, err)
	}
	for _, path := range []string{f.store.airLedgerPath(), f.store.airQuarantinePath(plan.CaseID)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("artifact remains after rollback: %v", err)
		}
	}
}

func TestRecoverAirOrphanResumesPreparedJournalBeforeOrAfterAirWrite(t *testing.T) {
	for _, airAlreadyCleaned := range []bool{false, true} {
		t.Run(map[bool]string{false: "before Air write", true: "after Air write"}[airAlreadyCleaned], func(t *testing.T) {
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
			if airAlreadyCleaned {
				if _, err := f.store.write(f.air, plan.removal.Preimage, plan.removal.Cleaned.Data, plan.removal.Cleaned.Mode); err != nil {
					t.Fatal(err)
				}
			}
			receipt, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
			if err != nil {
				t.Fatal(err)
			}
			if receipt.State != AirRecoveryStateAwaitingHostRoundtrip || receipt.Action != "resumed" {
				t.Fatalf("receipt = %#v", receipt)
			}
			current, err := transaction.CaptureFileSnapshot(f.air)
			if err != nil || current.SHA256 != plan.CleanedPostimageSHA256 {
				t.Fatalf("Air snapshot = %#v, %v", current, err)
			}
			ledger, present, err := f.store.loadAirLedger()
			if err != nil || !present || ledger.State != AirRecoveryStateAwaitingHostRoundtrip {
				t.Fatalf("ledger = %#v, %v, %v", ledger, present, err)
			}
		})
	}
}

func TestSettleAirRestoresLedgerWhenQuarantineRemovalFails(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	originalRemove := f.store.remove
	f.store.remove = func(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if path == f.store.airQuarantinePath(plan.CaseID) {
			return transaction.FileSnapshot{}, errors.New("injected quarantine removal failure")
		}
		return originalRemove(path, expected)
	}
	settle := AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
	if _, err := f.store.SettleAir(settle); err == nil {
		t.Fatal("injected quarantine removal failure unexpectedly succeeded")
	}
	ledger, present, err := f.store.loadAirLedger()
	if err != nil || !present || ledger.State != AirRecoveryStateAwaitingHostRoundtrip || ledger.SettledAt != nil {
		t.Fatalf("ledger after rollback = %#v, %v, %v", ledger, present, err)
	}
	if _, err := os.Stat(f.store.airQuarantinePath(plan.CaseID)); err != nil {
		t.Fatalf("quarantine missing after rollback: %v", err)
	}
}

func TestAirLedgerRejectsUnknownFieldsAndMismatchedCaseBinding(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(plan.preparedLedger, &body); err != nil {
		t.Fatal(err)
	}
	body["configuration_path"] = "/private/air/config.toml"
	data, _ := json.Marshal(body)
	if err := os.MkdirAll(filepath.Dir(f.store.airLedgerPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.store.airLedgerPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.captureAirLedger(); err == nil {
		t.Fatal("unknown ledger field accepted")
	}

	delete(body, "configuration_path")
	body["case_id"] = "air-000001-000000000000"
	data, _ = json.Marshal(body)
	if err := os.WriteFile(f.store.airLedgerPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.captureAirLedger(); err == nil {
		t.Fatal("mismatched case binding accepted")
	}
}

func TestSettleAirReappearedProjectionWaitsForReferenceProof(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.air, f.orphan, 0o640); err != nil {
		t.Fatal(err)
	}
	options := AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
	settlement, err := f.store.PlanAirSettlement(options)
	if err != nil {
		t.Fatal(err)
	}
	if settlement.State != "reappeared-after-recovery" || settlement.Action != "wait-for-reference" {
		t.Fatalf("settlement = %#v", settlement)
	}
	if _, err := f.store.SettleAir(options); err == nil {
		t.Fatal("reappeared projection settled without reference proof")
	}
	ledger, present, err := f.store.loadAirLedger()
	if err != nil || !present || ledger.State != AirRecoveryStateAwaitingHostRoundtrip {
		t.Fatalf("ledger = %#v, %v, %v", ledger, present, err)
	}
	projection, _ := os.ReadFile(f.standalone)
	if err := os.WriteFile(f.air, projection, 0o640); err != nil {
		t.Fatal(err)
	}
	if receipt, err := f.store.SettleAir(options); err != nil || receipt.State != AirRecoveryStateSettled {
		t.Fatalf("receipt = %#v, error = %v", receipt, err)
	}
}

func TestSettleAirQuarantinedCaseCannotTransitionToSettled(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	options := AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
	if err := os.WriteFile(f.air, []byte("# >>> AIGW managed provider >>>\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if receipt, err := f.store.SettleAir(options); err != nil || receipt.State != AirRecoveryStateQuarantined {
		t.Fatalf("receipt = %#v, error = %v", receipt, err)
	}
	if err := os.WriteFile(f.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	settlement, err := f.store.PlanAirSettlement(options)
	if err != nil {
		t.Fatal(err)
	}
	if settlement.State != AirRecoveryStateQuarantined || settlement.Action != "already-quarantined" {
		t.Fatalf("settlement = %#v", settlement)
	}
	if receipt, err := f.store.SettleAir(options); err != nil || receipt.State != AirRecoveryStateQuarantined {
		t.Fatalf("receipt = %#v, error = %v", receipt, err)
	}
	if _, err := os.Stat(f.store.airQuarantinePath(plan.CaseID)); err != nil {
		t.Fatalf("quarantine missing: %v", err)
	}
}

func TestPlanAirOrphanRecoveryRefusesNewGenerationUntilSettledPayloadIsGone(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	ledger, snapshot, err := f.store.captureAirLedger()
	if err != nil {
		t.Fatal(err)
	}
	now := f.store.now().UTC()
	ledger.State = AirRecoveryStateSettled
	ledger.SettledAt = &now
	data, _ := encodeAirLedger(ledger)
	if _, err := f.store.write(f.store.airLedgerPath(), snapshot, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.air, f.orphan, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
		t.Fatal("new generation admitted while settled quarantine payload remains")
	}
}

func TestSettleAirRejectsChangedClassificationInputBeforeOwnedWrites(t *testing.T) {
	for _, changed := range []string{"air", "air-sidecar", "standalone", "standalone-sidecar"} {
		t.Run(changed, func(t *testing.T) {
			f := newAirRecoveryFixture(t)
			plan, _ := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
			if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
				t.Fatal(err)
			}
			projection, _ := os.ReadFile(f.standalone)
			if err := os.WriteFile(f.air, projection, 0o640); err != nil {
				t.Fatal(err)
			}
			originalCapture := f.store.capture
			airCaptures := 0
			airSidecarCaptures := 0
			standaloneCaptures := 0
			standaloneSidecarCaptures := 0
			f.store.capture = func(path string) (transaction.FileSnapshot, error) {
				if path == f.air {
					airCaptures++
					if changed == "air" && airCaptures == 3 {
						if err := os.WriteFile(f.air, []byte("newer-air-state = true\n"), 0o640); err != nil {
							return transaction.FileSnapshot{}, err
						}
					}
				}
				if path == codexSidecarPath(f.air) {
					airSidecarCaptures++
					if changed == "air-sidecar" && airSidecarCaptures == 3 {
						if err := os.WriteFile(path, []byte(`{"writer_id":"foreign"}`), 0o600); err != nil {
							return transaction.FileSnapshot{}, err
						}
					}
				}
				if path == f.standalone {
					standaloneCaptures++
					if changed == "standalone" && standaloneCaptures == 3 {
						if err := os.WriteFile(f.standalone, append(projection, []byte("newer-reference = true\n")...), 0o600); err != nil {
							return transaction.FileSnapshot{}, err
						}
					}
				}
				if path == codexSidecarPath(f.standalone) {
					standaloneSidecarCaptures++
					if changed == "standalone-sidecar" && standaloneSidecarCaptures == 3 {
						body, err := os.ReadFile(path)
						if err != nil {
							return transaction.FileSnapshot{}, err
						}
						if err := os.WriteFile(path, append(body, ' '), 0o600); err != nil {
							return transaction.FileSnapshot{}, err
						}
					}
				}
				return originalCapture(path)
			}
			options := AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
			if _, err := f.store.SettleAir(options); err == nil {
				t.Fatal("changed external preimage was accepted")
			}
			ledger, present, err := f.store.loadAirLedger()
			if err != nil || !present || ledger.State != AirRecoveryStateAwaitingHostRoundtrip {
				t.Fatalf("ledger = %#v, %v, %v", ledger, present, err)
			}
			if _, err := os.Stat(f.store.airQuarantinePath(plan.CaseID)); err != nil {
				t.Fatalf("quarantine missing: %v", err)
			}
		})
	}
}
