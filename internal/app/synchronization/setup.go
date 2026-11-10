package synchronization

import (
	"context"
	"fmt"
	"slices"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// Setup commits a setup's configuration, optional provider Tokens and client
// files as one operation: the files of the named clients, which it activates,
// and of every other client whose files the change affects. Configuration,
// provider ownership and client scope are admitted before writes; downstream
// failure compensates only the operation's unchanged postimages. It runs on
// an empty or a configured home alike.
// Callers hold the configuration mutation lock, as for other transactions.
func (s Synchronizer) Setup(ctx context.Context, before, after domain.Config, tokens map[string]string, clients ...string) (_ domain.Config, resultErr error) {
	if err := ctx.Err(); err != nil {
		return domain.Config{}, err
	}
	after, err := s.admitSetupTransition(after, tokens, clients...)
	if err != nil {
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
	reconciled := s.setupClients(before, after, clients)
	if _, err := s.commit(ctx, before, after, "setup", len(reconciled) > 0, reconciled...); err != nil {
		return domain.Config{}, err
	}
	return after, nil
}

// setupClients returns, in admission order, the clients whose files a setup
// reconciles: the ones that it activates, and every other client whose files
// change, such as one that lists a model that a team file relabels.
func (s Synchronizer) setupClients(before, after domain.Config, activated []string) []string {
	changed := s.registry().ChangedClients(before, after)
	clients := make([]string, 0, len(changed)+len(activated))
	for _, id := range s.ClientIDs() {
		if slices.Contains(activated, id) || slices.Contains(changed, id) {
			clients = append(clients, id)
		}
	}
	return clients
}

// admitSetupTransition admits a setup before any write: a valid
// configuration, a configured provider for each Token, and an admitted
// adapter with a selected model for each client, which it enables in the
// returned configuration.
func (s Synchronizer) admitSetupTransition(after domain.Config, tokens map[string]string, clients ...string) (domain.Config, error) {
	if err := after.Validate(); err != nil {
		return domain.Config{}, err
	}
	for account := range tokens {
		if _, present := after.Providers[account]; !present {
			return domain.Config{}, fmt.Errorf("setup Token has no configured Account %q", account)
		}
	}
	for _, client := range clients {
		if !slices.Contains(s.ClientIDs(), client) {
			return domain.Config{}, fmt.Errorf("setup client %q has no admitted operational adapter", client)
		}
		binding := after.Clients[client]
		if binding.Model == "" {
			return domain.Config{}, fmt.Errorf("setup client %q has no selected route", client)
		}
		binding.Enabled = true
		after.Clients[client] = binding
	}
	if err := after.Validate(); err != nil {
		return domain.Config{}, err
	}
	return after, nil
}
