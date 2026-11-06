package synchronization

import (
	"context"
	"fmt"
	"strings"

	"aigw-cli/internal/domain"
)

// ConnectAccount adds one Account and its first Model and Route, selects the Route for
// that client, and projects only that client. Token acquisition follows identity
// and configuration admission. Existing identities are never implicitly replaced.
// Configuration, credential and projection failures retain their shared recovery
// semantics; the caller owns no compensation after this operation returns.
func (s Synchronizer) ConnectAccount(ctx context.Context, before domain.Config, name, client string, account domain.Provider, route domain.Model, acquireToken func() (string, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	after, err := s.PlanConnection(before, name, client, account, route)
	if err != nil {
		return err
	}
	token, err := acquireToken()
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("Account connection requires a non-empty Token")
	}
	_, _, _, err = s.selectRoute(ctx, before, after, client, domain.ConnectionChange{}, token)
	return err
}

// PlanConnection returns the configuration that ConnectAccount would write:
// the Account, its first Model and Route, and the client's selection of that
// Route. It reads no Token and writes nothing.
func (s Synchronizer) PlanConnection(before domain.Config, name, client string, account domain.Provider, route domain.Model) (domain.Config, error) {
	if _, exists := before.Models[name]; exists {
		return domain.Config{}, fmt.Errorf("Route %q already exists; creation does not replace Routes", name)
	}
	if _, exists := before.Providers[name]; exists {
		return domain.Config{}, fmt.Errorf("Account %q already exists; add a Route to that Account instead", name)
	}
	after := before.Clone()
	account.ID = name
	route.Provider = name
	if route.UpstreamModel == "" {
		route.UpstreamModel = route.Family
	}
	after.Providers[name] = account
	if _, exists := after.Families[route.Family]; !exists {
		after.Families[route.Family] = domain.Family{Label: route.Family}
	}
	after.Models[name] = route
	after, _, err := s.PrepareSelection(after, client, name, "", domain.ConnectionChange{})
	return after, err
}
