package recovery

import (
	"errors"
	"fmt"

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
	ProjectionFingerprintSHA256 string `json:"projection_fingerprint_sha256"`
	ConfigPreimageSHA256        string `json:"config_preimage_sha256"`
	CleanedPostimageSHA256      string `json:"cleaned_postimage_sha256"`
	QuarantineSHA256            string `json:"quarantine_sha256"`

	removal          adapters.AirOrphanRemovalPlan
	ledgerBefore     transaction.FileSnapshot
	quarantineBefore transaction.FileSnapshot
	preparedLedger   []byte
	finalLedger      []byte
	activeLedger     airLedger
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

	ledger       airLedger
	ledgerBefore transaction.FileSnapshot
	air          transaction.FileSnapshot
	quarantine   transaction.FileSnapshot
}

// AirSettlementReceipt is the bounded result of an applied settlement.
type AirSettlementReceipt struct {
	SurfaceID          string `json:"surface_id"`
	State              string `json:"state"`
	Action             string `json:"action"`
	CaseID             string `json:"case_id"`
	RecoveryGeneration int    `json:"recovery_generation"`
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

	removal, err := adapters.PlanAirOrphanRemoval(options.AirPath, options.StandalonePath)
	if err != nil {
		return AirRecoveryPlan{}, errors.New("Air is not an exact removable orphan")
	}
	generation := 1
	if ledgerBefore.Exists {
		generation = ledger.RecoveryGeneration + 1
	}
	caseID := airCaseID(generation, removal.Preimage.SHA256)
	quarantineBefore, err := s.capture(s.airQuarantinePath(caseID))
	if err != nil {
		return AirRecoveryPlan{}, errors.New("inspect Air recovery quarantine")
	}
	if quarantineBefore.Exists {
		return AirRecoveryPlan{}, errors.New("Air recovery quarantine already exists for the preview case")
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
		Action:                      "quarantine-and-clean",
		CaseID:                      caseID,
		RecoveryGeneration:          generation,
		ProjectionFingerprintSHA256: removal.ProjectionFingerprintSHA256,
		ConfigPreimageSHA256:        removal.Preimage.SHA256,
		CleanedPostimageSHA256:      removal.Cleaned.SHA256,
		QuarantineSHA256:            removal.Preimage.SHA256,
		removal:                     removal,
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

	committed := make([]committedRecoveryArtifact, 0, 4)
	quarantinePost, err := s.write(
		s.airQuarantinePath(plan.CaseID),
		plan.quarantineBefore,
		plan.removal.Preimage.Data,
		0o600,
	)
	if err != nil {
		return AirRecoveryReceipt{}, errors.New("write Air recovery quarantine")
	}
	committed = append(committed, committedRecoveryArtifact{
		path: s.airQuarantinePath(plan.CaseID), before: plan.quarantineBefore, post: quarantinePost,
	})

	preparedPost, err := s.write(s.airLedgerPath(), plan.ledgerBefore, plan.preparedLedger, 0o600)
	if err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("prepare Air recovery ledger", committed)
	}
	committed = append(committed, committedRecoveryArtifact{path: s.airLedgerPath(), before: plan.ledgerBefore, post: preparedPost})

	airPost, err := s.write(options.AirPath, plan.removal.Preimage, plan.removal.Cleaned.Data, plan.removal.Cleaned.Mode)
	if err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("clean exact Air orphan", committed)
	}
	committed = append(committed, committedRecoveryArtifact{path: options.AirPath, before: plan.removal.Preimage, post: airPost})

	if _, err := s.write(s.airLedgerPath(), preparedPost, plan.finalLedger, 0o600); err != nil {
		return AirRecoveryReceipt{}, s.failAirRecovery("finalize Air recovery ledger", committed)
	}
	return recoveryReceipt(plan, AirRecoveryStateAwaitingHostRoundtrip, "quarantined-and-cleaned"), nil
}

func (s Store) resumePreparedAirRecovery(options AirRecoverOptions, plan AirRecoveryPlan) (AirRecoveryReceipt, error) {
	ledger := plan.activeLedger
	quarantine, err := s.capture(s.airQuarantinePath(ledger.CaseID))
	if err != nil || !quarantine.Exists || quarantine.SHA256 != ledger.QuarantineSHA256 {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery quarantine is unavailable")
	}
	current, err := s.capture(options.AirPath)
	if err != nil || !current.Exists {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery configuration is unavailable")
	}
	if current.SHA256 == ledger.CleanedPostimageSHA256 {
		return s.finalizePreparedAirLedger(plan, plan.ledgerBefore)
	}
	if current.SHA256 != ledger.ConfigPreimageSHA256 {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery preimage changed")
	}
	removal, err := adapters.PlanAirOrphanRemoval(options.AirPath, options.StandalonePath)
	if err != nil || removal.ProjectionFingerprintSHA256 != ledger.ProjectionFingerprintSHA256 || removal.Cleaned.SHA256 != ledger.CleanedPostimageSHA256 {
		return AirRecoveryReceipt{}, errors.New("prepared Air recovery no longer matches the exact orphan")
	}
	airPost, err := s.write(options.AirPath, current, removal.Cleaned.Data, removal.Cleaned.Mode)
	if err != nil {
		return AirRecoveryReceipt{}, errors.New("resume exact Air orphan cleanup")
	}
	receipt, finalizeErr := s.finalizePreparedAirLedger(plan, plan.ledgerBefore)
	if finalizeErr != nil {
		if rollbackErr := s.restore(options.AirPath, current, airPost); rollbackErr != nil {
			return AirRecoveryReceipt{}, fmt.Errorf("%w; Air rollback also failed", finalizeErr)
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
	if _, err := s.write(s.airLedgerPath(), ledgerBefore, data, 0o600); err != nil {
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
	for index := len(committed) - 1; index >= 0; index-- {
		artifact := committed[index]
		if err := s.restore(artifact.path, artifact.before, artifact.post); err != nil {
			return err
		}
	}
	return nil
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
	quarantine, err := s.capture(s.airQuarantinePath(ledger.CaseID))
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
	inspection, err := adapters.InspectAirCodexConfig(options.AirPath, options.StandalonePath)
	if err != nil {
		return AirSettlementPlan{}, errors.New("inspect Air settlement state")
	}
	switch inspection.State {
	case "external-clean", "external-host-mirror":
		plan.State = inspection.State
		plan.Action = "settle"
	case "orphaned-aigw-marker":
		removal, removalErr := adapters.PlanAirOrphanRemoval(options.AirPath, options.StandalonePath)
		if removalErr == nil && removal.ProjectionFingerprintSHA256 == ledger.ProjectionFingerprintSHA256 {
			plan.State = "reappeared-after-recovery"
		} else {
			plan.State = "partial-or-foreign-residue"
		}
		plan.Action = "quarantine"
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
	case "already-settled":
		return settlementReceipt(plan, AirRecoveryStateSettled, "already-settled"), nil
	case "remove-settled-quarantine":
		if _, err := s.remove(s.airQuarantinePath(plan.CaseID), plan.quarantine); err != nil {
			return AirSettlementReceipt{}, errors.New("remove settled Air quarantine")
		}
		return settlementReceipt(plan, AirRecoveryStateSettled, "completed-settled-cleanup"), nil
	case "quarantine":
		ledger := plan.ledger
		ledger.State = AirRecoveryStateQuarantined
		ledger.ObservedRoundtripSHA256 = plan.air.SHA256
		data, err := encodeAirLedger(ledger)
		if err != nil {
			return AirSettlementReceipt{}, err
		}
		if _, err := s.write(s.airLedgerPath(), plan.ledgerBefore, data, 0o600); err != nil {
			return AirSettlementReceipt{}, errors.New("record quarantined Air recovery state")
		}
		return settlementReceipt(plan, AirRecoveryStateQuarantined, plan.State), nil
	case "settle":
		return s.applyAirSettlement(plan)
	default:
		return AirSettlementReceipt{}, errors.New("unsupported Air settlement action")
	}
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
	ledgerPost, err := s.write(s.airLedgerPath(), plan.ledgerBefore, data, 0o600)
	if err != nil {
		return AirSettlementReceipt{}, errors.New("record settled Air recovery state")
	}
	if _, err := s.remove(s.airQuarantinePath(plan.CaseID), plan.quarantine); err != nil {
		if rollbackErr := s.restore(s.airLedgerPath(), plan.ledgerBefore, ledgerPost); rollbackErr != nil {
			return AirSettlementReceipt{}, errors.New("remove Air quarantine; ledger rollback also failed")
		}
		return AirSettlementReceipt{}, errors.New("remove Air quarantine")
	}
	return settlementReceipt(plan, AirRecoveryStateSettled, "settled"), nil
}

func settlementReceipt(plan AirSettlementPlan, state, action string) AirSettlementReceipt {
	return AirSettlementReceipt{
		SurfaceID: airSurfaceID, State: state, Action: action,
		CaseID: plan.CaseID, RecoveryGeneration: plan.RecoveryGeneration,
	}
}
