package synchronization

import (
	"errors"
	"reflect"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// Projection is what a commit would project: the clients whose files it would
// reconcile, for which it prepares the credential reader, the clients of
// which it would write AIGW's record alone, the clients whose files it would
// withdraw alone, the files that it would change, and whether it would change
// the stored configuration.
type Projection struct {
	Clients []string
	// Recorded are the clients of which the commit writes AIGW's record
	// alone, because the record is behind the selection that their files
	// show and the commit does not write those files.
	Recorded []string
	// Withdrawn are the enabled clients whose provider has no Token that AIGW
	// can use, and whose files the transition drops: the dropped files are
	// restored, which needs no Token, and the files that the client keeps wait
	// for it.
	Withdrawn     []string
	Targets       []adapter.ProjectionPlan
	Configuration bool
}

// Preview reports what Commit would project for one configuration transition.
// It validates the transition as Commit does and plans the clients whose
// configuration changes, but it writes nothing.
func (s Synchronizer) Preview(before, after domain.Config) (Projection, error) {
	if err := validateTransition(after); err != nil {
		return Projection{}, err
	}
	clients := s.registry().ChangedClients(before, after)
	return project(s, before, after, len(clients) > 0, clients...)
}

// PreviewProjection reports what CommitProjection would project for the named
// clients, or for every client when none is named, and writes nothing.
func (s Synchronizer) PreviewProjection(before, after domain.Config, clientIDs ...string) (Projection, error) {
	if err := validateTransition(after); err != nil {
		return Projection{}, err
	}
	return project(s, before, after, true, clientIDs...)
}

// PreviewRestore reports what CommitRestore would project for a restore of
// restored from before, the configuration that the command worked from.
func (s Synchronizer) PreviewRestore(before, restored domain.Config) (Projection, error) {
	clients := s.restoredClients(before, restored)
	if len(clients) == 0 {
		return s.Preview(before, restored)
	}
	return s.PreviewProjection(before, restored, clients...)
}

// PreviewSetup reports what Setup would write for the named clients, as if the
// named providers' Tokens were stored: the configuration after the same
// admission, desired client configuration and validation, and the files of
// the named clients and of every other client whose files change. It reads,
// checks and stores no Token of those providers and writes nothing.
func (s Synchronizer) PreviewSetup(before, after domain.Config, providers []string, clients ...string) (domain.Config, Projection, error) {
	planner := s
	planner.Secrets = presumedTokens{providers: providers, store: s.Secrets}
	after, err := planner.admitSetupTransition(after, nil, clients...)
	if err != nil {
		return after, Projection{}, err
	}
	if len(clients) > 0 {
		if after, _, err = planner.DesiredClientConfiguration(after, clients...); err != nil {
			return domain.Config{}, Projection{}, err
		}
	}
	if err := validateTransition(after); err != nil {
		return domain.Config{}, Projection{}, err
	}
	reconciled := planner.setupClients(before, after, clients)
	projection, err := project(planner, before, after, len(reconciled) > 0, reconciled...)
	return after, projection, err
}

// PreviewSelection reports what SelectModel would write for after, which
// PrepareSelection made from before: the configuration with the client
// enabled and its desired configuration, and the files of the client and of
// every other client whose files change. The presumed providers' Tokens
// count as stored, as for a Token that the selection would store. It reads,
// checks and stores no Token and writes nothing.
func (s Synchronizer) PreviewSelection(before, after domain.Config, client string, presumed ...string) (domain.Config, Projection, error) {
	after = after.Clone()
	binding := after.Clients[client]
	binding.Enabled = selectionEnables(before.Clients[client], binding)
	after.Clients[client] = binding
	selector := s
	selector.Secrets = presumedTokens{providers: presumed, store: s.Secrets}
	selector.AuthorizedSelectionClient = client
	after, _, err := selector.DesiredClientConfiguration(after, client)
	if err != nil {
		return domain.Config{}, Projection{}, err
	}
	if err := validateTransition(after); err != nil {
		return domain.Config{}, Projection{}, err
	}
	projection, err := project(selector, before, after, true, selector.selectionClients(before, after, client)...)
	return after, projection, err
}

// project plans what a commit would reconcile: the credential-ready clients
// among the named ones, or among every client when none is named and
// reconcile is set, with every client whose own choice the commit would
// record, and their projections.
func project(planner Synchronizer, before, after domain.Config, reconcile bool, clientIDs ...string) (Projection, error) {
	// Without a store, as in a plan that nothing saves, the transition is
	// what the command gave.
	settled := transition{after: after, clients: clientIDs, reconcile: reconcile, saves: !reflect.DeepEqual(before, after)}
	if planner.Config != nil {
		snapshot, err := planner.Config.CaptureSnapshot()
		if err == nil {
			settled, err = planner.settle(snapshot, before, after, reconcile, clientIDs)
		}
		if err != nil {
			return Projection{}, err
		}
	}
	projection, err := planned(planner, before, settled.after, settled.reconcile, settled.clients, settled.left)
	if err != nil {
		return Projection{}, err
	}
	// Only a preview that the run would not refuse says what it records.
	planner.Follows.settle(planner.keeps(settled.followed, before, settled.after))
	projection.Configuration = settled.saves
	return projection, nil
}

// planned plans what a transition from before to after writes: the
// projections of the credential-ready clients among clientIDs, or among every
// client when none is named, when reconcile is set; and, at every transition,
// AIGW's record alone of each other client whose record is behind the
// selection that its files show (see recordsAlone). left names the clients whose
// choice the transition did not save. It writes nothing, so a plan that is
// refused is a PreflightError, for a command and for its preview alike.
func planned(planner Synchronizer, before, after domain.Config, reconcile bool, clientIDs, left []string) (Projection, error) {
	var projection Projection
	if reconcile {
		ready, err := planner.credentialReadyClients(after, clientIDs...)
		if err != nil {
			return Projection{}, &PreflightError{Err: err}
		}
		projection.Clients = ready
		projection.Withdrawn = planner.withdrawnClients(before, after, clientIDs, ready)
		if err := planner.admitsWaiting(before, after, clientIDs, ready); err != nil {
			return Projection{}, &PreflightError{Err: err}
		}
	}
	projection.Recorded = planner.recordsAlone(before, after, projection.Clients, left)
	if len(projection.Clients)+len(projection.Recorded)+len(projection.Withdrawn) == 0 {
		return Projection{}, nil
	}
	dependencies, err := planner.projectionDependencies(projection, before, after)
	if err != nil {
		return Projection{}, err
	}
	projection.Targets, err = planner.registry().Plan(dependencies, before, after, planner.reconciled(projection)...)
	if err != nil {
		return Projection{}, &PreflightError{Err: err}
	}
	return projection, nil
}

// errPresumedToken refuses a Token read or write during a preview.
var errPresumedToken = errors.New("a preview reads and writes no Token")

// presumedTokens answers that the named Accounts' Tokens are stored without
// consulting the credential store, so that a preview plans as if new Accounts
// were connected. Other Accounts' presence comes from the store; no Token is
// read or written.
type presumedTokens struct {
	providers []string
	store     secret.Store
}

func (presumed presumedTokens) Exists(account string) (bool, error) {
	if slices.Contains(presumed.providers, account) {
		return true, nil
	}
	if presumed.store == nil {
		return false, nil
	}
	return presumed.store.Exists(account)
}

// ObserveToken answers that a presumed Account's Token is one that AIGW can
// use, without consulting the credential store, and observes any other
// Account's there: the readiness owner and the client adapters then plan the
// clients that the previewed command would write.
func (presumed presumedTokens) ObserveToken(account string) secret.TokenObservation {
	if slices.Contains(presumed.providers, account) {
		return secret.TokenObservation{Present: true}
	}
	return secret.ObserveToken(presumed.store, account)
}

func (presumedTokens) Get(string) (string, error) { return "", errPresumedToken }

func (presumedTokens) Set(string, string) error { return errPresumedToken }

func (presumedTokens) Delete(string) error { return errPresumedToken }
