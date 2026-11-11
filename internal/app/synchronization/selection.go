package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
)

// PrepareSelection returns cfg with client's selection of the model that ref
// names on protocol, or on the protocol that the configuration resolves when it is
// empty, and the runtime that it resolves to, without changing cfg. Setting
// the selection drops forwarding when the upstream endpoint changes and retry
// budgets when the provider changes. The same result is used for credential
// admission and commit.
func (Synchronizer) PrepareSelection(cfg domain.Config, client string, ref domain.ModelRef, protocol domain.EndpointProtocol) (domain.Config, domain.Runtime, error) {
	if protocol == "" {
		resolved, err := cfg.ResolveRuntime(client, ref)
		if err != nil {
			return domain.Config{}, domain.Runtime{}, err
		}
		protocol = resolved.Protocol
	}
	selected := cfg.Clone()
	selected.SetSelectedModel(client, ref, protocol)
	if err := selected.Validate(); err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	runtime, err := selected.ResolveRuntime(client, domain.ModelRef{})
	if err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	return selected, runtime, nil
}

// SelectModel commits after, which PrepareSelection made from before and may
// extend with what the selection adds, and the files of the client and of
// every other client whose files change, optionally storing the client's
// validated provider Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration and client file changes separately, not Token acquisition or
// live inference.
func (s Synchronizer) SelectModel(ctx context.Context, before, after domain.Config, client, token string) (changed, projectionChanged bool, binding domain.ClientBinding, resultErr error) {
	return s.selectModel(ctx, before, after, client, token)
}

func (s Synchronizer) selectModel(ctx context.Context, before, after domain.Config, client, token string) (changed, projectionChanged bool, binding domain.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	selected, err := after.ResolveRuntime(client, domain.ModelRef{})
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	binding = after.Clients[client]
	binding.Enabled = true
	after.Clients[client] = binding
	var tokens map[string]string
	if token != "" {
		if !selected.RequiresProviderToken() {
			return false, false, domain.ClientBinding{}, fmt.Errorf("route %q uses client-owned authentication; Account Token storage is not part of selection", binding.Model)
		}
		tokens = map[string]string{selected.ProviderID: token}
	}
	rollback, err := secret.Replace(s.Secrets, tokens)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, platform.FailedUndo("removing the Token that this command stored for provider "+selected.ProviderID, rollback()))
		}
	}()
	after, _, err = s.DesiredClientConfiguration(after, client)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	if err := ctx.Err(); err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	binding = after.Clients[client]
	selector := s
	selector.AuthorizedSelectionClient = client
	projectionChanged, err = selector.commit(ctx, before, after, "client selection", true, selector.selectionClients(before, after, client)...)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	return !reflect.DeepEqual(before, after), projectionChanged, binding, nil
}

// selectionClients returns the clients whose files a selection for client
// reconciles, in admission order: client itself, and every other client whose
// files change, such as one that lists a model that the selection adds.
func (s Synchronizer) selectionClients(before, after domain.Config, client string) []string {
	changed := s.registry().ChangedClients(before, after)
	clients := make([]string, 0, len(changed)+1)
	for _, id := range s.ClientIDs() {
		if id == client || slices.Contains(changed, id) {
			clients = append(clients, id)
		}
	}
	return clients
}
