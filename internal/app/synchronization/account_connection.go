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
func (s Synchronizer) ConnectAccount(ctx context.Context, before domain.Config, name, client string, account domain.Account, route domain.Route, acquireToken func() (string, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := before.Routes[name]; exists {
		return fmt.Errorf("Route %q already exists; creation does not replace Routes", name)
	}
	if _, exists := before.Accounts[name]; exists {
		return fmt.Errorf("Account %q already exists; add a Route to that Account instead", name)
	}
	after := before.Clone()
	account.ID = name
	route.Account = name
	if route.UpstreamModel == "" {
		route.UpstreamModel = route.Model
	}
	after.Accounts[name] = account
	if _, exists := after.Models[route.Model]; !exists {
		after.Models[route.Model] = domain.Model{Label: route.Model}
	}
	after.Routes[name] = route
	after, _, err := s.PrepareSelection(after, client, name, "", domain.ConnectionChange{})
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
