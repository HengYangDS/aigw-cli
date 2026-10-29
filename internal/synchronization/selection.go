package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secrets"
)

// PrepareSelection resolves one proposed binding without changing the caller's
// configuration. The same result is used for credential admission and commit.
func (Synchronizer) PrepareSelection(cfg domain.Config, client, routeID string, protocol domain.EndpointProtocol, connection domain.ConnectionChange) (domain.Config, domain.Runtime, error) {
	if connection.NativeRetries && connection.RetryPolicy != nil {
		return domain.Config{}, domain.Runtime{}, fmt.Errorf("native retries and explicit retry budgets are mutually exclusive")
	}
	if protocol == "" {
		resolved, err := cfg.ResolveRuntime(client, routeID)
		if err != nil {
			return domain.Config{}, domain.Runtime{}, err
		}
		protocol = resolved.Protocol
	}
	selected := cfg.Clone()
	previous, previousErr := cfg.ResolveRuntime(client, "")
	selected.SetSelectedRoute(client, routeID, protocol)
	if connection.ForwardingEndpoint != nil {
		if err := selected.SetForwardingEndpoint(client, *connection.ForwardingEndpoint); err != nil {
			return domain.Config{}, domain.Runtime{}, err
		}
	}
	binding := selected.Clients[client]
	runtime, err := selected.ResolveRuntime(client, "")
	if err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	sameConnection := previousErr == nil && previous.ConnectionFingerprint() == runtime.ConnectionFingerprint()
	if err := validateRetryConnectionChange(cfg, selected, client, connection); err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	if connection.RetryPolicy != nil {
		policy := binding.RetryPolicy.Clone()
		if policy == nil || !sameConnection {
			policy = &domain.RetryPolicy{}
		}
		declared := connection.RetryPolicy.Clone()
		if declared.RequestMaxRetries != nil {
			policy.RequestMaxRetries = declared.RequestMaxRetries
		}
		if declared.StreamMaxRetries != nil {
			policy.StreamMaxRetries = declared.StreamMaxRetries
		}
		binding.RetryPolicy = policy
	}
	if connection.NativeRetries {
		binding.RetryPolicy = nil
	}
	selected.Clients[client] = binding
	if err := selected.Validate(); err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	runtime, err = selected.ResolveRuntime(client, "")
	if err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	return selected, runtime, nil
}

func validateRetryConnectionChange(before, after domain.Config, client string, connection domain.ConnectionChange) error {
	if before.Clients[client].RetryPolicy == nil || after.Clients[client].Route == "" || connection.NativeRetries || connection.RetryPolicy != nil {
		return nil
	}
	previous, previousErr := before.ResolveRuntime(client, "")
	current, currentErr := after.ResolveRuntime(client, "")
	if previousErr == nil && currentErr == nil && previous.ConnectionFingerprint() == current.ConnectionFingerprint() {
		return nil
	}
	return fmt.Errorf("client %q connection changed with a selected retry policy; declare budgets with `aigw use --for %s`, or select --native-retries on the current connection first", client, client)
}

func (s Synchronizer) validateRetryTransition(before, after domain.Config, connection domain.ConnectionChange) error {
	for _, client := range s.ClientIDs() {
		intent := domain.ConnectionChange{}
		if client == s.AuthorizedSelectionClient {
			intent = connection
		}
		if err := validateRetryConnectionChange(before, after, client, intent); err != nil {
			return err
		}
	}
	return nil
}

// SelectRoute commits one client's Route selection and projection, optionally storing
// its validated Account Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration and native projection changes separately, not Token acquisition
// or live inference.
func (s Synchronizer) SelectRoute(ctx context.Context, before domain.Config, client, routeID string, protocol domain.EndpointProtocol, connection domain.ConnectionChange, token string) (changed, projectionChanged bool, binding domain.ClientBinding, resultErr error) {
	after, _, err := s.PrepareSelection(before, client, routeID, protocol, connection)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	return s.selectRoute(ctx, before, after, client, connection, token)
}

func (s Synchronizer) selectRoute(ctx context.Context, before, after domain.Config, client string, connection domain.ConnectionChange, token string) (changed, projectionChanged bool, binding domain.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	selected, err := after.ResolveRuntime(client, "")
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	binding = after.Clients[client]
	binding.Enabled = true
	after.Clients[client] = binding
	var tokens map[string]string
	if token != "" {
		if !selected.RequiresAccountToken() {
			return false, false, domain.ClientBinding{}, fmt.Errorf("route %q uses client-owned authentication; Account Token storage is not part of selection", binding.Route)
		}
		tokens = map[string]string{selected.AccountID: token}
	}
	rollback, err := secrets.Replace(s.Secrets, tokens)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rollback())
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
	projectionChanged, err = selector.commit(ctx, before, after, "client selection", true, connection, client)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	return !reflect.DeepEqual(before, after), projectionChanged, binding, nil
}
