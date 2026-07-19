package recovery

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

// AirRecoverOptions identifies the two configuration surfaces and optionally
// binds an apply to the exact case returned by a previous preview.
type AirRecoverOptions struct {
	AirPath        string
	StandalonePath string
	CaseID         string
}

// AirSettleOptions binds settlement to one active Air recovery case.
type AirSettleOptions struct {
	AirPath        string
	StandalonePath string
	CaseID         string
}

// AirRecoveryPlan is the secret-free public preview for recover-orphan.
type AirRecoveryPlan struct {
	SurfaceID                   string `json:"surface_id"`
	State                       string `json:"state"`
	Action                      string `json:"action"`
	CaseID                      string `json:"case_id"`
	RecoveryGeneration          int    `json:"recovery_generation"`
	ProjectionFingerprintSHA256 string `json:"-"`
	ConfigPreimageSHA256        string `json:"-"`
	CleanedPostimageSHA256      string `json:"-"`
	QuarantineSHA256            string `json:"-"`

	removal           adapters.AirOrphanRemovalPlan
	air               transaction.FileSnapshot
	airSidecar        transaction.FileSnapshot
	standalone        transaction.FileSnapshot
	standaloneSidecar transaction.FileSnapshot
	ledgerBefore      transaction.FileSnapshot
	quarantineBefore  transaction.FileSnapshot
	preparedLedger    []byte
	finalLedger       []byte
	activeLedger      airLedger
}

// AirRecoveryReceipt is the bounded result of an applied recovery.
type AirRecoveryReceipt struct {
	SurfaceID          string `json:"surface_id"`
	State              string `json:"state"`
	Action             string `json:"action"`
	CaseID             string `json:"case_id"`
	RecoveryGeneration int    `json:"recovery_generation"`
}

// AirSettlementPlan is a read-only lifecycle classification. It contains no
// configuration body, path, route, or quarantine content.
type AirSettlementPlan struct {
	SurfaceID          string `json:"surface_id"`
	State              string `json:"state"`
	Action             string `json:"action"`
	CaseID             string `json:"case_id"`
	RecoveryGeneration int    `json:"recovery_generation"`

	ledger            airLedger
	ledgerBefore      transaction.FileSnapshot
	air               transaction.FileSnapshot
	airSidecar        transaction.FileSnapshot
	quarantine        transaction.FileSnapshot
	standalone        transaction.FileSnapshot
	standaloneSidecar transaction.FileSnapshot
}

// AirSettlementReceipt is the bounded result of an applied settlement.
type AirSettlementReceipt struct {
	SurfaceID          string `json:"surface_id"`
	State              string `json:"state"`
	Action             string `json:"action"`
	CaseID             string `json:"case_id"`
	RecoveryGeneration int    `json:"recovery_generation"`
}

// AirLifecycleStatus is the bounded read model used by route doctor. It
// separates persistent journal state from a currently derived configuration
// state and contains no case identifier, path, or digest.
type AirLifecycleStatus struct {
	RecoveryState      string `json:"recovery_state"`
	DerivedState       string `json:"derived_state"`
	RecoveryHealth     string `json:"recovery_health"`
	RecoveryReasonCode string `json:"recovery_reason_code"`
}

const (
	AirRecoveryHealthInactive = "inactive"
	AirRecoveryHealthHealthy  = "healthy"
	AirRecoveryHealthInvalid  = "invalid"

	AirRecoveryReasonOK                   = "ok"
	AirRecoveryReasonLedgerMissing        = "ledger-missing"
	AirRecoveryReasonLedgerUnreadable     = "ledger-unreadable"
	AirRecoveryReasonLedgerInvalid        = "ledger-invalid"
	AirRecoveryReasonLedgerPermission     = "ledger-permission-invalid"
	AirRecoveryReasonQuarantineMissing    = "quarantine-missing"
	AirRecoveryReasonQuarantineUnreadable = "quarantine-unreadable"
	AirRecoveryReasonQuarantineInvalid    = "quarantine-invalid"
	AirRecoveryReasonQuarantinePermission = "quarantine-permission-invalid"
	AirRecoveryReasonQuarantineUnexpected = "quarantine-unexpected"
	AirRecoveryReasonStoragePermission    = "storage-permission-invalid"
	AirRecoveryReasonStorageUnexpected    = "storage-unexpected"
)

// InspectAirLifecycle reads the private journal, quarantine metadata, and
// current Air snapshot without changing any of them. Storage defects are
// returned as bounded reason codes rather than path-bearing internal errors.
func (s Store) InspectAirLifecycle(airPath, standalonePath string) (AirLifecycleStatus, error) {
	status := AirLifecycleStatus{
		RecoveryState:      "unknown",
		RecoveryHealth:     AirRecoveryHealthInvalid,
		RecoveryReasonCode: AirRecoveryReasonLedgerUnreadable,
	}
	storage, storageErr := s.inspectAirRecoveryStorage()
	if storage.unsafeTraversal && !storage.ledgerExists {
		status.RecoveryState = "none"
		status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
		return status, nil
	}
	ledgerSnapshot, err := s.captureRecovery(s.airLedgerPath())
	if err != nil {
		if storage.unsafeTraversal {
			status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
		}
		return status, nil
	}
	if !ledgerSnapshot.Exists {
		status.RecoveryState = "none"
		if storageErr != nil {
			status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnreadable
			return status, nil
		}
		if storage.permissionInvalid {
			status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
			return status, nil
		}
		if storage.unexpectedStorage {
			status.RecoveryReasonCode = AirRecoveryReasonStorageUnexpected
			return status, nil
		}
		if storage.hasQuarantineArtifacts() {
			status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnexpected
			return status, nil
		}
		status.RecoveryHealth = AirRecoveryHealthInactive
		status.RecoveryReasonCode = AirRecoveryReasonLedgerMissing
		return status, nil
	}
	if runtime.GOOS != "windows" && ledgerSnapshot.Mode.Perm() != 0o600 {
		status.RecoveryReasonCode = AirRecoveryReasonLedgerPermission
		return status, nil
	}
	ledger, err := decodeAirLedger(ledgerSnapshot.Data)
	if err != nil {
		status.RecoveryReasonCode = AirRecoveryReasonLedgerInvalid
		return status, nil
	}
	status.RecoveryState = ledger.State
	if storage.unsafeTraversal {
		status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
		return status, nil
	}
	if storageErr != nil {
		status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnreadable
		return status, nil
	}
	if storage.permissionInvalid {
		status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
		return status, nil
	}
	if storage.unexpectedStorage {
		status.RecoveryReasonCode = AirRecoveryReasonStorageUnexpected
		return status, nil
	}
	var quarantine transaction.FileSnapshot
	quarantineExists, quarantineRegular := storage.quarantineFile(ledger.CaseID, "config.toml")
	if quarantineExists && !quarantineRegular {
		status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnreadable
		return status, nil
	}
	if quarantineExists {
		quarantine, err = s.captureRecovery(s.airQuarantinePath(ledger.CaseID))
		if err != nil {
			status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnreadable
			return status, nil
		}
	}
	if ledger.State == AirRecoveryStateSettled {
		if quarantine.Exists {
			if runtime.GOOS != "windows" && quarantine.Mode.Perm() != 0o600 {
				status.RecoveryReasonCode = AirRecoveryReasonQuarantinePermission
				return status, nil
			}
			if quarantine.SHA256 != ledger.QuarantineSHA256 {
				status.RecoveryReasonCode = AirRecoveryReasonQuarantineInvalid
				return status, nil
			}
			status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnexpected
			return status, nil
		}
		if err := s.validateAirRecoveryDirectories(ledger.CaseID); err != nil {
			status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
			return status, nil
		}
		if storage.hasUnexpectedQuarantine(ledger.CaseID, false) {
			status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnexpected
			return status, nil
		}
		status.RecoveryHealth = AirRecoveryHealthHealthy
		status.RecoveryReasonCode = AirRecoveryReasonOK
		return status, nil
	}
	if !quarantine.Exists {
		status.RecoveryReasonCode = AirRecoveryReasonQuarantineMissing
		return status, nil
	}
	if runtime.GOOS != "windows" && quarantine.Mode.Perm() != 0o600 {
		status.RecoveryReasonCode = AirRecoveryReasonQuarantinePermission
		return status, nil
	}
	if quarantine.SHA256 != ledger.QuarantineSHA256 {
		status.RecoveryReasonCode = AirRecoveryReasonQuarantineInvalid
		return status, nil
	}
	if err := s.validateAirRecoveryStorage(ledger.CaseID, ledgerSnapshot, quarantine); err != nil {
		status.RecoveryReasonCode = AirRecoveryReasonStoragePermission
		return status, nil
	}
	if storage.hasUnexpectedQuarantine(ledger.CaseID, true) {
		status.RecoveryReasonCode = AirRecoveryReasonQuarantineUnexpected
		return status, nil
	}
	status.RecoveryHealth = AirRecoveryHealthHealthy
	status.RecoveryReasonCode = AirRecoveryReasonOK
	if ledger.State == AirRecoveryStatePrepared {
		return status, nil
	}
	current, err := s.capture(airPath)
	if err != nil || !current.Exists {
		return status, nil
	}
	if current.SHA256 == ledger.CleanedPostimageSHA256 {
		return status, nil
	}
	inspection, err := adapters.InspectAirCodexConfig(airPath, standalonePath)
	if err != nil {
		return status, nil
	}
	if inspection.State != adapters.AirStateOrphanedExactFullSelection {
		return status, nil
	}
	removal, err := adapters.PlanAirOrphanRemoval(airPath, standalonePath)
	if err == nil && removal.ProjectionFingerprintSHA256 == ledger.ProjectionFingerprintSHA256 {
		status.DerivedState = "reappeared-after-recovery"
	}
	return status, nil
}

// PlanAirOrphanRecovery captures every preimage and computes a deterministic
// case without creating the recovery directory or changing Air.
func (s Store) PlanAirOrphanRecovery(options AirRecoverOptions) (AirRecoveryPlan, error) {
	if options.AirPath == "" || options.StandalonePath == "" {
		return AirRecoveryPlan{}, errors.New("Air orphan recovery surfaces are unavailable")
	}
	ledger, ledgerBefore, err := s.captureAirLedger()
	if err != nil {
		return AirRecoveryPlan{}, err
	}
	if ledgerBefore.Exists && ledger.State != AirRecoveryStateSettled {
		return s.planExistingAirRecovery(options, ledger, ledgerBefore)
	}
	if ledgerBefore.Exists && ledger.State == AirRecoveryStateSettled {
		previousQuarantine, captureErr := s.captureRecovery(s.airQuarantinePath(ledger.CaseID))
		if captureErr != nil {
			return AirRecoveryPlan{}, errors.New("inspect settled Air recovery quarantine")
		}
		if previousQuarantine.Exists {
			return AirRecoveryPlan{}, errors.New("settled Air recovery cleanup is incomplete")
		}
	}

	inputs, err := s.captureAirRecoveryInputs(options)
	if err != nil {
		return AirRecoveryPlan{}, err
	}
	removal, err := adapters.PlanAirOrphanRemoval(options.AirPath, options.StandalonePath)
	if err != nil {
		return AirRecoveryPlan{}, errors.New("Air is not an exact removable orphan")
	}
	inputsAfter, err := s.captureAirRecoveryInputs(options)
	if err != nil || !sameAirRecoveryInputs(inputs, inputsAfter) || !sameRecoverySnapshot(inputs.air, removal.Preimage) {
		return AirRecoveryPlan{}, errors.New("Air recovery inputs changed during classification")
	}
	generation := 1
	if ledgerBefore.Exists {
		if ledger.RecoveryGeneration >= maxAirRecoveryGeneration {
			return AirRecoveryPlan{}, errors.New("Air recovery generation is exhausted")
		}
		generation = ledger.RecoveryGeneration + 1
	}
	caseID := airCaseID(generation, removal.Preimage.SHA256)
	quarantineBefore, err := s.captureRecovery(s.airQuarantinePath(caseID))
	if err != nil {
		return AirRecoveryPlan{}, errors.New("inspect Air recovery quarantine")
	}
	action := "quarantine-and-clean"
	if quarantineBefore.Exists {
		if !sameRecoverySnapshot(quarantineBefore, desiredSnapshot(removal.Preimage.Data, 0o600)) {
			return AirRecoveryPlan{}, errors.New("Air recovery quarantine already exists for the preview case")
		}
		if err := s.validateAirRecoveryStorage(caseID, ledgerBefore, quarantineBefore); err != nil {
			return AirRecoveryPlan{}, err
		}
		action = "resume-quarantine-first"
	}

	now := s.now().UTC()
	prepared := airLedger{
		SchemaVersion:               airLedgerSchemaVersion,
		SurfaceID:                   airSurfaceID,
		RecoveryGeneration:          generation,
		CaseID:                      caseID,
		State:                       AirRecoveryStatePrepared,
		CreatedAt:                   now,
		ProjectionFingerprintSHA256: removal.ProjectionFingerprintSHA256,
		ConfigPreimageSHA256:        removal.Preimage.SHA256,
		ConfigPreimageMode:          uint32(removal.Preimage.Mode.Perm()),
		CleanedPostimageSHA256:      removal.Cleaned.SHA256,
		QuarantineSHA256:            removal.Preimage.SHA256,
	}
	preparedData, err := encodeAirLedger(prepared)
	if err != nil {
		return AirRecoveryPlan{}, err
	}
	recoveredAt := now
	final := prepared
	final.State = AirRecoveryStateAwaitingHostRoundtrip
	final.RecoveredAt = &recoveredAt
	finalData, err := encodeAirLedger(final)
	if err != nil {
		return AirRecoveryPlan{}, err
	}
	return AirRecoveryPlan{
		SurfaceID:                   airSurfaceID,
		State:                       "orphaned-exact-full-selection",
		Action:                      action,
		CaseID:                      caseID,
		RecoveryGeneration:          generation,
		ProjectionFingerprintSHA256: removal.ProjectionFingerprintSHA256,
		ConfigPreimageSHA256:        removal.Preimage.SHA256,
		CleanedPostimageSHA256:      removal.Cleaned.SHA256,
		QuarantineSHA256:            removal.Preimage.SHA256,
		removal:                     removal,
		air:                         inputs.air,
		airSidecar:                  inputs.airSidecar,
		standalone:                  inputs.standalone,
		standaloneSidecar:           inputs.standaloneSidecar,
		ledgerBefore:                ledgerBefore,
		quarantineBefore:            quarantineBefore,
		preparedLedger:              preparedData,
		finalLedger:                 finalData,
	}, nil
}

func (s Store) planExistingAirRecovery(options AirRecoverOptions, ledger airLedger, ledgerBefore transaction.FileSnapshot) (AirRecoveryPlan, error) {
	if options.CaseID != "" && options.CaseID != ledger.CaseID {
		return AirRecoveryPlan{}, errors.New("another Air recovery case is active")
	}
	action := "already-active"
	if ledger.State == AirRecoveryStatePrepared {
		action = "resume-prepared"
	}
	return AirRecoveryPlan{
		SurfaceID:                   airSurfaceID,
		State:                       ledger.State,
		Action:                      action,
		CaseID:                      ledger.CaseID,
		RecoveryGeneration:          ledger.RecoveryGeneration,
		ProjectionFingerprintSHA256: ledger.ProjectionFingerprintSHA256,
		ConfigPreimageSHA256:        ledger.ConfigPreimageSHA256,
		CleanedPostimageSHA256:      ledger.CleanedPostimageSHA256,
		QuarantineSHA256:            ledger.QuarantineSHA256,
		ledgerBefore:                ledgerBefore,
		activeLedger:                ledger,
	}, nil
}

// RecoverAirOrphan applies only the exact case bound by options.CaseID.
func (s Store) RecoverAirOrphan(options AirRecoverOptions) (AirRecoveryReceipt, error) {
	plan, err := s.PlanAirOrphanRecovery(options)
	if err != nil {
		return AirRecoveryReceipt{}, err
	}
	if options.CaseID == "" || options.CaseID != plan.CaseID {
		return AirRecoveryReceipt{}, errors.New("Air recovery requires the exact preview case ID")
	}
	if plan.Action == "already-active" {
		return recoveryReceipt(plan, plan.State, "already-active"), nil
	}
	if plan.Action == "resume-prepared" {
		return s.resumePreparedAirRecovery(options, plan)
	}
	if err := s.verifyAirRecoveryInputs(options, plan); err != nil {
		return AirRecoveryReceipt{}, err
	}

	committed := make([]committedRecoveryArtifact, 0, 4)
	quarantinePost, quarantineWritten, err := s.guardedWrite(
		s.airQuarantinePath(plan.CaseID),
		plan.quarantineBefore,
		plan.removal.Preimage.Data,
		0o600,
	)
	if quarantineWritten {
		committed = append(committed, committedRecoveryArtifact{
			path: s.airQuarantinePath(plan.CaseID), before: plan.quarantineBefore, post: quarantinePost,
		})
	}
	if err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("write Air recovery quarantine", committed)
	}

	preparedPost, preparedWritten, err := s.guardedWrite(s.airLedgerPath(), plan.ledgerBefore, plan.preparedLedger, 0o600)
	if preparedWritten {
		committed = append(committed, committedRecoveryArtifact{path: s.airLedgerPath(), before: plan.ledgerBefore, post: preparedPost})
	}
	if err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("prepare Air recovery ledger", committed)
	}
	if err := s.verifyAirRecoveryInputs(options, plan); err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("Air recovery inputs changed before config commit", committed)
	}

	airPost, airWritten, err := s.guardedWrite(options.AirPath, plan.removal.Preimage, plan.removal.Cleaned.Data, plan.removal.Cleaned.Mode)
	if airWritten {
		committed = append(committed, committedRecoveryArtifact{path: options.AirPath, before: plan.removal.Preimage, post: airPost})
	}
	if err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("clean exact Air orphan", committed)
	}

	finalPost, finalWritten, err := s.guardedWrite(s.airLedgerPath(), preparedPost, plan.finalLedger, 0o600)
	if finalWritten {
		committed = append(committed, committedRecoveryArtifact{path: s.airLedgerPath(), before: preparedPost, post: finalPost})
	}
	if err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("finalize Air recovery ledger", committed)
	}
	return recoveryReceipt(plan, AirRecoveryStateAwaitingHostRoundtrip, "quarantined-and-cleaned"), nil
}

func (s Store) resumePreparedAirRecovery(options AirRecoverOptions, plan AirRecoveryPlan) (AirRecoveryReceipt, error) {
	ledger := plan.activeLedger
	quarantine, err := s.captureRecovery(s.airQuarantinePath(ledger.CaseID))
	if err != nil || !quarantine.Exists || quarantine.SHA256 != ledger.QuarantineSHA256 {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery quarantine is unavailable")
	}
	if err := s.validateAirRecoveryStorage(ledger.CaseID, plan.ledgerBefore, quarantine); err != nil {
		return AirRecoveryReceipt{}, err
	}
	inputs, err := s.captureAirRecoveryInputs(options)
	if err != nil || inputs.airSidecar.Exists {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery inputs are unavailable")
	}
	inputsAfter, err := s.captureAirRecoveryInputs(options)
	if err != nil || !sameAirRecoveryInputs(inputs, inputsAfter) {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery inputs changed during inspection")
	}
	current := inputs.air
	if current.Mode.Perm() != os.FileMode(ledger.ConfigPreimageMode).Perm() {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery config mode changed")
	}
	plan.air = inputs.air
	plan.airSidecar = inputs.airSidecar
	plan.standalone = inputs.standalone
	plan.standaloneSidecar = inputs.standaloneSidecar
	if current.SHA256 == ledger.CleanedPostimageSHA256 {
		if err := s.verifyAirRecoveryInputs(options, plan); err != nil {
			return AirRecoveryReceipt{}, err
		}
		return s.finalizePreparedAirLedger(plan, plan.ledgerBefore)
	}
	if current.SHA256 != ledger.ConfigPreimageSHA256 {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery preimage changed")
	}
	removal, err := adapters.PlanAirOrphanRemoval(options.AirPath, options.StandalonePath)
	if err != nil || removal.ProjectionFingerprintSHA256 != ledger.ProjectionFingerprintSHA256 || removal.Cleaned.SHA256 != ledger.CleanedPostimageSHA256 {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery no longer matches the exact orphan")
	}
	if !sameRecoverySnapshot(current, removal.Preimage) {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery preimage snapshot changed")
	}
	if err := s.verifyAirRecoveryInputs(options, plan); err != nil {
		return AirRecoveryReceipt{}, err
	}
	airPost, airWritten, err := s.guardedWrite(options.AirPath, current, removal.Cleaned.Data, removal.Cleaned.Mode)
	if err != nil {
		if airWritten {
			_ = s.restore(options.AirPath, current, airPost)
		}
		return AirRecoveryReceipt{}, errors.New("resume exact Air orphan cleanup")
	}
	receipt, finalizeErr := s.finalizePreparedAirLedger(plan, plan.ledgerBefore)
	if finalizeErr != nil {
		if airWritten {
			if rollbackErr := s.restore(options.AirPath, current, airPost); rollbackErr != nil {
				return AirRecoveryReceipt{}, fmt.Errorf("%w; Air rollback also failed", finalizeErr)
			}
		}
		return AirRecoveryReceipt{}, finalizeErr
	}
	return receipt, nil
}

func (s Store) finalizePreparedAirLedger(plan AirRecoveryPlan, ledgerBefore transaction.FileSnapshot) (AirRecoveryReceipt, error) {
	ledger := plan.activeLedger
	now := s.now().UTC()
	ledger.State = AirRecoveryStateAwaitingHostRoundtrip
	ledger.RecoveredAt = &now
	data, err := encodeAirLedger(ledger)
	if err != nil {
		return AirRecoveryReceipt{}, err
	}
	ledgerPost, written, err := s.guardedWrite(s.airLedgerPath(), ledgerBefore, data, 0o600)
	if err != nil {
		if written {
			_ = s.restore(s.airLedgerPath(), ledgerBefore, ledgerPost)
		}
		return AirRecoveryReceipt{}, errors.New("finalize prepared Air recovery ledger")
	}
	return AirRecoveryReceipt{
		SurfaceID: airSurfaceID, State: AirRecoveryStateAwaitingHostRoundtrip,
		Action: "resumed", CaseID: ledger.CaseID, RecoveryGeneration: ledger.RecoveryGeneration,
	}, nil
}

func recoveryReceipt(plan AirRecoveryPlan, state, action string) AirRecoveryReceipt {
	return AirRecoveryReceipt{
		SurfaceID: airSurfaceID, State: state, Action: action,
		CaseID: plan.CaseID, RecoveryGeneration: plan.RecoveryGeneration,
	}
}

type committedRecoveryArtifact struct {
	path   string
	before transaction.FileSnapshot
	post   transaction.FileSnapshot
}

func (s Store) failAirRecovery(message string, committed []committedRecoveryArtifact) error {
	if err := s.rollbackAirRecovery(committed); err != nil {
		return fmt.Errorf("%s; recovery rollback also failed", message)
	}
	return errors.New(message)
}

func (s Store) rollbackAirRecovery(committed []committedRecoveryArtifact) error {
	var rollbackErrors []error
	for index := len(committed) - 1; index >= 0; index-- {
		artifact := committed[index]
		if err := s.restore(artifact.path, artifact.before, artifact.post); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	return errors.Join(rollbackErrors...)
}

// PlanAirSettlement observes the active case and never changes Air, the
// journal, or quarantine.
func (s Store) PlanAirSettlement(options AirSettleOptions) (AirSettlementPlan, error) {
	if options.CaseID == "" || !airCaseIDPattern.MatchString(options.CaseID) {
		return AirSettlementPlan{}, errors.New("Air settlement requires a valid case ID")
	}
	ledger, ledgerBefore, err := s.captureAirLedger()
	if err != nil {
		return AirSettlementPlan{}, err
	}
	if !ledgerBefore.Exists || ledger.CaseID != options.CaseID {
		return AirSettlementPlan{}, errors.New("Air recovery case is not active")
	}
	quarantine, err := s.captureRecovery(s.airQuarantinePath(ledger.CaseID))
	if err != nil {
		return AirSettlementPlan{}, errors.New("inspect Air recovery quarantine")
	}
	if ledger.State == AirRecoveryStateSettled {
		action := "already-settled"
		if quarantine.Exists {
			if quarantine.SHA256 != ledger.QuarantineSHA256 {
				return AirSettlementPlan{}, errors.New("settled Air quarantine changed")
			}
			action = "remove-settled-quarantine"
		}
		return AirSettlementPlan{
			SurfaceID: airSurfaceID, State: AirRecoveryStateSettled, Action: action,
			CaseID: ledger.CaseID, RecoveryGeneration: ledger.RecoveryGeneration,
			ledger: ledger, ledgerBefore: ledgerBefore, quarantine: quarantine,
		}, nil
	}
	if ledger.State == AirRecoveryStatePrepared {
		return AirSettlementPlan{}, errors.New("Air recovery is prepared but not finalized")
	}
	if !quarantine.Exists || quarantine.SHA256 != ledger.QuarantineSHA256 {
		return AirSettlementPlan{}, errors.New("Air recovery quarantine is unavailable")
	}
	if ledger.State == AirRecoveryStateQuarantined {
		return AirSettlementPlan{
			SurfaceID: airSurfaceID, State: AirRecoveryStateQuarantined, Action: "already-quarantined",
			CaseID: ledger.CaseID, RecoveryGeneration: ledger.RecoveryGeneration,
			ledger: ledger, ledgerBefore: ledgerBefore, quarantine: quarantine,
		}, nil
	}
	current, err := s.capture(options.AirPath)
	if err != nil || !current.Exists {
		return AirSettlementPlan{}, errors.New("inspect Air settlement configuration")
	}
	plan := AirSettlementPlan{
		SurfaceID: airSurfaceID, CaseID: ledger.CaseID,
		RecoveryGeneration: ledger.RecoveryGeneration,
		ledger:             ledger, ledgerBefore: ledgerBefore, air: current, quarantine: quarantine,
	}
	if current.SHA256 == ledger.CleanedPostimageSHA256 {
		plan.State = AirRecoveryStateAwaitingHostRoundtrip
		plan.Action = "wait"
		return plan, nil
	}
	airSidecar, err := s.capture(codexSidecarPath(options.AirPath))
	if err != nil {
		return AirSettlementPlan{}, errors.New("inspect Air settlement sidecar")
	}
	standalone, err := s.capture(options.StandalonePath)
	if err != nil || !standalone.Exists {
		return AirSettlementPlan{}, errors.New("inspect Air settlement reference")
	}
	standaloneSidecar, err := s.capture(codexSidecarPath(options.StandalonePath))
	if err != nil {
		return AirSettlementPlan{}, errors.New("inspect Air settlement reference sidecar")
	}
	inspection, err := adapters.InspectAirCodexConfig(options.AirPath, options.StandalonePath)
	if err != nil {
		return AirSettlementPlan{}, errors.New("inspect Air settlement state")
	}
	currentAfter, currentErr := s.capture(options.AirPath)
	airSidecarAfter, airSidecarErr := s.capture(codexSidecarPath(options.AirPath))
	standaloneAfter, standaloneErr := s.capture(options.StandalonePath)
	standaloneSidecarAfter, standaloneSidecarErr := s.capture(codexSidecarPath(options.StandalonePath))
	if currentErr != nil || airSidecarErr != nil || standaloneErr != nil || standaloneSidecarErr != nil ||
		!sameRecoverySnapshot(current, currentAfter) || !sameRecoverySnapshot(airSidecar, airSidecarAfter) ||
		!sameRecoverySnapshot(standalone, standaloneAfter) || !sameRecoverySnapshot(standaloneSidecar, standaloneSidecarAfter) {
		return AirSettlementPlan{}, errors.New("Air settlement inputs changed during classification")
	}
	plan.standalone = standalone
	plan.airSidecar = airSidecar
	plan.standaloneSidecar = standaloneSidecar
	switch inspection.State {
	case adapters.AirStateExternalClean, adapters.AirStateExternalHostMirror:
		plan.State = inspection.State
		plan.Action = "settle"
	case adapters.AirStateOrphanedExactFullSelection:
		removal, removalErr := adapters.PlanAirOrphanRemoval(options.AirPath, options.StandalonePath)
		if removalErr == nil && removal.ProjectionFingerprintSHA256 == ledger.ProjectionFingerprintSHA256 {
			plan.State = "reappeared-after-recovery"
			plan.Action = "wait-for-reference"
		} else {
			plan.State = "partial-or-foreign-residue"
			plan.Action = "quarantine"
		}
	default:
		plan.State = "partial-or-foreign-residue"
		plan.Action = "quarantine"
	}
	return plan, nil
}

// SettleAir updates only the AIGW-owned ledger and quarantine. It never writes
// the Air configuration.
func (s Store) SettleAir(options AirSettleOptions) (AirSettlementReceipt, error) {
	plan, err := s.PlanAirSettlement(options)
	if err != nil {
		return AirSettlementReceipt{}, err
	}
	switch plan.Action {
	case "wait":
		return AirSettlementReceipt{}, errors.New("Air host roundtrip has not changed the cleaned postimage")
	case "wait-for-reference":
		return AirSettlementReceipt{}, errors.New("reappeared Air projection lacks current standalone reference proof")
	case "already-settled":
		return settlementReceipt(plan, AirRecoveryStateSettled, "already-settled"), nil
	case "remove-settled-quarantine":
		post, removed, err := s.guardedRemove(s.airQuarantinePath(plan.CaseID), plan.quarantine)
		committed := make([]committedRecoveryArtifact, 0, 1)
		if removed {
			committed = append(committed, committedRecoveryArtifact{
				path: s.airQuarantinePath(plan.CaseID), before: plan.quarantine, post: post,
			})
		}
		if err != nil {
			return AirSettlementReceipt{}, s.failAirRecovery("remove settled Air quarantine", committed)
		}
		return settlementReceipt(plan, AirRecoveryStateSettled, "completed-settled-cleanup"), nil
	case "already-quarantined":
		return settlementReceipt(plan, AirRecoveryStateQuarantined, "already-quarantined"), nil
	case "quarantine":
		if err := s.verifyAirSettlementInputs(options, plan); err != nil {
			return AirSettlementReceipt{}, err
		}
		ledger := plan.ledger
		ledger.State = AirRecoveryStateQuarantined
		ledger.ObservedRoundtripSHA256 = plan.air.SHA256
		data, err := encodeAirLedger(ledger)
		if err != nil {
			return AirSettlementReceipt{}, err
		}
		post, written, err := s.guardedWrite(s.airLedgerPath(), plan.ledgerBefore, data, 0o600)
		committed := make([]committedRecoveryArtifact, 0, 1)
		if written {
			committed = append(committed, committedRecoveryArtifact{
				path: s.airLedgerPath(), before: plan.ledgerBefore, post: post,
			})
		}
		if err != nil {
			return AirSettlementReceipt{}, s.failAirRecovery("record quarantined Air recovery state", committed)
		}
		return settlementReceipt(plan, AirRecoveryStateQuarantined, plan.State), nil
	case "settle":
		if err := s.verifyAirSettlementInputs(options, plan); err != nil {
			return AirSettlementReceipt{}, err
		}
		return s.applyAirSettlement(plan)
	default:
		return AirSettlementReceipt{}, errors.New("unsupported Air settlement action")
	}
}

func (s Store) verifyAirSettlementInputs(options AirSettleOptions, plan AirSettlementPlan) error {
	current, err := s.capture(options.AirPath)
	if err != nil || !sameRecoverySnapshot(current, plan.air) {
		return errors.New("Air settlement configuration changed before commit")
	}
	standalone, err := s.capture(options.StandalonePath)
	if err != nil || !sameRecoverySnapshot(standalone, plan.standalone) {
		return errors.New("Air settlement reference changed before commit")
	}
	airSidecar, err := s.capture(codexSidecarPath(options.AirPath))
	if err != nil || !sameRecoverySnapshot(airSidecar, plan.airSidecar) {
		return errors.New("Air settlement sidecar changed before commit")
	}
	standaloneSidecar, err := s.capture(codexSidecarPath(options.StandalonePath))
	if err != nil || !sameRecoverySnapshot(standaloneSidecar, plan.standaloneSidecar) {
		return errors.New("Air settlement reference sidecar changed before commit")
	}
	return nil
}

func sameRecoverySnapshot(left, right transaction.FileSnapshot) bool {
	return left.Exists == right.Exists && left.SHA256 == right.SHA256 && left.Mode == right.Mode && bytes.Equal(left.Data, right.Data)
}

type airRecoveryInputs struct {
	air               transaction.FileSnapshot
	airSidecar        transaction.FileSnapshot
	standalone        transaction.FileSnapshot
	standaloneSidecar transaction.FileSnapshot
}

func (s Store) captureAirRecoveryInputs(options AirRecoverOptions) (airRecoveryInputs, error) {
	air, err := s.capture(options.AirPath)
	if err != nil || !air.Exists {
		return airRecoveryInputs{}, errors.New("inspect Air recovery configuration")
	}
	airSidecar, err := s.capture(codexSidecarPath(options.AirPath))
	if err != nil {
		return airRecoveryInputs{}, errors.New("inspect Air recovery sidecar")
	}
	standalone, err := s.capture(options.StandalonePath)
	if err != nil || !standalone.Exists {
		return airRecoveryInputs{}, errors.New("inspect Air recovery reference")
	}
	standaloneSidecar, err := s.capture(codexSidecarPath(options.StandalonePath))
	if err != nil {
		return airRecoveryInputs{}, errors.New("inspect Air recovery reference sidecar")
	}
	return airRecoveryInputs{
		air: air, airSidecar: airSidecar, standalone: standalone, standaloneSidecar: standaloneSidecar,
	}, nil
}

func sameAirRecoveryInputs(left, right airRecoveryInputs) bool {
	return sameRecoverySnapshot(left.air, right.air) &&
		sameRecoverySnapshot(left.airSidecar, right.airSidecar) &&
		sameRecoverySnapshot(left.standalone, right.standalone) &&
		sameRecoverySnapshot(left.standaloneSidecar, right.standaloneSidecar)
}

func (s Store) verifyAirRecoveryInputs(options AirRecoverOptions, plan AirRecoveryPlan) error {
	current, err := s.captureAirRecoveryInputs(options)
	if err != nil || !sameAirRecoveryInputs(current, airRecoveryInputs{
		air: plan.air, airSidecar: plan.airSidecar, standalone: plan.standalone, standaloneSidecar: plan.standaloneSidecar,
	}) {
		return errors.New("Air recovery inputs changed before commit")
	}
	return nil
}

func (s Store) guardedWrite(path string, expected transaction.FileSnapshot, data []byte, defaultMode os.FileMode) (transaction.FileSnapshot, bool, error) {
	mode := defaultMode
	if expected.Exists {
		mode = expected.Mode
	}
	desired := desiredSnapshot(data, mode)
	if sameRecoverySnapshot(expected, desired) {
		return expected, false, nil
	}
	observed, writeErr := s.write(path, expected, data, defaultMode)
	if writeErr == nil && sameRecoverySnapshot(observed, desired) {
		return desired, true, nil
	}
	current, captureErr := captureStorePath(s, path)
	if captureErr == nil && sameRecoverySnapshot(current, desired) {
		if writeErr != nil {
			return desired, true, errors.New("guarded write reported failure after commit")
		}
		return desired, true, nil
	}
	if writeErr != nil {
		return transaction.FileSnapshot{}, false, errors.New("guarded write failed before a confirmed commit")
	}
	return transaction.FileSnapshot{}, false, errors.New("guarded write postimage changed")
}

func (s Store) guardedRemove(path string, expected transaction.FileSnapshot) (transaction.FileSnapshot, bool, error) {
	if !expected.Exists {
		return transaction.FileSnapshot{}, false, nil
	}
	observed, removeErr := s.remove(path, expected)
	if removeErr == nil && !observed.Exists {
		return transaction.FileSnapshot{}, true, nil
	}
	current, captureErr := captureStorePath(s, path)
	if captureErr == nil && !current.Exists {
		if removeErr != nil {
			return transaction.FileSnapshot{}, true, errors.New("guarded remove reported failure after commit")
		}
		return transaction.FileSnapshot{}, true, nil
	}
	if removeErr != nil {
		return transaction.FileSnapshot{}, false, errors.New("guarded remove failed before a confirmed commit")
	}
	return transaction.FileSnapshot{}, false, errors.New("guarded remove postimage changed")
}

func codexSidecarPath(configPath string) string {
	return configPath + ".aigw-state.json"
}

func (s Store) applyAirSettlement(plan AirSettlementPlan) (AirSettlementReceipt, error) {
	ledger := plan.ledger
	now := s.now().UTC()
	ledger.State = AirRecoveryStateSettled
	ledger.SettledAt = &now
	ledger.ObservedRoundtripSHA256 = plan.air.SHA256
	data, err := encodeAirLedger(ledger)
	if err != nil {
		return AirSettlementReceipt{}, err
	}
	committed := make([]committedRecoveryArtifact, 0, 2)
	ledgerPost, ledgerWritten, err := s.guardedWrite(s.airLedgerPath(), plan.ledgerBefore, data, 0o600)
	if ledgerWritten {
		committed = append(committed, committedRecoveryArtifact{
			path: s.airLedgerPath(), before: plan.ledgerBefore, post: ledgerPost,
		})
	}
	if err != nil {
		return AirSettlementReceipt{}, s.failAirRecovery("record settled Air recovery state", committed)
	}
	quarantinePost, quarantineRemoved, err := s.guardedRemove(s.airQuarantinePath(plan.CaseID), plan.quarantine)
	if quarantineRemoved {
		committed = append(committed, committedRecoveryArtifact{
			path: s.airQuarantinePath(plan.CaseID), before: plan.quarantine, post: quarantinePost,
		})
	}
	if err != nil {
		return AirSettlementReceipt{}, s.failAirRecovery("remove Air quarantine", committed)
	}
	return settlementReceipt(plan, AirRecoveryStateSettled, "settled"), nil
}

func settlementReceipt(plan AirSettlementPlan, state, action string) AirSettlementReceipt {
	return AirSettlementReceipt{
		SurfaceID: airSurfaceID, State: state, Action: action,
		CaseID: plan.CaseID, RecoveryGeneration: plan.RecoveryGeneration,
	}
}
