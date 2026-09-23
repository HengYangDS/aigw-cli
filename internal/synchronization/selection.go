package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

// SelectProfile commits one client's Profile selection and projection, optionally storing
// its validated Account Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration change, not Token acquisition or live inference.
func (s Synchronizer) SelectProfile(ctx context.Context, before configuration.Config, client, profileID, token string) (changed bool, binding configuration.ClientBinding, resultErr error) {
	return s.selectProfile(ctx, before, before.Clone(), client, profileID, token)
}

func (s Synchronizer) selectProfile(ctx context.Context, before, after configuration.Config, client, profileID, token string) (changed bool, binding configuration.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, configuration.ClientBinding{}, err
	}
	_, exists := after.Routes[profileID]
	if !exists {
		return false, configuration.ClientBinding{}, fmt.Errorf("unknown profile %q", profileID)
	}
	after.SetSelectedRoute(client, profileID)
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
			return false, configuration.ClientBinding{}, fmt.Errorf("profile %q uses client-owned authentication; Account Token storage is not part of selection", profileID)
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
