package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

// PrepareSelection resolves one proposed binding without credentials, discovery,
// network requests or writes. An omitted destination retains the binding; an
// explicitly empty destination restores its Account endpoint.
func (s Synchronizer) PrepareSelection(before configuration.Config, client, routeID string, destination ...string) (configuration.Config, configuration.Runtime, error) {
	if len(destination) > 1 {
		return configuration.Config{}, configuration.Runtime{}, errors.New("selection requires at most one forwarding destination")
	}
	if len(destination) != 0 && client != configuration.ClientCodex {
		return configuration.Config{}, configuration.Runtime{}, errors.New("forwarding selection requires the Codex client")
	}
	if _, exists := before.Routes[routeID]; !exists {
		return configuration.Config{}, configuration.Runtime{}, fmt.Errorf("unknown route %q", routeID)
	}
	after := before.Clone()
	if after.SelectedRoute(client) != routeID {
		after.SetSelectedRoute(client, routeID)
	}
	if len(destination) == 1 {
		if err := after.SetForwardingEndpoint(client, destination[0]); err != nil {
			return configuration.Config{}, configuration.Runtime{}, err
		}
	}
	binding := after.Clients[client]
	binding.Enabled = true
	after.Clients[client] = binding
	selected, err := after.ResolveRuntime(client, "")
	return after, selected, err
}

// SelectRoute commits one client's Route selection and projection, optionally storing
// its validated Account Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration change, not Token acquisition or live inference.
func (s Synchronizer) SelectRoute(ctx context.Context, before configuration.Config, client, routeID, token string, destination ...string) (changed bool, binding configuration.ClientBinding, resultErr error) {
	return s.selectRoute(ctx, before, before.Clone(), client, routeID, token, destination...)
}

func (s Synchronizer) selectRoute(ctx context.Context, before, after configuration.Config, client, routeID, token string, destination ...string) (changed bool, binding configuration.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, configuration.ClientBinding{}, err
	}
	after, selected, err := s.PrepareSelection(after, client, routeID, destination...)
	if err != nil {
		return false, configuration.ClientBinding{}, err
	}
	var tokens map[string]string
	if token != "" {
		if !selected.RequiresAccountToken() {
			return false, configuration.ClientBinding{}, fmt.Errorf("route %q uses client-owned authentication; Account Token storage is not part of selection", routeID)
		}
		tokens = map[string]string{selected.AccountID: token}
	}
	rollback, err := secrets.Replace(s.Secrets, tokens)
	if err != nil {
		return false, configuration.ClientBinding{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rollback())
		}
	}()
	after, _, err = s.DesiredClientConfiguration(after, client)
	if err != nil {
		return false, configuration.ClientBinding{}, err
	}
	if err := ctx.Err(); err != nil {
		return false, configuration.ClientBinding{}, err
	}
	binding = after.Clients[client]
	selector := s
	selector.AuthorizeCodexRouteSelection = client == configuration.ClientCodex
	if reflect.DeepEqual(before, after) {
		return false, binding, selector.ReconcileClient(ctx, after, client)
	}
	if err := selector.commit(ctx, before, after, "client selection", true, client); err != nil {
		return false, configuration.ClientBinding{}, err
	}
	return true, binding, nil
}
