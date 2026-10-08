package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

// PrepareSelection resolves one proposed binding without changing the caller's
// configuration. The same result is used for credential admission and commit.
func (Synchronizer) PrepareSelection(cfg configuration.Config, client, routeID string, protocol configuration.EndpointProtocol, forwarding *string) (configuration.Config, configuration.Runtime, error) {
	if protocol == "" {
		resolved, err := cfg.ResolveRuntime(client, routeID)
		if err != nil {
			return configuration.Config{}, configuration.Runtime{}, err
		}
		protocol = resolved.Protocol
	}
	selected := cfg.Clone()
	selected.SetSelectedRoute(client, routeID, protocol)
	binding := selected.Clients[client]
	if forwarding != nil {
		binding.ForwardingEndpoint = *forwarding
	}
	selected.Clients[client] = binding
	runtime, err := selected.ResolveRuntime(client, "")
	if err != nil {
		return configuration.Config{}, configuration.Runtime{}, err
	}
	return selected, runtime, nil
}

// SelectRoute commits one client's Route selection and projection, optionally storing
// its validated Account Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration and native projection changes separately, not Token acquisition
// or live inference.
func (s Synchronizer) SelectRoute(ctx context.Context, before configuration.Config, client, routeID string, protocol configuration.EndpointProtocol, forwarding *string, token string) (changed, projectionChanged bool, binding configuration.ClientBinding, resultErr error) {
	after, _, err := s.PrepareSelection(before, client, routeID, protocol, forwarding)
	if err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	return s.selectRoute(ctx, before, after, client, routeID, protocol, token)
}

func (s Synchronizer) selectRoute(ctx context.Context, before, after configuration.Config, client, routeID string, protocol configuration.EndpointProtocol, token string) (changed, projectionChanged bool, binding configuration.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	_, exists := after.Routes[routeID]
	if !exists {
		return false, false, configuration.ClientBinding{}, fmt.Errorf("unknown route %q", routeID)
	}
	after, selected, err := s.PrepareSelection(after, client, routeID, protocol, nil)
	if err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	binding = after.Clients[client]
	binding.Enabled = true
	after.Clients[client] = binding
	var tokens map[string]string
	if token != "" {
		if !selected.RequiresAccountToken() {
			return false, false, configuration.ClientBinding{}, fmt.Errorf("route %q uses client-owned authentication; Account Token storage is not part of selection", routeID)
		}
		tokens = map[string]string{selected.AccountID: token}
	}
	rollback, err := secrets.Replace(s.Secrets, tokens)
	if err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rollback())
		}
	}()
	after, _, err = s.DesiredClientConfiguration(after, client)
	if err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	if err := ctx.Err(); err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	binding = after.Clients[client]
	selector := s
	selector.AuthorizeCodexRouteSelection = client == configuration.ClientCodex
	projectionChanged, err = selector.commit(ctx, before, after, "client selection", true, client)
	if err != nil {
		return false, false, configuration.ClientBinding{}, err
	}
	return !reflect.DeepEqual(before, after), projectionChanged, binding, nil
}
