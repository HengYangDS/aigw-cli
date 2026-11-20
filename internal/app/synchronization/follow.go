package synchronization

import (
	"reflect"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// Followed is a model that a client's own file selects and that the working
// configuration records as that client's selection.
type Followed struct {
	// Now is the followed selection and Earlier the one that the stored
	// configuration holds.
	Now, Earlier adapter.RecordedSelection
	// Returns is the step that selects the earlier model again, or "" when
	// the configuration no longer holds that model.
	Returns string
	// ForwardingCleared and RetriesCleared say that the earlier selection's
	// forwarding or retry budgets ended with its upstream or its provider.
	ForwardingCleared, RetriesCleared bool
	// protocol is the followed binding's protocol, as the configuration
	// records it.
	protocol domain.EndpointProtocol
}

// Follows is what one invocation made of the clients' own choices: the
// choices that its working configuration follows and, once a transition or
// its preview ran, those that the transition records. The zero value is
// ready; a nil Follows records nothing.
type Follows struct {
	observed, kept []Followed
	settled        bool
}

// Observe records the choices that the invocation's working configuration
// follows.
func (follows *Follows) Observe(followed []Followed) {
	if follows != nil {
		follows.observed = followed
	}
}

// settle records the choices that a transition, or the preview of one,
// records as the clients' selections. A later transition of the same
// invocation, such as the run after its own preview, replaces it.
func (follows *Follows) settle(kept []Followed) {
	if follows != nil {
		follows.kept, follows.settled = kept, true
	}
}

// Observed are the choices that the working configuration follows.
func (follows *Follows) Observed() []Followed {
	if follows == nil {
		return nil
	}
	return follows.observed
}

// Recorded are the choices that the invocation's last transition, or the
// preview of it, records, and whether one ran.
func (follows *Follows) Recorded() ([]Followed, bool) {
	if follows == nil {
		return nil, false
	}
	return follows.kept, follows.settled
}

// Working returns the configuration that every command works from: stored
// with the model that each enabled client chose in its own file recorded as
// that client's selection, and what it followed, in admission order. A
// command's own change is made on top of it, so an explicit selection wins
// and no other change is lost to a choice.
//
// A choice that AIGW does not follow, a file that it cannot read, or a
// followed selection that would not validate leaves that client as stored:
// the command that writes the client's files refuses it then, with the step
// that restores the recorded selection, and a command that writes none of
// them is not stopped by another client's file.
func (s Synchronizer) Working(stored domain.Config) (domain.Config, []Followed) {
	dependencies, err := s.clientDependencies(nil)
	if err != nil {
		return stored, nil
	}
	working := stored
	var followed []Followed
	for _, client := range s.ClientIDs() {
		candidate := working.Clone()
		earlier, now, follows, err := s.registry().Follow(dependencies, &candidate, client)
		if err != nil || !follows || validateTransition(candidate) != nil {
			continue
		}
		was, is := working.Clients[client], candidate.Clients[client]
		working = candidate
		followed = append(followed, Followed{
			Now: now, Earlier: earlier, Returns: earlier.Restore(), protocol: is.Protocol,
			ForwardingCleared: was.ForwardingEndpoint != "" && is.ForwardingEndpoint == "",
			RetriesCleared:    was.RetryPolicy != nil && is.RetryPolicy == nil,
		})
	}
	return working, followed
}

// preimage binds before, the working configuration that a command read, to
// the stored configuration in snapshot, and returns that stored
// configuration and the choices that before follows. matched is false when
// what is stored, or a client's file, changed since the command read it.
func (s Synchronizer) preimage(snapshot store.Snapshot, before domain.Config) (stored domain.Config, followed []Followed, matched bool, err error) {
	matched, err = snapshot.MatchesConfiguration(before, func(current domain.Config) domain.Config {
		stored = current
		working, choices := s.Working(current)
		followed = choices
		return working
	})
	return stored, followed, matched, err
}

// transition is one configuration transition as a commit, or the preview of
// one, settles it against what is stored: the configuration that it saves,
// the clients whose files it reconciles, the choices that the command's
// configuration follows, whether what is stored and the clients' files still
// give that configuration, and whether the stored configuration changes.
type transition struct {
	after     domain.Config
	clients   []string
	reconcile bool
	followed  []Followed
	matched   bool
	saves     bool
}

// settle settles the transition from before to after against snapshot. A
// transition whose preimage no longer matches is returned as the command
// gave it: a commit refuses it, and a preview plans it as it stands.
func (s Synchronizer) settle(snapshot store.Snapshot, before, after domain.Config, reconcile bool, clientIDs []string) (transition, error) {
	settled := transition{after: after, clients: clientIDs, reconcile: reconcile, saves: !reflect.DeepEqual(before, after)}
	stored, followed, matched, err := s.preimage(snapshot, before)
	if err != nil || !matched {
		return settled, err
	}
	settled.followed, settled.matched = followed, true
	if settled.after, settled.clients, settled.reconcile, err = s.recorded(before, after, stored, followed, reconcile, clientIDs); err != nil {
		return settled, err
	}
	same, err := snapshot.MatchesConfiguration(settled.after, nil)
	settled.saves = !same
	return settled, err
}

// recorded settles what a transition from before to after does with the
// choices that before follows, and returns the configuration that it saves
// and the clients whose files it reconciles.
//
// A choice is saved only in the transition that also writes AIGW's record of
// the client's file: a record that stayed behind would read the client's
// next choice of the earlier model as no choice at all, and the next
// synchronization would write the saved one over it. So a client whose
// choice the command does not touch joins the reconciled clients when its
// files can be planned. A followed client whose files the transition cannot
// write with the choice, because it waits for the chosen model's Token or a
// file of it is refused, is left out of the transition altogether: its files
// are not written, what is saved holds its selection as stored, and the
// choice stays in its file, which the next command follows again. A command
// that changes such a client's other settings saves that change alone, beside
// the stored selection; one that selects a model for it saves that model.
func (s Synchronizer) recorded(before, after, stored domain.Config, followed []Followed, reconcile bool, clientIDs []string) (domain.Config, []string, bool, error) {
	if len(followed) == 0 {
		return after, clientIDs, reconcile, nil
	}
	named := slices.Clone(clientIDs)
	if reconcile && len(named) == 0 {
		named = slices.Clone(s.ClientIDs())
	}
	settled := after.Clone()
	for _, choice := range followed {
		client := choice.Now.Client
		written := slices.Contains(named, client)
		was, is := before.Clients[client], settled.Clients[client]
		untouched := reflect.DeepEqual(was, is)
		switch {
		case !written && untouched && s.plans(before, settled, client):
			named = append(named, client)
			continue
		case written:
			ready, err := s.credentialReadyClients(settled, client)
			if err != nil {
				return domain.Config{}, nil, false, &PreflightError{Err: err}
			}
			if len(ready) > 0 || was.Model != is.Model || was.Protocol != is.Protocol {
				continue
			}
		}
		binding, err := withStoredSelection(settled, was, is, stored.Clients[client], client)
		if err != nil {
			return domain.Config{}, nil, false, &PreflightError{Err: err}
		}
		settled.Clients[client] = binding
		named = slices.DeleteFunc(named, func(id string) bool { return id == client })
	}
	// What is saved may hold a client as stored beside the command's own
	// change, such as the removal of the model that the stored selection names.
	if err := validateTransition(settled); err != nil {
		return domain.Config{}, nil, false, &PreflightError{Err: err}
	}
	return settled, s.inAdmissionOrder(named), len(named) > 0, nil
}

// withStoredSelection returns the client's binding is, which a command made
// from the binding was that follows a choice, with the selection that kept,
// the stored binding, holds: the stored model and protocol, and the retry
// budgets and forwarding that the choice ended. What the command itself
// changed stays; a forwarding that it set reaches the stored selection's
// upstream.
func withStoredSelection(cfg domain.Config, was, is, kept domain.ClientBinding, client string) (domain.ClientBinding, error) {
	binding := is
	binding.Model, binding.Protocol = kept.Model, kept.Protocol
	if reflect.DeepEqual(was.RetryPolicy, is.RetryPolicy) {
		binding.RetryPolicy = kept.RetryPolicy
	}
	binding.ForwardingEndpoint = kept.ForwardingEndpoint
	binding.ForwardingUpstreamIdentity, binding.ForwardingProtocol = kept.ForwardingUpstreamIdentity, kept.ForwardingProtocol
	if was.ForwardingEndpoint == is.ForwardingEndpoint {
		return binding, nil
	}
	working := cfg.Clone()
	working.Clients[client] = binding
	if err := working.SetForwardingEndpoint(client, is.ForwardingEndpoint); err != nil {
		return domain.ClientBinding{}, err
	}
	return working.Clients[client], nil
}

// plans reports whether the named client's files can be written for the
// transition: it has the Token that it needs, and its plan is not refused.
func (s Synchronizer) plans(before, after domain.Config, client string) bool {
	projection, err := planned(s, before, after, true, []string{client})
	return err == nil && slices.Contains(projection.Clients, client)
}

// inAdmissionOrder returns the named clients in admission order.
func (s Synchronizer) inAdmissionOrder(named []string) []string {
	if len(named) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(named))
	for _, id := range s.ClientIDs() {
		if slices.Contains(named, id) {
			ordered = append(ordered, id)
		}
	}
	return ordered
}

// keeps returns the followed choices that a transition to after keeps as the
// clients' selections: not the client that the transition selects for, and
// not a client whose selection it replaces. A choice whose earlier model
// after no longer holds has no step that returns to it.
func (s Synchronizer) keeps(followed []Followed, after domain.Config) []Followed {
	var kept []Followed
	for _, choice := range followed {
		binding := after.Clients[choice.Now.Client]
		if choice.Now.Client == s.AuthorizedSelectionClient || !binding.Enabled || binding.Model != choice.Now.Model || binding.Protocol != choice.protocol {
			continue
		}
		if _, held := after.Models[choice.Earlier.Model]; !held {
			choice.Returns = ""
		}
		kept = append(kept, choice)
	}
	return kept
}
