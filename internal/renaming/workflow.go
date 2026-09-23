package renaming

import (
	"context"
	"fmt"
)

// RenameRoute updates one Route identity and all Client Binding references as one transaction.
func (s Renamer) RenameRoute(ctx context.Context, oldID, newID string, dryRun bool) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	cfg, err := s.Config.Load()
	if err != nil {
		return Plan{}, err
	}
	plan, err := planRoute(cfg, oldID, newID)
	if err != nil || dryRun {
		return plan, err
	}
	if err := s.Synchronizer.Commit(ctx, cfg, plan.Config, "route rename"); err != nil {
		return Plan{}, err
	}
	plan.Status, plan.Actions.Backup = StatusApplied, "refreshed"
	return plan, nil
}

// RenameAccount prepares target credentials and commits the new Account identity.
// Source and prepared target slots remain available after a failed commit for retry or rollback.
func (s Renamer) RenameAccount(ctx context.Context, oldID, newID string, dryRun bool) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	cfg, err := s.Config.Load()
	if err != nil {
		return Plan{}, err
	}
	plan, err := planAccount(cfg, oldID, newID)
	if err != nil {
		return Plan{}, err
	}
	plan, err = planCredentialCopies(s, plan)
	if err != nil || dryRun {
		return plan, err
	}
	if plan.blockedReason != "" {
		return Plan{}, fmt.Errorf("Account rename is blocked: %s", plan.blockedReason)
	}
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	if err := applyCredentialCopies(s, plan); err != nil {
		return Plan{}, fmt.Errorf("Prepare target credential slots; source and target slots were retained: %w", err)
	}
	if err := s.Synchronizer.Commit(ctx, cfg, plan.Config, "account rename"); err != nil {
		return Plan{}, fmt.Errorf("Account rename configuration commit failed; source and target credential slots were retained: %w", err)
	}
	plan.Status, plan.Actions.Backup = StatusApplied, "refreshed"
	return plan, nil
}

// FinalizeAccount converges a verified rollback baseline before retiring old credential slots.
func (s Renamer) FinalizeAccount(ctx context.Context, oldID, newID string, dryRun bool, options FinalizeOptions) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	plan, err := planFinalize(s, oldID, newID, options)
	if err != nil || dryRun || plan.Status == StatusAlreadyFinalized {
		return plan, err
	}
	if plan.blockedReason != "" {
		return Plan{}, fmt.Errorf("Account finalization confirmation required: %s", plan.blockedReason)
	}
	return applyFinalize(ctx, s, plan)
}
