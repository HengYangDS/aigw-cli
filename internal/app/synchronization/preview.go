package synchronization

import (
	"errors"
	"fmt"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// Projection is what a commit would project: the clients whose projections it
// would reconcile, for which it prepares the credential reader, and the client
// files that it would change.
type Projection struct {
	Clients []string
	Targets []adapter.ProjectionPlan
}

// Preview reports what Commit would project for one configuration transition.
// It validates the transition as Commit does and plans the clients whose
// configuration changes, but it writes nothing.
func (s Synchronizer) Preview(before, after domain.Config) (Projection, error) {
	if err := s.validateTransition(before, after, domain.ConnectionChange{}); err != nil {
		return Projection{}, err
	}
	clients := s.registry().ChangedClients(before, after)
	if len(clients) == 0 {
		return Projection{}, nil
	}
	return project(s, before, after, clients...)
}

// PreviewProjection reports what CommitProjection would project for the named
// clients, or for every client when none is named, and writes nothing.
func (s Synchronizer) PreviewProjection(before, after domain.Config, clientIDs ...string) (Projection, error) {
	if err := s.validateTransition(before, after, domain.ConnectionChange{}); err != nil {
		return Projection{}, err
	}
	return project(s, before, after, clientIDs...)
}

// PreviewSetup reports what Setup would write for the named clients, as if the
// named Accounts' Tokens were stored: the configuration after the same
// admission, desired client configuration and validation, and its projection.
// It reads, checks and stores no Token of those Accounts and writes nothing.
func (s Synchronizer) PreviewSetup(before, after domain.Config, accounts []string, clients ...string) (domain.Config, Projection, error) {
	planner := s
	planner.Secrets = presumedTokens{accounts: accounts, store: s.Secrets}
	after, err := planner.admitSetupTransition(before, after, nil, clients...)
	if err != nil || len(clients) == 0 {
		return after, Projection{}, err
	}
	if after, _, err = planner.DesiredClientConfiguration(after, clients...); err != nil {
		return domain.Config{}, Projection{}, err
	}
	if err := planner.validateTransition(before, after, domain.ConnectionChange{}); err != nil {
		return domain.Config{}, Projection{}, err
	}
	projection, err := project(planner, before, after, clients...)
	return after, projection, err
}

// PreviewConnection reports what ConnectAccount would write, as if the new
// Account's Token were stored: the configuration, with the client's selection
// of the new Route and its desired configuration, and its projection. It
// reads, checks and stores no Token of that Account and writes nothing.
func (s Synchronizer) PreviewConnection(before domain.Config, name, client string, account domain.Account, route domain.Route) (domain.Config, Projection, error) {
	after, err := s.PlanConnection(before, name, client, account, route)
	if err != nil {
		return domain.Config{}, Projection{}, err
	}
	selected, err := after.ResolveRuntime(client, "")
	if err != nil {
		return domain.Config{}, Projection{}, err
	}
	if !selected.RequiresAccountToken() {
		return domain.Config{}, Projection{}, fmt.Errorf("route %q uses client-owned authentication; Account Token storage is not part of selection", name)
	}
	binding := after.Clients[client]
	binding.Enabled = true
	after.Clients[client] = binding
	selector := s
	selector.Secrets = presumedTokens{accounts: []string{name}, store: s.Secrets}
	selector.AuthorizedSelectionClient = client
	if after, _, err = selector.DesiredClientConfiguration(after, client); err != nil {
		return domain.Config{}, Projection{}, err
	}
	if err := s.validateTransition(before, after, domain.ConnectionChange{}); err != nil {
		return domain.Config{}, Projection{}, err
	}
	projection, err := project(selector, before, after, client)
	return after, projection, err
}

// project plans what a commit would reconcile: the credential-ready clients
// among the named ones, or among every client when none is named, and their
// projections.
func project(planner Synchronizer, before, after domain.Config, clientIDs ...string) (Projection, error) {
	clients, err := planner.credentialReadyClients(after, clientIDs...)
	if err != nil || len(clients) == 0 {
		return Projection{}, err
	}
	dependencies, err := planner.clientDependencies(clients, before, after)
	if err != nil {
		return Projection{}, err
	}
	targets, err := planner.registry().Plan(dependencies, before, after, clients...)
	if err != nil {
		return Projection{}, err
	}
	return Projection{Clients: clients, Targets: targets}, nil
}

// errPresumedToken refuses a Token read or write during a preview.
var errPresumedToken = errors.New("a preview reads and writes no Token")

// presumedTokens answers that the named Accounts' Tokens are stored without
// consulting the credential store, so that a preview plans as if new Accounts
// were connected. Other Accounts' presence comes from the store; no Token is
// read or written.
type presumedTokens struct {
	accounts []string
	store    secret.Store
}

func (presumed presumedTokens) Exists(account string) (bool, error) {
	if slices.Contains(presumed.accounts, account) {
		return true, nil
	}
	if presumed.store == nil {
		return false, nil
	}
	return presumed.store.Exists(account)
}

func (presumedTokens) Get(string) (string, error) { return "", errPresumedToken }

func (presumedTokens) Set(string, string) error { return errPresumedToken }

func (presumedTokens) Delete(string) error { return errPresumedToken }
