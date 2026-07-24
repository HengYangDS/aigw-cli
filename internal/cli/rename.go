package cli

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

type renameActions struct {
	Configuration  string `json:"configuration"`
	APIToken       string `json:"api_token"`
	AccountProbe   string `json:"account_probe"`
	Authentication string `json:"authentication"`
	Backup         string `json:"backup"`
}

type renamePlan struct {
	Resource           string        `json:"resource"`
	OldID              string        `json:"old_id"`
	NewID              string        `json:"new_id"`
	Status             string        `json:"status"`
	AffectedReferences []string      `json:"affected_references"`
	Actions            renameActions `json:"actions"`
	ExternalTODOs      []string      `json:"external_todos"`

	config               domain.Config  `json:"-"`
	profile              domain.Profile `json:"-"`
	account              domain.Account `json:"-"`
	tokenCopy            tokenCopyPlan  `json:"-"`
	probeCopy            probeCopyPlan  `json:"-"`
	blockedReason        string         `json:"-"`
	finalize             bool           `json:"-"`
	finalSnapshot        config.VerifiedBackupSnapshot
	deleteToken          bool `json:"-"`
	deleteProbe          bool `json:"-"`
	verifyProbe          bool `json:"-"`
	externalTokenCleanup bool `json:"-"`
}

type tokenCopyPlan struct {
	value string
	copy  bool
}

type probeCopyPlan struct {
	value account.Credential
	copy  bool
}

type accountFinalizeOptions struct {
	confirmAPITokenRotation     bool
	confirmAccountProbeRotation bool
}

func resolveRenameIDs(app *App, resource string, args []string, choices []Choice) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	if !app.Interactive {
		return "", "", fmt.Errorf("%s rename requires <old> <new> in non-interactive mode", resource)
	}
	if app.Prompt == nil {
		return "", "", fmt.Errorf("%s rename requires an interactive prompt or explicit <old> <new> arguments", resource)
	}

	oldID := ""
	if len(args) == 1 {
		oldID = args[0]
	} else {
		if len(choices) == 0 {
			return "", "", fmt.Errorf("No %ss are configured", resource)
		}
		selected, err := app.Prompt.Select("Select the "+resource+" to rename: ", choices)
		if err != nil {
			return "", "", fmt.Errorf("Select %s to rename: %w", resource, err)
		}
		oldID = selected
	}
	newID, err := app.Prompt.Text("New " + resource + " ID: ")
	if err != nil {
		return "", "", fmt.Errorf("Read new %s ID: %w", resource, err)
	}
	return oldID, strings.TrimSpace(newID), nil
}

func profileRenameChoices(cfg domain.Config) []Choice {
	names := sortedProfileNames(cfg)
	choices := make([]Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, Choice{Value: name, Label: profileChoiceLabel(cfg.Profiles[name])})
	}
	return choices
}

func accountRenameChoices(cfg domain.Config) []Choice {
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, Choice{Value: name, Label: cfg.Accounts[name].Label})
	}
	return choices
}

func planAccountRename(cfg domain.Config, oldID, newID string) (renamePlan, error) {
	if !domain.ValidProfileName(newID) {
		return renamePlan{}, fmt.Errorf("Invalid new account ID %q", newID)
	}
	providerAccount, ok := cfg.Accounts[oldID]
	if !ok {
		return renamePlan{}, fmt.Errorf("Unknown account %q", oldID)
	}
	if _, exists := cfg.Accounts[newID]; exists {
		return renamePlan{}, fmt.Errorf("Account %q already exists", newID)
	}

	next := cloneConfig(cfg)
	delete(next.Accounts, oldID)
	providerAccount.ID = newID
	next.Accounts[newID] = providerAccount
	references := make([]string, 0, len(next.Profiles))
	for profileID, profile := range next.Profiles {
		if profile.Account != oldID {
			continue
		}
		profile.Account = newID
		next.Profiles[profileID] = profile
		references = append(references, "profiles."+profileID+".account")
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return renamePlan{}, fmt.Errorf("Validate account rename: %w", err)
	}
	authenticationAction := "unchanged"
	if codexAuthenticationChanged(cfg, next) {
		authenticationAction = "rebind-codex"
	}

	return renamePlan{
		Resource:           "account",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
		AffectedReferences: references,
		Actions: renameActions{
			Configuration:  "rename-and-update-profile-references",
			APIToken:       "inspect",
			AccountProbe:   "inspect",
			Authentication: authenticationAction,
			Backup:         "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		config:        next,
		account:       providerAccount,
	}, nil
}

func planProfileRename(cfg domain.Config, oldID, newID string) (renamePlan, error) {
	if !domain.ValidProfileName(newID) {
		return renamePlan{}, fmt.Errorf("Invalid new profile ID %q", newID)
	}
	profile, ok := cfg.Profiles[oldID]
	if !ok {
		return renamePlan{}, fmt.Errorf("Unknown profile %q", oldID)
	}
	if _, exists := cfg.Profiles[newID]; exists {
		return renamePlan{}, fmt.Errorf("Profile %q already exists", newID)
	}

	next := cloneConfig(cfg)
	delete(next.Profiles, oldID)
	next.Profiles[newID] = profile
	references := make([]string, 0, 1+len(next.Routes.Overrides))
	if next.Routes.Default == oldID {
		next.Routes.Default = newID
		references = append(references, "routes.default")
	}
	for client, profileID := range next.Routes.Overrides {
		if profileID != oldID {
			continue
		}
		next.Routes.Overrides[client] = newID
		references = append(references, "routes.overrides."+client)
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return renamePlan{}, fmt.Errorf("Validate profile rename: %w", err)
	}

	return renamePlan{
		Resource:           "profile",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
		AffectedReferences: references,
		Actions: renameActions{
			Configuration:  "rename-and-update-references",
			APIToken:       "unchanged",
			AccountProbe:   "unchanged",
			Authentication: "unchanged",
			Backup:         "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		config:        next,
		profile:       profile,
	}, nil
}

func planAccountCredentialCopies(app *App, plan renamePlan) (renamePlan, error) {
	sourceToken, sourceTokenPresent, err := readOptionalToken(app.Secrets, plan.OldID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read source API token credential slot: %w", err)
	}
	targetToken, targetTokenPresent, err := readOptionalToken(app.Secrets, plan.NewID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read target API token credential slot: %w", err)
	}
	switch {
	case !sourceTokenPresent && !targetTokenPresent:
		plan.Actions.APIToken = "absent"
	case !sourceTokenPresent && targetTokenPresent:
		return renamePlan{}, errors.New("API token credential slot state is inconsistent: target exists while source is absent")
	case sourceTokenPresent && targetTokenPresent:
		if !secretValuesEqual(sourceToken, targetToken) {
			return renamePlan{}, errors.New("API token credential slots differ")
		}
		plan.Actions.APIToken = "reuse-equal-target-and-retain-source"
	case secrets.IsReadOnly(app.Secrets):
		plan.Actions.APIToken = "provide-equal-environment-variable"
		plan.Status = "blocked"
		sourceKey := secrets.EnvironmentKey(plan.OldID)
		targetKey := secrets.EnvironmentKey(plan.NewID)
		plan.blockedReason = fmt.Sprintf("set %s to the same value as %s outside AIGW", targetKey, sourceKey)
		plan.ExternalTODOs = append(plan.ExternalTODOs, "Set "+targetKey+" to the same value as "+sourceKey+" outside AIGW")
	default:
		plan.Actions.APIToken = "copy-and-retain-source"
		plan.tokenCopy = tokenCopyPlan{value: sourceToken, copy: true}
	}

	sourceProbe, sourceProbePresent, err := readOptionalProbeCredential(app.Accounts, plan.OldID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read source account probe credential slot: %w", err)
	}
	targetProbe, targetProbePresent, err := readOptionalProbeCredential(app.Accounts, plan.NewID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read target account probe credential slot: %w", err)
	}
	switch {
	case !sourceProbePresent && !targetProbePresent:
		plan.Actions.AccountProbe = "absent"
	case !sourceProbePresent && targetProbePresent:
		return renamePlan{}, errors.New("account probe credential slot state is inconsistent: target exists while source is absent")
	case sourceProbePresent && targetProbePresent:
		if !probeCredentialsEqual(sourceProbe, targetProbe) {
			return renamePlan{}, errors.New("account probe credential slots differ")
		}
		plan.Actions.AccountProbe = "reuse-equal-target-and-retain-source"
	default:
		plan.Actions.AccountProbe = "copy-and-retain-source"
		plan.probeCopy = probeCopyPlan{value: sourceProbe, copy: true}
	}
	return plan, nil
}

func applyAccountCredentialCopies(app *App, plan renamePlan) error {
	if plan.tokenCopy.copy {
		if err := app.Secrets.Set(plan.NewID, plan.tokenCopy.value); err != nil {
			return fmt.Errorf("Copy API token credential slot: %w", err)
		}
		copied, err := app.Secrets.Get(plan.NewID)
		if err != nil {
			return fmt.Errorf("verify target API token slot: %w", err)
		}
		if !secretValuesEqual(plan.tokenCopy.value, copied) {
			return errors.New("verify target API token slot: copied value differs")
		}
	}
	if plan.probeCopy.copy {
		if err := app.Accounts.Set(plan.NewID, plan.probeCopy.value); err != nil {
			return fmt.Errorf("Copy account probe credential slot: %w", err)
		}
		copied, err := app.Accounts.Get(plan.NewID)
		if err != nil {
			return fmt.Errorf("verify target account probe credential slot: %w", err)
		}
		if !probeCredentialsEqual(plan.probeCopy.value, copied) {
			return errors.New("verify target account probe credential slot: copied value differs")
		}
	}
	return nil
}

func readOptionalToken(store secrets.Store, id string) (string, bool, error) {
	value, err := store.Get(id)
	if errors.Is(err, secrets.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func readOptionalProbeCredential(store account.Store, id string) (account.Credential, bool, error) {
	value, err := store.Get(id)
	if errors.Is(err, account.ErrNotFound) {
		return account.Credential{}, false, nil
	}
	if err != nil {
		return account.Credential{}, false, err
	}
	return value, true, nil
}

func secretValuesEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func probeCredentialsEqual(left, right account.Credential) bool {
	return secretValuesEqual(left.SystemToken, right.SystemToken) && secretValuesEqual(left.UserID, right.UserID)
}

func planAccountFinalize(app *App, oldID, newID string, options accountFinalizeOptions) (renamePlan, error) {
	state, err := app.Config.CaptureVerifiedBackupState()
	if err != nil {
		return renamePlan{}, fmt.Errorf("Load current configuration and verified checkpoint: %w", err)
	}
	cfg := state.Current
	if _, exists := cfg.Accounts[oldID]; exists {
		return renamePlan{}, fmt.Errorf("source account %q still exists in current configuration", oldID)
	}
	targetAccount, exists := cfg.Accounts[newID]
	if !exists {
		return renamePlan{}, fmt.Errorf("target account %q does not exist in current configuration", newID)
	}
	references := make([]string, 0, len(cfg.Profiles))
	for profileID, profile := range cfg.Profiles {
		if profile.Account == oldID {
			return renamePlan{}, fmt.Errorf("profile %q still references source account %q", profileID, oldID)
		}
		if profile.Account == newID {
			references = append(references, "profiles."+profileID+".account")
		}
	}
	sort.Strings(references)
	if !configsSemanticallyEqual(cfg, state.Checkpoint.Config) {
		return renamePlan{}, errors.New("verified checkpoint does not match current configuration")
	}
	if !coversAllAdmittedClients(state.Checkpoint.Clients) {
		return renamePlan{}, errors.New("verified checkpoint does not cover all admitted clients")
	}

	targetToken, targetTokenPresent, err := readOptionalToken(app.Secrets, newID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read target API token credential slot: %w", err)
	}
	if !targetTokenPresent {
		return renamePlan{}, fmt.Errorf("target API token for account %q is unavailable", newID)
	}
	sourceToken, sourceTokenPresent, err := readOptionalToken(app.Secrets, oldID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read source API token credential slot: %w", err)
	}
	sourceProbe, sourceProbePresent, err := readOptionalProbeCredential(app.Accounts, oldID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read source account probe credential slot: %w", err)
	}
	targetProbe, targetProbePresent, err := readOptionalProbeCredential(app.Accounts, newID)
	if err != nil {
		return renamePlan{}, fmt.Errorf("Read target account probe credential slot: %w", err)
	}

	backupConverged := state.Snapshot.Backup.Exists &&
		state.Snapshot.Backup.Mode == 0o600 &&
		bytes.Equal(state.Snapshot.Backup.Data, state.Snapshot.Config.Data)
	plan := renamePlan{
		Resource:           "account",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
		AffectedReferences: references,
		Actions: renameActions{
			Configuration:  "already-renamed",
			APIToken:       "already-absent",
			AccountProbe:   "already-absent",
			Authentication: "unchanged",
			Backup:         "converge-to-verified-current",
		},
		ExternalTODOs: []string{},
		account:       targetAccount,
		finalize:      true,
		finalSnapshot: state.Snapshot,
	}
	if backupConverged {
		plan.Actions.Backup = "already-converged"
	}

	blocked := make([]string, 0, 2)
	if sourceTokenPresent {
		rotated := !secretValuesEqual(sourceToken, targetToken)
		if rotated && !options.confirmAPITokenRotation {
			plan.Actions.APIToken = "rotation-confirmation-required"
			plan.ExternalTODOs = append(plan.ExternalTODOs, "Re-run `aigw verify --for all`, then pass --confirm-api-token-rotation")
			blocked = append(blocked, "API token rotation confirmation is required")
		} else if secrets.IsReadOnly(app.Secrets) {
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
			return renamePlan{}, fmt.Errorf("target account probe credential for account %q is unavailable", newID)
		}
		rotated := !probeCredentialsEqual(sourceProbe, targetProbe)
		if rotated && !options.confirmAccountProbeRotation {
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

func configsSemanticallyEqual(left, right domain.Config) bool {
	normalizeConfigIdentity(&left)
	normalizeConfigIdentity(&right)
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func normalizeConfigIdentity(cfg *domain.Config) {
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
	admitted := domain.AdmittedClientIDs()
	if len(clients) != len(admitted) {
		return false
	}
	seen := make(map[string]bool, len(clients))
	for _, client := range clients {
		if seen[client] || !domain.IsAdmittedClient(client) {
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

func applyAccountFinalize(ctx context.Context, app *App, plan renamePlan) (renamePlan, error) {
	if plan.verifyProbe {
		if err := verifyFinalizedAccountProbe(ctx, app, plan); err != nil {
			return renamePlan{}, err
		}
	}
	if _, err := app.Config.ConvergeVerifiedBackup(plan.finalSnapshot); err != nil {
		return renamePlan{}, fmt.Errorf("Converge verified configuration backup: %w", err)
	}
	plan.Actions.Backup = "converged"

	cleanupErrors := make([]error, 0, 2)
	if plan.deleteToken {
		if err := app.Secrets.Delete(plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete source API token credential slot: %w", err))
		} else if _, present, err := readOptionalToken(app.Secrets, plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("verify source API token credential deletion: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, errors.New("verify source API token credential deletion: source slot still exists"))
		} else {
			plan.Actions.APIToken = "source-deleted"
		}
	}
	if plan.externalTokenCleanup {
		if _, present, err := readOptionalToken(app.Secrets, plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("check external source API token cleanup: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("unset %s outside AIGW, then retry finalization", secrets.EnvironmentKey(plan.OldID)))
		} else {
			plan.Actions.APIToken = "already-absent"
		}
	}
	if plan.deleteProbe {
		if err := app.Accounts.Delete(plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete source account probe credential slot: %w", err))
		} else if _, present, err := readOptionalProbeCredential(app.Accounts, plan.OldID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("verify source account probe credential deletion: %w", err))
		} else if present {
			cleanupErrors = append(cleanupErrors, errors.New("verify source account probe credential deletion: source slot still exists"))
		} else {
			plan.Actions.AccountProbe = "source-deleted"
		}
	}
	if len(cleanupErrors) > 0 {
		return renamePlan{}, fmt.Errorf("account finalization incomplete after backup convergence: %w", errors.Join(cleanupErrors...))
	}
	plan.Status = "finalized"
	return plan, nil
}

func verifyFinalizedAccountProbe(ctx context.Context, app *App, plan renamePlan) error {
	providerAccount := plan.account
	if providerAccount.AccountProbe == nil {
		return fmt.Errorf("target account %q does not declare precise diagnostics", plan.NewID)
	}
	if !providers.Supports(providerAccount.AccountProbe.Kind) {
		return fmt.Errorf("target account diagnostics provider %q is not included in this AIGW version", providerAccount.AccountProbe.Kind)
	}
	apiToken, err := app.Secrets.Get(plan.NewID)
	if err != nil {
		return fmt.Errorf("Read target API token for balance verification: %w", err)
	}
	credential, err := app.Accounts.Get(plan.NewID)
	if err != nil {
		return fmt.Errorf("Read target account probe credential for balance verification: %w", err)
	}
	providerAccount.ID = plan.NewID
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := providers.Probe(probeCtx, app.HTTP, providerAccount, apiToken, credential); err != nil {
		return fmt.Errorf("Target account balance verification failed: %w", err)
	}
	return nil
}

func newAccountRenameCommand(app *App) *cobra.Command {
	var dryRun, jsonMode, finalize, confirmAPITokenRotation, confirmAccountProbeRotation bool
	cmd := &cobra.Command{
		Use: "rename [old] [new]", Short: "Rename an account and update its profile references", Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if finalize {
				if len(args) != 2 {
					return errors.New("account rename --finalize requires explicit <old> <new> arguments")
				}
				plan, err := planAccountFinalize(app, args[0], args[1], accountFinalizeOptions{
					confirmAPITokenRotation:     confirmAPITokenRotation,
					confirmAccountProbeRotation: confirmAccountProbeRotation,
				})
				if err != nil {
					return err
				}
				if dryRun || plan.Status == "already-finalized" {
					return writeRenameResult(app, plan, jsonMode)
				}
				if plan.blockedReason != "" {
					return fmt.Errorf("Account finalization confirmation required: %s", plan.blockedReason)
				}
				plan, err = applyAccountFinalize(cmd.Context(), app, plan)
				if err != nil {
					return err
				}
				return writeRenameResult(app, plan, jsonMode)
			}
			if confirmAPITokenRotation || confirmAccountProbeRotation {
				return errors.New("credential rotation confirmations require --finalize")
			}
			if len(args) < 2 && !app.Interactive {
				return errors.New("account rename requires <old> <new> in non-interactive mode")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			oldID, newID, err := resolveRenameIDs(app, "account", args, accountRenameChoices(cfg))
			if err != nil {
				return err
			}
			plan, err := planAccountRename(cfg, oldID, newID)
			if err != nil {
				return err
			}
			plan, err = planAccountCredentialCopies(app, plan)
			if err != nil {
				return err
			}
			if dryRun {
				return writeRenameResult(app, plan, jsonMode)
			}
			if plan.blockedReason != "" {
				return fmt.Errorf("Account rename is blocked: %s", plan.blockedReason)
			}
			if err := applyAccountCredentialCopies(app, plan); err != nil {
				return fmt.Errorf("Prepare target credential slots; source and target slots were retained: %w", err)
			}
			if err := commitConfigAndSync(cmd.Context(), app, cfg, plan.config, "account rename"); err != nil {
				return fmt.Errorf("Account rename configuration commit failed; source and target credential slots were retained: %w", err)
			}
			plan.Status = "applied"
			plan.Actions.Backup = "refreshed"
			return writeRenameResult(app, plan, jsonMode)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration or credentials")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	cmd.Flags().BoolVar(&finalize, "finalize", false, "Converge the verified rollback baseline and remove source credential slots")
	cmd.Flags().BoolVar(&confirmAPITokenRotation, "confirm-api-token-rotation", false, "Confirm that a different target API token was intentionally rotated and fully verified")
	cmd.Flags().BoolVar(&confirmAccountProbeRotation, "confirm-account-probe-rotation", false, "Confirm that different target account probe credentials were intentionally rotated")
	return cmd
}

func writeRenameResult(app *App, plan renamePlan, jsonMode bool) error {
	if jsonMode {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	r := renderer(app)
	isPlan := plan.Status != "applied"
	if plan.Resource == "account" {
		if plan.finalize && plan.Status == "finalized" {
			r.Title("AIGW", "Account finalization complete")
		} else if plan.finalize && plan.Status == "already-finalized" {
			r.Title("AIGW", "Account already finalized")
		} else if plan.finalize {
			r.Title("AIGW", "Account finalization plan")
		} else if isPlan {
			r.Title("AIGW", "Account rename plan")
		} else {
			r.Title("AIGW", "Account renamed")
		}
		r.Row("Previous account", plan.OldID)
		r.Row("New account", plan.NewID)
		r.Row("Label", plan.account.Label)
		if len(plan.AffectedReferences) > 0 {
			r.Row("Profile references", fmt.Sprintf("%d", len(plan.AffectedReferences)))
		}
	} else {
		if isPlan {
			r.Title("AIGW", "Profile rename plan")
		} else {
			r.Title("AIGW", "Profile renamed")
		}
		r.Row("Previous profile", plan.OldID)
		r.Row("New profile", plan.NewID)
		r.Row("Account", plan.profile.Account)
		if len(plan.AffectedReferences) > 0 {
			r.Row("Route references", fmt.Sprintf("%d", len(plan.AffectedReferences)))
		}
	}
	for _, todo := range plan.ExternalTODOs {
		r.Row("External action", todo)
	}
	switch plan.Status {
	case "blocked":
		r.Status(presentation.Warn, "Plan", "Blocked; no changes were made")
	case "planned":
		r.Success("Dry run complete; no changes were made")
	case "already-finalized":
		r.Success("The verified rollback baseline and source credential cleanup are already complete")
	case "finalized":
		r.Success("The verified rollback baseline was converged and source credential slots were removed")
	case "applied":
		if plan.Resource == "account" {
			r.Success("Configuration and target credentials are ready; source credential slots were retained for rollback")
			r.Next("aigw verify --for all")
			r.Next("aigw account rename " + plan.OldID + " " + plan.NewID + " --finalize")
		} else {
			r.Success("The account token remains in place and routes were synchronized")
		}
	}
	return nil
}
