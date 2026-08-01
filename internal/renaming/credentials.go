package renaming

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"runtime"

	"aigw-cli/internal/account"
	"aigw-cli/internal/secrets"
)

func planCredentialCopies(deps Dependencies, plan Plan) (Plan, error) {
	sourceToken, sourceTokenPresent, err := readOptionalToken(deps.Secrets, plan.OldID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read source API token credential slot: %w", err)
	}
	targetToken, targetTokenPresent, err := readOptionalToken(deps.Secrets, plan.NewID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read target API token credential slot: %w", err)
	}
	switch {
	case !sourceTokenPresent && !targetTokenPresent:
		plan.Actions.APIToken = "absent"
	case !sourceTokenPresent && targetTokenPresent:
		return Plan{}, errors.New("API token credential slot state is inconsistent: target exists while source is absent")
	case sourceTokenPresent && targetTokenPresent:
		if !secretValuesEqual(sourceToken, targetToken) {
			return Plan{}, errors.New("API token credential slots differ")
		}
		plan.Actions.APIToken = "reuse-equal-target-and-retain-source"
	case secrets.IsReadOnly(deps.Secrets):
		plan.Actions.APIToken = "provide-equal-environment-variable"
		plan.Status = "blocked"
		sourceKey := secrets.EnvironmentKey(plan.OldID)
		targetKey := secrets.EnvironmentKey(plan.NewID)
		plan.blockedReason = fmt.Sprintf("set %s to the same value as %s outside AIGW", targetKey, sourceKey)
		plan.ExternalTODOs = append(plan.ExternalTODOs, "Set "+targetKey+" to the same value as "+sourceKey+" outside AIGW")
	default:
		plan.Actions.APIToken = "copy-and-retain-source"
		plan.tokenCopy = tokenCopy{value: sourceToken, copy: true}
	}

	sourceProbe, sourceProbePresent, err := readOptionalProbeCredential(deps.Accounts, plan.OldID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read source account probe credential slot: %w", err)
	}
	targetProbe, targetProbePresent, err := readOptionalProbeCredential(deps.Accounts, plan.NewID)
	if err != nil {
		return Plan{}, fmt.Errorf("Read target account probe credential slot: %w", err)
	}
	switch {
	case !sourceProbePresent && !targetProbePresent:
		plan.Actions.AccountProbe = "absent"
	case !sourceProbePresent && targetProbePresent:
		return Plan{}, errors.New("account probe credential slot state is inconsistent: target exists while source is absent")
	case sourceProbePresent && targetProbePresent:
		if !probeCredentialsEqual(sourceProbe, targetProbe) {
			return Plan{}, errors.New("account probe credential slots differ")
		}
		plan.Actions.AccountProbe = "reuse-equal-target-and-retain-source"
	default:
		plan.Actions.AccountProbe = "copy-and-retain-source"
		plan.probeCopy = probeCopy{value: sourceProbe, copy: true}
	}
	return plan, nil
}

func applyCredentialCopies(deps Dependencies, plan Plan) error {
	if plan.tokenCopy.copy {
		if err := deps.Secrets.Set(plan.NewID, plan.tokenCopy.value); err != nil {
			return fmt.Errorf("Copy API token credential slot: %w", err)
		}
		copied, err := deps.Secrets.Get(plan.NewID)
		if err != nil {
			return fmt.Errorf("verify target API token slot: %w", err)
		}
		if !secretValuesEqual(plan.tokenCopy.value, copied) {
			return errors.New("verify target API token slot: copied value differs")
		}
	}
	if plan.probeCopy.copy {
		if err := deps.Accounts.Set(plan.NewID, plan.probeCopy.value); err != nil {
			return fmt.Errorf("Copy account probe credential slot: %w", err)
		}
		copied, err := deps.Accounts.Get(plan.NewID)
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

func expectedPersistedMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o666
	}
	return 0o600
}

func secretValuesEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func probeCredentialsEqual(left, right account.Credential) bool {
	return secretValuesEqual(left.SystemToken, right.SystemToken) && secretValuesEqual(left.UserID, right.UserID)
}
