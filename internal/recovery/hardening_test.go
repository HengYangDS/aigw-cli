package recovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestAirRecoveryPlanJSONExposesOnlyBoundedPreviewFields(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(body))
	for key := range body {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{"action", "case_id", "recovery_generation", "state", "surface_id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON keys = %v, want %v; body = %s", got, want, data)
	}
}

func TestAirRecoveryPublicJSONSchemasAreExactAndPrivateFieldsStayPrivate(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatal(err)
	}
	settlementPlan, err := f.store.PlanAirSettlement(AirSettleOptions{
		AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	settlementReceipt, err := f.store.SettleAir(AirSettleOptions{
		AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID,
	})
	if err != nil {
		t.Fatal(err)
	}

	private := []string{
		f.air,
		f.standalone,
		f.store.root,
		"https://orphan.test/v1",
		"https://gateway.test/v1",
		plan.ProjectionFingerprintSHA256,
		plan.ConfigPreimageSHA256,
		plan.CleanedPostimageSHA256,
		plan.QuarantineSHA256,
	}
	tests := []struct {
		name      string
		value     any
		wantKeys  []string
		forbidden []string
	}{
		{
			name: "recover dry-run plan", value: plan,
			wantKeys:  []string{"action", "case_id", "recovery_generation", "state", "surface_id"},
			forbidden: private,
		},
		{
			name: "recover apply receipt", value: receipt,
			wantKeys:  []string{"action", "case_id", "recovery_generation", "state", "surface_id"},
			forbidden: private,
		},
		{
			name: "settle dry-run plan", value: settlementPlan,
			wantKeys:  []string{"action", "case_id", "recovery_generation", "state", "surface_id"},
			forbidden: private,
		},
		{
			name: "settle apply receipt", value: settlementReceipt,
			wantKeys:  []string{"action", "case_id", "recovery_generation", "state", "surface_id"},
			forbidden: private,
		},
		{
			name: "lifecycle status", value: lifecycle,
			wantKeys:  []string{"derived_state", "recovery_health", "recovery_reason_code", "recovery_state"},
			forbidden: append(append([]string{}, private...), plan.CaseID, plan.ConfigPreimageSHA256[:12]),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertAirPublicJSONSchema(t, tt.value, tt.wantKeys, tt.forbidden)
		})
	}
}

func TestInspectAirLifecycleClassifiesRecoveryStorageWithoutWrites(t *testing.T) {
	tests := []struct {
		name       string
		prepare    bool
		mutate     func(t *testing.T, f airRecoveryFixture, caseID string)
		wantState  string
		wantHealth string
		wantReason string
	}{
		{name: "ledger missing", wantState: "none", wantHealth: "inactive", wantReason: "ledger-missing"},
		{
			name: "ledger invalid", prepare: true,
			mutate: func(t *testing.T, f airRecoveryFixture, _ string) {
				if err := os.WriteFile(f.store.airLedgerPath(), []byte("{\"private_url\":\"https://doctor-secret.invalid/v1\"}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "unknown", wantHealth: "invalid", wantReason: "ledger-invalid",
		},
		{
			name: "ledger permission invalid", prepare: true,
			mutate: func(t *testing.T, f airRecoveryFixture, _ string) {
				if runtime.GOOS == "windows" {
					t.Skip("POSIX permission contract")
				}
				if err := os.Chmod(f.store.airLedgerPath(), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "unknown", wantHealth: "invalid", wantReason: "ledger-permission-invalid",
		},
		{
			name: "quarantine missing", prepare: true,
			mutate: func(t *testing.T, f airRecoveryFixture, caseID string) {
				if err := os.Remove(f.store.airQuarantinePath(caseID)); err != nil {
					t.Fatal(err)
				}
			},
			wantState: AirRecoveryStateAwaitingHostRoundtrip, wantHealth: "invalid", wantReason: "quarantine-missing",
		},
		{
			name: "quarantine invalid", prepare: true,
			mutate: func(t *testing.T, f airRecoveryFixture, caseID string) {
				if err := os.WriteFile(f.store.airQuarantinePath(caseID), []byte("https://doctor-secret.invalid/v1\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantState: AirRecoveryStateAwaitingHostRoundtrip, wantHealth: "invalid", wantReason: "quarantine-invalid",
		},
		{
			name: "quarantine permission invalid", prepare: true,
			mutate: func(t *testing.T, f airRecoveryFixture, caseID string) {
				if runtime.GOOS == "windows" {
					t.Skip("POSIX permission contract")
				}
				if err := os.Chmod(f.store.airQuarantinePath(caseID), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState: AirRecoveryStateAwaitingHostRoundtrip, wantHealth: "invalid", wantReason: "quarantine-permission-invalid",
		},
		{
			name: "valid active recovery", prepare: true,
			wantState: AirRecoveryStateAwaitingHostRoundtrip, wantHealth: "healthy", wantReason: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAirRecoveryFixture(t)
			caseID := ""
			if tt.prepare {
				plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
				if err != nil {
					t.Fatal(err)
				}
				caseID = plan.CaseID
				if _, err := f.store.RecoverAirOrphan(f.recoverOptions(caseID)); err != nil {
					t.Fatal(err)
				}
			}
			if tt.mutate != nil {
				tt.mutate(t, f, caseID)
			}
			if !tt.prepare {
				if _, err := os.Stat(f.store.root); !os.IsNotExist(err) {
					t.Fatalf("unexpected recovery root before inspection: %v", err)
				}
			}
			f.store.write = func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error) {
				t.Fatal("lifecycle inspection attempted a write")
				return transaction.FileSnapshot{}, nil
			}
			f.store.remove = func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error) {
				t.Fatal("lifecycle inspection attempted a remove")
				return transaction.FileSnapshot{}, nil
			}
			f.store.restore = func(string, transaction.FileSnapshot, transaction.FileSnapshot) error {
				t.Fatal("lifecycle inspection attempted a restore")
				return nil
			}
			status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
			if err != nil {
				t.Fatalf("inspection returned internal storage error: %v", err)
			}
			if status.RecoveryState != tt.wantState || status.RecoveryHealth != tt.wantHealth || status.RecoveryReasonCode != tt.wantReason {
				t.Fatalf("status = %#v, want state=%q health=%q reason=%q", status, tt.wantState, tt.wantHealth, tt.wantReason)
			}
			if !tt.prepare {
				if _, err := os.Stat(f.store.root); !os.IsNotExist(err) {
					t.Fatalf("read-only inspection created recovery root: %v", err)
				}
			}
		})
	}
}

func TestInspectAirLifecycleRejectsSettledLedgerWithUnsafeStoragePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission contract")
	}
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
	if err := os.Chmod(f.store.root, 0o755); err != nil {
		t.Fatal(err)
	}

	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if status.RecoveryState != AirRecoveryStateSettled || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStoragePermission {
		t.Fatalf("status = %#v, want settled invalid storage-permission-invalid", status)
	}
}

func TestInspectAirLifecycleRejectsOrphanQuarantineWithoutLedger(t *testing.T) {
	f := newAirRecoveryFixture(t)
	caseID := validPreparedAirLedgerForTest().CaseID
	quarantinePath := f.store.airQuarantinePath(caseID)
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o700); err != nil {
		t.Fatal(err)
	}
	payload := []byte("private orphan quarantine payload")
	if err := os.WriteFile(quarantinePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if status.RecoveryState != "none" || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonQuarantineUnexpected {
		t.Fatalf("status = %#v, want none invalid quarantine-unexpected", status)
	}
	got, err := os.ReadFile(quarantinePath)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("read-only inspection changed orphan quarantine: %q, %v", got, err)
	}
}

func TestInspectAirLifecycleRejectsUntrackedQuarantineAlongsideActiveCase(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}

	untrackedCaseID := "air-999999-deadbeefcafe"
	untrackedPath := f.store.airQuarantinePath(untrackedCaseID)
	if err := os.MkdirAll(filepath.Dir(untrackedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	privatePayload := []byte("https://untracked-private.invalid/v1\naigw-test-untracked-private-credential\n")
	if err := os.WriteFile(untrackedPath, privatePayload, 0o600); err != nil {
		t.Fatal(err)
	}
	trackedBefore, err := os.ReadFile(f.store.airQuarantinePath(plan.CaseID))
	if err != nil {
		t.Fatal(err)
	}

	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if status.RecoveryState != AirRecoveryStateAwaitingHostRoundtrip || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonQuarantineUnexpected {
		t.Fatalf("status = %#v, want awaiting-host-roundtrip invalid quarantine-unexpected", status)
	}
	assertAirPublicJSONSchema(t, status,
		[]string{"derived_state", "recovery_health", "recovery_reason_code", "recovery_state"},
		[]string{plan.CaseID, untrackedCaseID, filepath.Dir(untrackedPath), string(privatePayload), plan.ConfigPreimageSHA256},
	)
	trackedAfter, err := os.ReadFile(f.store.airQuarantinePath(plan.CaseID))
	if err != nil || !bytes.Equal(trackedAfter, trackedBefore) {
		t.Fatalf("read-only inspection changed tracked quarantine: %q, %v", trackedAfter, err)
	}
	untrackedAfter, err := os.ReadFile(untrackedPath)
	if err != nil || !bytes.Equal(untrackedAfter, privatePayload) {
		t.Fatalf("read-only inspection changed untracked quarantine: %q, %v", untrackedAfter, err)
	}
}

func TestInspectAirLifecycleRejectsUnexpectedRecoveryStorageEntries(t *testing.T) {
	t.Run("active ledger temporary residue", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
			t.Fatal(err)
		}
		residue := filepath.Join(f.store.root, "air", ".aigw-write-review")
		if err := os.WriteFile(residue, []byte("crash residue\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
		if err != nil {
			t.Fatal(err)
		}
		if status.RecoveryState != AirRecoveryStateAwaitingHostRoundtrip || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStorageUnexpected {
			t.Fatalf("status = %#v, want awaiting-host-roundtrip invalid storage-unexpected", status)
		}
		if got, err := os.ReadFile(residue); err != nil || string(got) != "crash residue\n" {
			t.Fatalf("read-only inspection changed residue: %q, %v", got, err)
		}
	})

	t.Run("root residue without ledger", func(t *testing.T) {
		f := newAirRecoveryFixture(t)
		if err := os.MkdirAll(f.store.root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.store.root, "foreign.bin"), []byte("private\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
		if err != nil {
			t.Fatal(err)
		}
		if status.RecoveryState != "none" || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStorageUnexpected {
			t.Fatalf("status = %#v, want none invalid storage-unexpected", status)
		}
	})
}

func TestInspectAirLifecycleRejectsUnsafeExistingRecoveryDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission contract")
	}
	tests := []struct {
		name      string
		prepare   bool
		unsafeDir func(airRecoveryFixture, string) string
		wantState string
	}{
		{
			name: "empty recovery root without ledger",
			unsafeDir: func(f airRecoveryFixture, _ string) string {
				return f.store.root
			},
			wantState: "none",
		},
		{
			name: "empty Air state directory without ledger",
			unsafeDir: func(f airRecoveryFixture, _ string) string {
				return filepath.Join(f.store.root, "air")
			},
			wantState: "none",
		},
		{
			name: "empty quarantine directory without ledger",
			unsafeDir: func(f airRecoveryFixture, _ string) string {
				return filepath.Join(f.store.root, "air", "quarantine")
			},
			wantState: "none",
		},
		{
			name:    "active case directory",
			prepare: true,
			unsafeDir: func(f airRecoveryFixture, caseID string) string {
				return filepath.Dir(f.store.airQuarantinePath(caseID))
			},
			wantState: AirRecoveryStateAwaitingHostRoundtrip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAirRecoveryFixture(t)
			caseID := ""
			if tt.prepare {
				plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
				if err != nil {
					t.Fatal(err)
				}
				caseID = plan.CaseID
				if _, err := f.store.RecoverAirOrphan(f.recoverOptions(caseID)); err != nil {
					t.Fatal(err)
				}
			}
			unsafeDir := tt.unsafeDir(f, caseID)
			if !tt.prepare {
				if err := os.MkdirAll(unsafeDir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(unsafeDir, 0o755); err != nil {
				t.Fatal(err)
			}

			status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
			if err != nil {
				t.Fatalf("inspection returned internal storage error: %v", err)
			}
			if status.RecoveryState != tt.wantState || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStoragePermission {
				t.Fatalf("status = %#v, want state=%q invalid storage-permission-invalid", status, tt.wantState)
			}
			info, err := os.Stat(unsafeDir)
			if err != nil {
				t.Fatalf("read-only inspection removed unsafe directory: %v", err)
			}
			if info.Mode().Perm() != 0o755 {
				t.Fatalf("read-only inspection changed unsafe directory mode: %v", info.Mode().Perm())
			}
		})
	}
}

func TestInspectAirLifecycleRejectsSymlinkedRecoveryRootWithoutFollowingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink contract")
	}
	f := newAirRecoveryFixture(t)
	target := filepath.Join(f.root, "private-target")
	if err := os.MkdirAll(filepath.Join(target, "air", "quarantine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.store.root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, f.store.root); err != nil {
		t.Fatal(err)
	}

	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if status.RecoveryState != "none" || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStoragePermission {
		t.Fatalf("status = %#v, want none invalid storage-permission-invalid", status)
	}
}

func TestInspectAirLifecycleRejectsQuarantineSwapToSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink contract")
	}
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	quarantinePath := f.store.airQuarantinePath(plan.CaseID)
	payload, err := os.ReadFile(quarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(f.root, "same-quarantine-target.toml")
	if err := os.WriteFile(target, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	originalCapture := f.store.captureRecovery
	swapped := false
	f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
		if path == quarantinePath && !swapped {
			swapped = true
			if err := os.Remove(quarantinePath); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, quarantinePath); err != nil {
				t.Fatal(err)
			}
		}
		return originalCapture(path)
	}
	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if !swapped {
		t.Fatal("test seam did not swap the inventoried quarantine")
	}
	if status.RecoveryState != AirRecoveryStateAwaitingHostRoundtrip || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonQuarantineUnreadable {
		t.Fatalf("status = %#v, want awaiting-host-roundtrip invalid quarantine-unreadable", status)
	}
}

func TestInspectAirLifecyclePrioritizesUnsafeRootPermissionsWithoutLedgerTraversal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission contract")
	}
	f := newAirRecoveryFixture(t)
	if err := os.MkdirAll(f.store.root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.store.root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.store.root, 0o700) })

	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if status.RecoveryState != "none" || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStoragePermission {
		t.Fatalf("status = %#v, want none invalid storage-permission-invalid", status)
	}
}

func TestInspectAirLifecycleRejectsSymlinkedQuarantineCaseDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink contract")
	}
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	caseDir := filepath.Dir(f.store.airQuarantinePath(plan.CaseID))
	target := filepath.Join(f.root, "private-case-target")
	if err := os.Rename(caseDir, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, caseDir); err != nil {
		t.Fatal(err)
	}

	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatalf("inspection returned internal storage error: %v", err)
	}
	if status.RecoveryState != AirRecoveryStateAwaitingHostRoundtrip || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStoragePermission {
		t.Fatalf("status = %#v, want awaiting-host-roundtrip invalid storage-permission-invalid", status)
	}
}

func assertAirPublicJSONSchema(t *testing.T, value any, wantKeys, forbidden []string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(body))
	for key := range body {
		got = append(got, key)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("JSON keys = %v, want %v; body = %s", got, wantKeys, data)
	}
	for _, secret := range forbidden {
		if secret != "" && bytes.Contains(data, []byte(secret)) {
			t.Fatalf("public JSON leaked %q: %s", secret, data)
		}
	}
}

func TestAirLedgerRejectsCrossFieldStateTimeDigestAndModeViolations(t *testing.T) {
	base := validPreparedAirLedgerForTest()
	oneMinute := time.Minute
	tests := []struct {
		name   string
		mutate func(*airLedger)
	}{
		{name: "generation above six digit maximum", mutate: func(ledger *airLedger) {
			ledger.RecoveryGeneration = 1_000_000
			ledger.CaseID = "air-1000000-" + ledger.ConfigPreimageSHA256[:12]
		}},
		{name: "quarantine is not exact config preimage", mutate: func(ledger *airLedger) {
			ledger.QuarantineSHA256 = digestForTest("different quarantine")
		}},
		{name: "cleaned postimage equals config preimage", mutate: func(ledger *airLedger) {
			ledger.CleanedPostimageSHA256 = ledger.ConfigPreimageSHA256
		}},
		{name: "mode contains non permission bits", mutate: func(ledger *airLedger) {
			ledger.ConfigPreimageMode = 0o1640
		}},
		{name: "prepared has recovered time", mutate: func(ledger *airLedger) {
			recovered := ledger.CreatedAt
			ledger.RecoveredAt = &recovered
		}},
		{name: "prepared has roundtrip digest", mutate: func(ledger *airLedger) {
			ledger.ObservedRoundtripSHA256 = digestForTest("roundtrip")
		}},
		{name: "awaiting lacks recovered time", mutate: func(ledger *airLedger) {
			ledger.State = AirRecoveryStateAwaitingHostRoundtrip
		}},
		{name: "awaiting has observed roundtrip", mutate: func(ledger *airLedger) {
			ledger.State = AirRecoveryStateAwaitingHostRoundtrip
			recovered := ledger.CreatedAt.Add(oneMinute)
			ledger.RecoveredAt = &recovered
			ledger.ObservedRoundtripSHA256 = digestForTest("roundtrip")
		}},
		{name: "recovered before created", mutate: func(ledger *airLedger) {
			ledger.State = AirRecoveryStateAwaitingHostRoundtrip
			recovered := ledger.CreatedAt.Add(-oneMinute)
			ledger.RecoveredAt = &recovered
		}},
		{name: "quarantined lacks observed roundtrip", mutate: func(ledger *airLedger) {
			ledger.State = AirRecoveryStateQuarantined
			recovered := ledger.CreatedAt.Add(oneMinute)
			ledger.RecoveredAt = &recovered
		}},
		{name: "settled lacks settled time", mutate: func(ledger *airLedger) {
			ledger.State = AirRecoveryStateSettled
			recovered := ledger.CreatedAt.Add(oneMinute)
			ledger.RecoveredAt = &recovered
			ledger.ObservedRoundtripSHA256 = digestForTest("roundtrip")
		}},
		{name: "settled before recovered", mutate: func(ledger *airLedger) {
			ledger.State = AirRecoveryStateSettled
			recovered := ledger.CreatedAt.Add(2 * oneMinute)
			settled := ledger.CreatedAt.Add(oneMinute)
			ledger.RecoveredAt = &recovered
			ledger.SettledAt = &settled
			ledger.ObservedRoundtripSHA256 = digestForTest("roundtrip")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := base
			tt.mutate(&ledger)
			if err := validateAirLedger(ledger); err == nil {
				t.Fatalf("invalid ledger accepted: %#v", ledger)
			}
		})
	}
}

func TestPlanAirOrphanRecoveryRejectsGenerationExhaustion(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	ledger := validPreparedAirLedgerForTest()
	ledger.RecoveryGeneration = 999_999
	ledger.CaseID = airCaseID(ledger.RecoveryGeneration, plan.ConfigPreimageSHA256)
	ledger.ConfigPreimageSHA256 = plan.ConfigPreimageSHA256
	ledger.QuarantineSHA256 = plan.ConfigPreimageSHA256
	ledger.CleanedPostimageSHA256 = plan.CleanedPostimageSHA256
	ledger.ProjectionFingerprintSHA256 = plan.ProjectionFingerprintSHA256
	ledger.State = AirRecoveryStateSettled
	recovered := ledger.CreatedAt.Add(time.Minute)
	settled := recovered.Add(time.Minute)
	ledger.RecoveredAt = &recovered
	ledger.SettledAt = &settled
	ledger.ObservedRoundtripSHA256 = digestForTest("roundtrip")
	data, err := encodeAirLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.store.airLedgerPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.store.airLedgerPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PlanAirOrphanRecovery(f.recoverOptions("")); err == nil {
		t.Fatal("generation 1,000,000 was admitted")
	}
}

func TestRecoverAirOrphanRejectsChangedFourSurfaceAdmissionSnapshots(t *testing.T) {
	for _, changed := range []string{"air", "air-sidecar", "standalone", "standalone-sidecar"} {
		t.Run(changed, func(t *testing.T) {
			f := newAirRecoveryFixture(t)
			preview, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
			if err != nil {
				t.Fatal(err)
			}
			originalCapture := f.store.captureRecovery
			changedOnce := false
			f.store.captureRecovery = func(path string) (transaction.FileSnapshot, error) {
				if !changedOnce && path == f.store.airQuarantinePath(preview.CaseID) {
					changedOnce = true
					switch changed {
					case "air":
						if err := os.WriteFile(f.air, append(append([]byte(nil), f.orphan...), []byte("newer = true\n")...), 0o640); err != nil {
							return transaction.FileSnapshot{}, err
						}
					case "air-sidecar":
						if err := os.WriteFile(codexSidecarPath(f.air), []byte(`{"writer_id":"foreign"}`), 0o600); err != nil {
							return transaction.FileSnapshot{}, err
						}
					case "standalone":
						body, err := os.ReadFile(f.standalone)
						if err != nil {
							return transaction.FileSnapshot{}, err
						}
						if err := os.WriteFile(f.standalone, append(body, []byte("newer_reference = true\n")...), 0o600); err != nil {
							return transaction.FileSnapshot{}, err
						}
					case "standalone-sidecar":
						path := codexSidecarPath(f.standalone)
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
			if _, err := f.store.RecoverAirOrphan(f.recoverOptions(preview.CaseID)); err == nil {
				t.Fatal("recovery admitted changed classification input")
			}
			if !changedOnce {
				t.Fatal("test did not inject the concurrent change")
			}
			for _, path := range []string{f.store.airLedgerPath(), f.store.airQuarantinePath(preview.CaseID)} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("owned artifact was committed: %s: %v", path, err)
				}
			}
		})
	}
}

func TestRecoverAirOrphanResumesMatchingQuarantineFirstCrash(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.store.airQuarantinePath(plan.CaseID)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), f.orphan, 0o600); err != nil {
		t.Fatal(err)
	}
	receipt, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != AirRecoveryStateAwaitingHostRoundtrip {
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
}

func TestPreparedAirRecoveryRejectsUnsafeModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission contract")
	}
	tests := []string{
		"ledger file", "quarantine file", "config file",
		"recovery root", "air state directory", "quarantine directory", "case directory",
	}
	for _, changed := range tests {
		t.Run(changed, func(t *testing.T) {
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
				// Loading may itself reject an unsafe ledger mode after hardening.
				if err == nil {
					t.Fatalf("unsafe ledger mode remained readable: %#v, %v", ledger, present)
				}
				return
			}
			if err != nil || !present || ledger.State != AirRecoveryStatePrepared {
				t.Fatalf("ledger = %#v, %v, %v", ledger, present, err)
			}
		})
	}
}

func TestRecoverAirOrphanRollsBackCommitThenErrorAtEveryWriteBoundary(t *testing.T) {
	for _, boundary := range []string{"quarantine", "prepared-ledger", "air", "final-ledger"} {
		t.Run(boundary, func(t *testing.T) {
			f := newAirRecoveryFixture(t)
			plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
			if err != nil {
				t.Fatal(err)
			}
			originalWrite := f.store.write
			ledgerWrites := 0
			f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
				post, err := originalWrite(path, expected, data, mode)
				if err != nil {
					return post, err
				}
				selected := ""
				switch path {
				case f.store.airQuarantinePath(plan.CaseID):
					selected = "quarantine"
				case f.air:
					selected = "air"
				case f.store.airLedgerPath():
					ledgerWrites++
					if ledgerWrites == 1 {
						selected = "prepared-ledger"
					} else {
						selected = "final-ledger"
					}
				}
				if selected == boundary {
					return post, errors.New("injected error after commit")
				}
				return post, nil
			}
			if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
				t.Fatal("commit-then-error unexpectedly succeeded")
			}
			current, err := os.ReadFile(f.air)
			if err != nil || !bytes.Equal(current, f.orphan) {
				t.Fatalf("Air was not restored: %q, %v", current, err)
			}
			for _, path := range []string{f.store.airLedgerPath(), f.store.airQuarantinePath(plan.CaseID)} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("owned artifact remains after rollback: %s: %v", path, err)
				}
			}
		})
	}
}

func TestRecoverRollbackPreservesConcurrentPostimageAndContinuesAllArtifacts(t *testing.T) {
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	concurrent := []byte("concurrent_host_edit = true\n")
	originalWrite := f.store.write
	ledgerWrites := 0
	f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if path == f.store.airLedgerPath() {
			ledgerWrites++
			if ledgerWrites == 2 {
				if err := os.WriteFile(f.air, concurrent, 0o640); err != nil {
					return transaction.FileSnapshot{}, err
				}
				return transaction.FileSnapshot{}, errors.New("injected final-ledger failure")
			}
		}
		return originalWrite(path, expected, data, mode)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err == nil {
		t.Fatal("injected failure unexpectedly succeeded")
	}
	current, err := os.ReadFile(f.air)
	if err != nil || !bytes.Equal(current, concurrent) {
		t.Fatalf("concurrent Air postimage was overwritten: %q, %v", current, err)
	}
	for _, path := range []string{f.store.airLedgerPath(), f.store.airQuarantinePath(plan.CaseID)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rollback stopped before cleaning %s: %v", path, err)
		}
	}
}

func TestSettleAirRollsBackCommitThenErrorAtOwnedBoundaries(t *testing.T) {
	t.Run("quarantine ledger", func(t *testing.T) {
		f, plan, options := preparedSettlementFixture(t, []byte("# >>> AIGW managed provider >>>\n"))
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			post, err := originalWrite(path, expected, data, mode)
			if err == nil && path == f.store.airLedgerPath() {
				return post, errors.New("injected error after quarantine-ledger commit")
			}
			return post, err
		}
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("commit-then-error unexpectedly succeeded")
		}
		assertAwaitingSettlementArtifacts(t, f, plan.CaseID)
	})

	t.Run("settled ledger", func(t *testing.T) {
		f, plan, options := preparedSettlementFixture(t, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"))
		originalWrite := f.store.write
		f.store.write = func(path string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
			post, err := originalWrite(path, expected, data, mode)
			if err == nil && path == f.store.airLedgerPath() {
				return post, errors.New("injected error after settled-ledger commit")
			}
			return post, err
		}
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("commit-then-error unexpectedly succeeded")
		}
		assertAwaitingSettlementArtifacts(t, f, plan.CaseID)
	})

	t.Run("quarantine removal", func(t *testing.T) {
		f, plan, options := preparedSettlementFixture(t, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"))
		originalRemove := f.store.remove
		f.store.remove = func(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
			post, err := originalRemove(path, expected)
			if err == nil && path == f.store.airQuarantinePath(plan.CaseID) {
				return post, errors.New("injected error after quarantine removal")
			}
			return post, err
		}
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("commit-then-error unexpectedly succeeded")
		}
		assertAwaitingSettlementArtifacts(t, f, plan.CaseID)
	})

	t.Run("settled cleanup removal", func(t *testing.T) {
		f, plan, options := preparedSettlementFixture(t, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"))
		if _, err := f.store.SettleAir(options); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.store.airQuarantinePath(plan.CaseID), f.orphan, 0o600); err != nil {
			t.Fatal(err)
		}
		originalRemove := f.store.remove
		f.store.remove = func(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
			post, err := originalRemove(path, expected)
			if err == nil && path == f.store.airQuarantinePath(plan.CaseID) {
				return post, errors.New("injected error after settled cleanup removal")
			}
			return post, err
		}
		if _, err := f.store.SettleAir(options); err == nil {
			t.Fatal("commit-then-error unexpectedly succeeded")
		}
		payload, err := os.ReadFile(f.store.airQuarantinePath(plan.CaseID))
		if err != nil || !bytes.Equal(payload, f.orphan) {
			t.Fatalf("settled cleanup did not restore quarantine: %q, %v", payload, err)
		}
	})
}

func preparedSettlementFixture(t *testing.T, roundtrip []byte) (airRecoveryFixture, AirRecoveryPlan, AirSettleOptions) {
	t.Helper()
	f := newAirRecoveryFixture(t)
	plan, err := f.store.PlanAirOrphanRecovery(f.recoverOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RecoverAirOrphan(f.recoverOptions(plan.CaseID)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.air, roundtrip, 0o640); err != nil {
		t.Fatal(err)
	}
	return f, plan, AirSettleOptions{AirPath: f.air, StandalonePath: f.standalone, CaseID: plan.CaseID}
}

func assertAwaitingSettlementArtifacts(t *testing.T, f airRecoveryFixture, caseID string) {
	t.Helper()
	ledger, present, err := f.store.loadAirLedger()
	if err != nil || !present || ledger.State != AirRecoveryStateAwaitingHostRoundtrip || ledger.SettledAt != nil || ledger.ObservedRoundtripSHA256 != "" {
		t.Fatalf("ledger after rollback = %#v, %v, %v", ledger, present, err)
	}
	payload, err := os.ReadFile(f.store.airQuarantinePath(caseID))
	if err != nil || !bytes.Equal(payload, f.orphan) {
		t.Fatalf("quarantine after rollback = %q, %v", payload, err)
	}
}

func validPreparedAirLedgerForTest() airLedger {
	preimage := digestForTest("config preimage")
	return airLedger{
		SchemaVersion:               airLedgerSchemaVersion,
		SurfaceID:                   airSurfaceID,
		RecoveryGeneration:          1,
		CaseID:                      airCaseID(1, preimage),
		State:                       AirRecoveryStatePrepared,
		CreatedAt:                   time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC),
		ProjectionFingerprintSHA256: digestForTest("projection"),
		ConfigPreimageSHA256:        preimage,
		ConfigPreimageMode:          0o640,
		CleanedPostimageSHA256:      digestForTest("cleaned postimage"),
		QuarantineSHA256:            preimage,
	}
}

func digestForTest(value string) string {
	return desiredSnapshot([]byte(value), 0o600).SHA256
}
