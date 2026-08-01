package renaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/providers"
	"aigw-cli/internal/secrets"
)

func planFinalize(deps Dependencies, oldID, newID string, options FinalizeOptions) (Plan, error) {
	state, err := deps.Config.CaptureVerifiedBackupState()
	if err != nil {
		return Plan{}, fmt.Errorf("Load current configuration and verified checkpoint: %w", err)
	}
	cfg := state.Current
	if _, exists := cfg.Accounts[oldID]; exists {
		return Plan{}, fmt.Errorf("source account %q still exists in current configuration", oldID)
	}
	targetAccount, exists := cfg.Accounts[newID]
	if !exists {
		return Plan{}, fmt.Errorf("target account %q does not exist in current configuration", newID)
	}
	references := make([]string, 0, len(cfg.Profiles))
	for profileID, profile := range cfg.Profiles {
		if profile.Account == oldID {
			return Plan{}, fmt.Errorf("profile %q still references source account %q", profileID, oldID)
		}
		if profile.Account == newID {
			references = append(references, "profiles."+profileID+".account")
		}
	}
	sort.Strings(references)
	if !configsSemanticallyEqual(cfg, state.Checkpoint.Config) {
		return Plan{}, errors.New("verified checkpoint does not match current configuration")
	}
	if !coversAllAdmittedClients(state.Checkpoint.Clients) {
		return Plan{}, errors.New("verified checkpoint does not cover all admitted clients")
	}

	targetToken, targetTokenPresent, err := readOptionalToken(deps.Secrets, newID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read target API token credential slot: %w", err)
	}
	if !targetTokenPresent {
		return Plan{}, fmt.Errorf("target API token for account %q is unavailable", newID)
	}
	sourceToken, sourceTokenPresent, err := readOptionalToken(deps.Secrets, oldID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read source API token credential slot: %w", err)
	}
	sourceProbe, sourceProbePresent, err := readOptionalProbeCredential(deps.Accounts, oldID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read source account probe credential slot: %w", err)
	}
	targetProbe, targetProbePresent, err := readOptionalProbeCredential(deps.Accounts, newID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read target account probe credential slot: %w", err)
	}

	backupConverged := state.Snapshot.Backup.Exists &&
		state.Snapshot.Backup.Mode == expectedPersistedMode() &&
		bytes.Equal(state.Snapshot.Backup.Data, state.Snapshot.Config.Data)
	plan := Plan{
		Resource:           "account",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
		AffectedReferences: references,
		Actions: Actions{
			Configuration:  "already-renamed",
			APIToken:       "already-absent",
			AccountProbe:   "already-absent",
			Authentication: "unchanged",
			Backup:         "converge-to-verified-current",
		},
		ExternalTODOs: []string{},
		Account:       targetAccount,
		Finalize:      true,
		snapshot:      state.Snapshot,
	}
	if backupConverged {
		plan.Actions.Backup = "already-converged"
	}

	blocked := make([]string, 0, 2)
	if sourceTokenPresent {
		rotated := !secretValuesEqual(sourceToken, targetToken)
		if rotated && !options.ConfirmAPITokenRotation {
			plan.Actions.APIToken = "rotation-confirmation-required"
			plan.ExternalTODOs = append(plan.ExternalTODOs, "Re-run `aigw verify --for all`, then pass --confirm-api-token-rotation")
			blocked = append(blocked, "API token rotation confirmation is required")
		} else if secrets.IsReadOnly(deps.Secrets) {
			plan.Actions.APIToken = "external-cleanup-required"
			plan.externalTokenCleanup = true
			plan.ExternalTODOs = append(plan.ExternalTODOs, "Unset "+secrets.EnvironmentKey(oldID)+" outside AIGW, then retry finalization")
		} else {
			plan.deleteToken = true
			if rotated {
				plan.Actions.APIToken = "delete-confirmed-rotated-source"
			} else {
				plan.Actions.APIToken = "delete-source"
			}
		}
	}

	if sourceProbePresent {
		if !targetProbePresent {
			return Plan{}, fmt.Errorf("target account probe credential for account %q is unavailable", newID)
		}
		rotated := !probeCredentialsEqual(sourceProbe, targetProbe)
		if rotated && !options.ConfirmAccountProbeRotation {
			plan.Actions.AccountProbe = "rotation-confirmation-required"
			plan.ExternalTODOs = append(plan.ExternalTODOs, "Run `aigw balance "+newID+"`, then pass --confirm-account-probe-rotation")
			blocked = append(blocked, "account probe credential rotation confirmation is required")
		} else {
			plan.deleteProbe = true
			if rotated {
				plan.Actions.AccountProbe = "verify-balance-and-delete-confirmed-rotated-source"
				plan.verifyProbe = true
			} else {
				plan.Actions.AccountProbe = "delete-source"
			}
		}
	}

	if len(blocked) > 0 {
		plan.Status = "blocked"
		plan.blockedReason = strings.Join(blocked, "; ")
	} else if plan.externalTokenCleanup {
		plan.Status = "blocked"
	} else if backupConverged && !sourceTokenPresent && !sourceProbePresent {
		plan.Status = "already-finalized"
	}
	return plan, nil
}

func configsSemanticallyEqual(left, right configuration.Config) bool {
	normalizeConfigIdentity(&left)
	normalizeConfigIdentity(&right)
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func normalizeConfigIdentity(cfg *configuration.Config) {
	cfg.Normalize()
	for id, providerAccount := range cfg.Accounts {
		providerAccount.ID = ""
		cfg.Accounts[id] = providerAccount
	}
	for id, profile := range cfg.Profiles {
		profile.ID = ""
		cfg.Profiles[id] = profile
	}
}

func coversAllAdmittedClients(clients []string) bool {
	admitted := configuration.AdmittedClientIDs()
	if len(clients) != len(admitted) {
		return false
	}
	seen := make(map[string]bool, len(clients))
	for _, client := range clients {
		if seen[client] || !configuration.IsAdmittedClient(client) {
			return false
		}
		seen[client] = true
	}
	for _, client := range admitted {
		if !seen[client] {
			return false
		}
	}
	return true
}

func applyFinalize(ctx context.Context, deps Dependencies, plan Plan) (Plan, error) {
	if plan.verifyProbe {
		if err := verifyFinalizedAccountProbe(ctx, deps, plan); err != nil {
			return Plan{}, err
		}
	}
	if _, err := deps.Config.ConvergeVerifiedBackup(plan.snapshot); err != nil {
		return Plan{}, fmt.Errorf("Converge verified configuration backup: %w", err)
	}
	plan.Actions.Backup = "converged"

	cleanupErrors := make([]error, 0, 2)
	if plan.deleteToken {
		if err := deps.Secrets.Delete(plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete source API token credential slot: %w", err))
		} else if _, present, err := readOptionalToken(deps.Secrets, plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("verify source API token credential deletion: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, errors.New("verify source API token credential deletion: source slot still exists"))
		} else {
			plan.Actions.APIToken = "source-deleted"
		}
	}
	if plan.externalTokenCleanup {
		if _, present, err := readOptionalToken(deps.Secrets, plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("check external source API token cleanup: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("unset %s outside AIGW, then retry finalization", secrets.EnvironmentKey(plan.OldID)))
		} else {
			plan.Actions.APIToken = "already-absent"
		}
	}
	if plan.deleteProbe {
		if err := deps.Accounts.Delete(plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete source account probe credential slot: %w", err))
		} else if _, present, err := readOptionalProbeCredential(deps.Accounts, plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("verify source account probe credential deletion: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, errors.New("verify source account probe credential deletion: source slot still exists"))
		} else {
			plan.Actions.AccountProbe = "source-deleted"
		}
	}
	if len(cleanupErrors) > 0 {
		return Plan{}, fmt.Errorf("account finalization incomplete after backup convergence: %w", errors.Join(cleanupErrors...))
	}
	plan.Status = "finalized"
	return plan, nil
}

func verifyFinalizedAccountProbe(ctx context.Context, deps Dependencies, plan Plan) error {
	providerAccount := plan.Account
	if providerAccount.AccountProbe == nil {
		return fmt.Errorf("target account %q does not declare precise diagnostics", plan.NewID)
	}
	if !providers.Supports(providerAccount.AccountProbe.Kind) {
		return fmt.Errorf("target account diagnostics provider %q is not included in this AIGW version", providerAccount.AccountProbe.Kind)
	}
	apiToken, err := deps.Secrets.Get(plan.NewID)
	if err != nil {
		return fmt.Errorf("Read target API token for balance verification: %w", err)
	}
	credential, err := deps.Accounts.Get(plan.NewID)
	if err != nil {
		return fmt.Errorf("Read target account probe credential for balance verification: %w", err)
	}
	providerAccount.ID = plan.NewID
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := providers.Probe(probeCtx, deps.HTTP, providerAccount, apiToken, credential); err != nil {
		return fmt.Errorf("Target account balance verification failed: %w", err)
	}
	return nil
}
