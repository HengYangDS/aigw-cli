package renaming

import (
	"aigw-cli/internal/store"
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"aigw-cli/internal/balanceprobe"
	"aigw-cli/internal/balanceprobe/diagnostic"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
)

func planFinalize(deps Renamer, oldID, newID string, options FinalizeOptions) (Plan, error) {
	state, err := deps.Config.CaptureVerifiedBackupState()
	if err != nil {
		return Plan{}, fmt.Errorf("Load current configuration and verified checkpoint: %w", err)
	}
	cfg := state.Current
	if _, exists := cfg.Providers[oldID]; exists {
		return Plan{}, fmt.Errorf("source account %q still exists in current configuration", oldID)
	}
	targetAccount, exists := cfg.Providers[newID]
	if !exists {
		return Plan{}, fmt.Errorf("target account %q does not exist in current configuration", newID)
	}
	references := make([]string, 0, len(cfg.Models))
	for routeID, route := range cfg.Models {
		if route.Provider == newID {
			references = append(references, "routes."+routeID+".account")
		}
	}
	sort.Strings(references)
	for _, client := range cfg.EnabledClientIDs() {
		if !slices.ContainsFunc(state.Checkpoint.Clients, func(verified store.VerifiedClient) bool { return verified.Client == client }) {
			return Plan{}, fmt.Errorf("verified checkpoint does not cover enabled client %q; run `aigw check --depth client`", client)
		}
	}

	backupConverged := state.Snapshot.BackupConverged()
	plan := Plan{
		Resource:           ResourceProvider,
		OldID:              oldID,
		NewID:              newID,
		Status:             StatusPlanned,
		AffectedReferences: references,
		Actions: Actions{
			Configuration: "already_renamed",
			APIToken:      "already_absent",
			BalanceProbe:  "already_absent",
			Backup:        "converge_to_verified_current",
		},
		ExternalTODOs: []string{},
		Provider:      targetAccount,
		Finalize:      true,
		snapshot:      state.Snapshot,
	}
	if backupConverged {
		plan.Actions.Backup = "already_converged"
	}

	blocked := make([]string, 0, 2)
	sourceTokenPresent, err := planFinalToken(deps, &plan, oldID, newID, options.ConfirmAPITokenRotation, &blocked)
	if err != nil {
		return Plan{}, err
	}
	sourceProbePresent, err := planFinalProbe(deps, &plan, oldID, newID, options.ConfirmBalanceProbeRotation, &blocked)
	if err != nil {
		return Plan{}, err
	}

	if len(blocked) > 0 {
		plan.Status = StatusBlocked
		plan.blockedReason = strings.Join(blocked, "; ")
	} else if plan.externalTokenCleanup {
		plan.Status = StatusBlocked
	} else if backupConverged && !sourceTokenPresent && !sourceProbePresent {
		plan.Status = StatusAlreadyFinalized
	}
	return plan, nil
}

func planFinalToken(deps Renamer, plan *Plan, oldID, newID string, confirmed bool, blocked *[]string) (bool, error) {
	target, targetPresent, err := readOptionalToken(deps.Secrets, newID)
	if err != nil {
		return false, fmt.Errorf("Read target API token credential slot: %w", err)
	}
	source, sourcePresent, err := readOptionalToken(deps.Secrets, oldID)
	if err != nil {
		return false, fmt.Errorf("Read source API token credential slot: %w", err)
	}
	if !sourcePresent {
		return false, nil
	}
	if !targetPresent {
		return false, fmt.Errorf("target API token for account %q is unavailable", newID)
	}
	rotated := !secretValuesEqual(source, target)
	switch {
	case rotated && !confirmed:
		plan.Actions.APIToken = "rotation_confirmation_required"
		plan.ExternalTODOs = append(plan.ExternalTODOs, "Re-run `aigw check --depth client`, then pass --confirm-api-token-rotation")
		*blocked = append(*blocked, "API token rotation confirmation is required")
	case secret.IsReadOnly(deps.Secrets):
		plan.Actions.APIToken = "external_cleanup_required"
		plan.externalTokenCleanup = true
		plan.ExternalTODOs = append(plan.ExternalTODOs, "Unset "+secret.EnvironmentKey(oldID)+" outside AIGW, then retry finalization")
	default:
		plan.deleteToken = true
		plan.Actions.APIToken = "delete_source"
		if rotated {
			plan.Actions.APIToken = "delete_confirmed_rotated_source"
		}
	}
	return true, nil
}

func planFinalProbe(deps Renamer, plan *Plan, oldID, newID string, confirmed bool, blocked *[]string) (bool, error) {
	source, sourcePresent, err := readOptionalProbeCredential(deps.Providers, oldID)
	if err != nil {
		return false, fmt.Errorf("Read source account probe credential slot: %w", err)
	}
	if !sourcePresent {
		return false, nil
	}
	target, targetPresent, err := readOptionalProbeCredential(deps.Providers, newID)
	if err != nil {
		return false, fmt.Errorf("Read target account probe credential slot: %w", err)
	}
	if !targetPresent {
		return false, fmt.Errorf("target account probe credential for account %q is unavailable", newID)
	}
	rotated := !probeCredentialsEqual(source, target)
	if rotated && !confirmed {
		plan.Actions.BalanceProbe = "rotation_confirmation_required"
		plan.ExternalTODOs = append(plan.ExternalTODOs, "Run `aigw balance "+newID+"`, then pass --confirm-account-probe-rotation")
		*blocked = append(*blocked, "account probe credential rotation confirmation is required")
		return true, nil
	}
	plan.deleteProbe = true
	plan.Actions.BalanceProbe = "delete_source"
	if rotated {
		plan.Actions.BalanceProbe = "verify_balance_and_delete_confirmed_rotated_source"
		plan.verifyProbe = true
	}
	return true, nil
}

func applyFinalize(ctx context.Context, deps Renamer, plan Plan) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	if plan.verifyProbe {
		if err := verifyFinalizedBalanceProbe(ctx, deps, plan); err != nil {
			return Plan{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	if err := deps.Config.ConvergeVerifiedBackup(plan.snapshot); err != nil {
		return Plan{}, fmt.Errorf("Converge verified configuration backup: %w", err)
	}
	plan.Actions.Backup = "converged"

	cleanupErrors := make([]error, 0, 2)
	if plan.deleteToken {
		if err := deleteCredentialSlot(deps.Secrets, plan.OldID, "API token"); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		} else {
			plan.Actions.APIToken = "source_deleted"
		}
	}
	if plan.externalTokenCleanup {
		if present, err := deps.Secrets.Exists(plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("check external source API token cleanup: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("unset %s outside AIGW, then retry finalization", secret.EnvironmentKey(plan.OldID)))
		} else {
			plan.Actions.APIToken = "already_absent"
		}
	}
	if plan.deleteProbe {
		if err := deleteCredentialSlot(deps.Providers, plan.OldID, "account probe"); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		} else {
			plan.Actions.BalanceProbe = "source_deleted"
		}
	}
	if len(cleanupErrors) > 0 {
		return Plan{}, fmt.Errorf("account finalization incomplete after backup convergence: %w", errors.Join(cleanupErrors...))
	}
	plan.Status = StatusFinalized
	return plan, nil
}

func verifyFinalizedBalanceProbe(ctx context.Context, deps Renamer, plan Plan) error {
	providerAccount := plan.Provider
	if providerAccount.BalanceProbe == nil {
		return fmt.Errorf("target account %q does not declare precise diagnostics", plan.NewID)
	}
	if !balanceprobe.Supports(providerAccount.BalanceProbe.Kind) {
		return fmt.Errorf("target account diagnostics provider %q is not included in this AIGW version", providerAccount.BalanceProbe.Kind)
	}
	apiToken, err := deps.Secrets.Get(plan.NewID)
	if err != nil {
		return fmt.Errorf("Read target API token for balance verification: %w", err)
	}
	credential, err := deps.Providers.Get(plan.NewID)
	if err != nil {
		return fmt.Errorf("Read target account probe credential for balance verification: %w", err)
	}
	providerAccount.ID = plan.NewID
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := balanceprobe.Probe(probeCtx, probe.Sender{Client: deps.HTTP}, providerAccount, apiToken, diagnostic.Credential(credential)); err != nil {
		return fmt.Errorf("Target account balance verification failed: %w", err)
	}
	return nil
}
