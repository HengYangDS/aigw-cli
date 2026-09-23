package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

// SelectRoute commits one client's Route selection and projection, optionally storing
// its validated Account Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration change, not Token acquisition or live inference.
func (s Synchronizer) SelectRoute(ctx context.Context, before configuration.Config, client, routeID, token string) (changed bool, binding configuration.ClientBinding, resultErr error) {
	return s.selectRoute(ctx, before, before.Clone(), client, routeID, token)
}

func (s Synchronizer) selectRoute(ctx context.Context, before, after configuration.Config, client, routeID, token string) (changed bool, binding configuration.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, configuration.ClientBinding{}, err
	}
	_, exists := after.Routes[routeID]
	if !exists {
		return false, configuration.ClientBinding{}, fmt.Errorf("unknown route %q", routeID)
	}
	after.SetSelectedRoute(client, routeID)
	binding = after.Clients[client]
	binding.Enabled = true
	after.Clients[client] = binding
	selected, err := after.ResolveRuntime(client, "")
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
	if reflect.DeepEqual(before, after) {
		return false, binding, s.ReconcileClient(ctx, after, client)
	}
	if err := s.commit(ctx, before, after, "client selection", true, client); err != nil {
		return false, configuration.ClientBinding{}, err
	}
	return true, binding, nil
}
