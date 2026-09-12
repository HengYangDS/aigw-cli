package synchronization

import (
	"context"
	"fmt"
	"slices"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

// AdmitSetup requires an installation without Profiles before first-time setup.
// Existing installations use configuration import, selection or Token rotation.
func (Synchronizer) AdmitSetup(before configuration.Config) error {
	if len(before.Profiles) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw profile add` to add a model profile, or `aigw config import` to merge a reviewed manifest")
	}
	return nil
}

// Setup commits first-time configuration, optional Account Tokens and the selected
// client projections as one operation. Configuration, Account ownership and
// client scope are admitted before writes; downstream failure compensates only
// the operation's unchanged postimages.
// Callers hold the configuration mutation lock, as for other transactions.
func (s Synchronizer) Setup(ctx context.Context, before, after configuration.Config, tokens map[string]string, clients ...string) (_ configuration.Config, resultErr error) {
	if err := ctx.Err(); err != nil {
		return configuration.Config{}, err
	}
	if err := s.AdmitSetup(before); err != nil {
		return configuration.Config{}, err
	}
	if err := after.Validate(); err != nil {
		return configuration.Config{}, err
	}
	for account := range tokens {
		if _, present := after.Accounts[account]; !present {
			return configuration.Config{}, fmt.Errorf("setup Token has no configured Account %q", account)
		}
	}
	for _, client := range clients {
		if !slices.Contains(s.ClientIDs(), client) {
			return configuration.Config{}, fmt.Errorf("setup client %q has no admitted operational adapter", client)
		}
	}
	rollback, err := secrets.Replace(s.Secrets, tokens)
	if err != nil {
		return configuration.Config{}, err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		if rollbackErr := rollback(); rollbackErr != nil {
			resultErr = fmt.Errorf("setup failed: %w; %w", resultErr, rollbackErr)
			return
		}
		resultErr = fmt.Errorf("setup failed; credentials were rolled back: %w", resultErr)
	}()
	if len(clients) > 0 {
		after, _, err = s.DesiredClientConfiguration(after, clients...)
		if err != nil {
			return configuration.Config{}, err
		}
	}
	if err := s.Commit(ctx, before, after, "setup"); err != nil {
		return configuration.Config{}, err
	}
	return after, nil
}
