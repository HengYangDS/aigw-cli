package synchronization

import (
	"context"
	"fmt"
	"slices"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// AdmitSetup requires an installation without Routes before first-time setup.
// Existing installations use configuration import, selection or Token rotation.
func (Synchronizer) AdmitSetup(before domain.Config) error {
	if len(before.Routes) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw route add` to add a model route, or `aigw config import` to merge a reviewed manifest")
	}
	return nil
}

// Setup commits first-time configuration, optional Account Tokens and the selected
// client projections as one operation. Configuration, Account ownership and
// client scope are admitted before writes; downstream failure compensates only
// the operation's unchanged postimages.
// Callers hold the configuration mutation lock, as for other transactions.
func (s Synchronizer) Setup(ctx context.Context, before, after domain.Config, tokens map[string]string, clients ...string) (_ domain.Config, resultErr error) {
	if err := ctx.Err(); err != nil {
		return domain.Config{}, err
	}
	if err := s.AdmitSetup(before); err != nil {
		return domain.Config{}, err
	}
	if err := after.Validate(); err != nil {
		return domain.Config{}, err
	}
	for account := range tokens {
		if _, present := after.Accounts[account]; !present {
			return domain.Config{}, fmt.Errorf("setup Token has no configured Account %q", account)
		}
	}
	for _, client := range clients {
		if !slices.Contains(s.ClientIDs(), client) {
			return domain.Config{}, fmt.Errorf("setup client %q has no admitted operational adapter", client)
		}
		binding := after.Clients[client]
		if binding.Route == "" {
			return domain.Config{}, fmt.Errorf("setup client %q has no selected route", client)
		}
		binding.Enabled = true
		after.Clients[client] = binding
	}
	if err := after.Validate(); err != nil {
		return domain.Config{}, err
	}
	if err := s.validateRetryTransition(before, after, domain.ConnectionChange{}); err != nil {
		return domain.Config{}, err
	}
	rollback, err := secret.Replace(s.Secrets, tokens)
	if err != nil {
		return domain.Config{}, err
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
			return domain.Config{}, err
		}
	}
	if _, err := s.commit(ctx, before, after, "setup", len(clients) > 0, domain.ConnectionChange{}, clients...); err != nil {
		return domain.Config{}, err
	}
	return after, nil
}
