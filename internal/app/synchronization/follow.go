package synchronization

import (
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

// Follows is what one invocation made of the clients' own choices, for the
// invocation's report: the choices that its working configuration follows
// and, once a transition or its preview ran, those that the transition
// keeps. The zero value is ready; a nil Follows records nothing.
type Follows struct {
	// Reports marks an invocation that saves nothing and reports the
	// working configuration, so that it names every choice that it follows.
	Reports bool
	// Reported says that the invocation's output has named the choices.
	Reported bool

	observed, kept []Followed
	decided        bool
}

// Observe records the choices that the invocation's working configuration
// follows.
func (follows *Follows) Observe(followed []Followed) {
	if follows != nil {
		follows.observed = followed
	}
}

func (follows *Follows) decide(kept []Followed) {
	if follows != nil {
		follows.kept, follows.decided = kept, true
	}
}

// Selections are the followed choices that the invocation reports: those
// that its transition, or the preview of it, keeps as the clients'
// selections; without a transition, every choice of a reporting invocation;
// and none for an invocation that neither saved, previewed nor reports.
func (follows *Follows) Selections() []Followed {
	switch {
	case follows == nil:
		return nil
	case follows.decided:
		return follows.kept
	case follows.Reports:
		return follows.observed
	}
	return nil
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
// the stored configuration in snapshot, and returns the choices that it
// follows. matched is false when what is stored, or a client's file, changed
// since the command read it.
func (s Synchronizer) preimage(snapshot store.Snapshot, before domain.Config) (followed []Followed, matched bool, err error) {
	matched, err = snapshot.MatchesConfiguration(before, func(stored domain.Config) domain.Config {
		working, choices := s.Working(stored)
		followed = choices
		return working
	})
	return followed, matched, err
}

// reconciled returns the clients whose files a transition reconciles, and
// whether it reconciles any: the named ones, or every client when none is
// named and reconcile is set, and with them every client whose own choice
// the transition records. A choice is never recorded without AIGW's record
// of the client's file: a record that stayed behind would read the client's
// next choice of the earlier model as no choice at all.
func (s Synchronizer) reconciled(followed []Followed, reconcile bool, clientIDs []string) ([]string, bool) {
	if len(followed) == 0 || reconcile && len(clientIDs) == 0 {
		return clientIDs, reconcile
	}
	clients := make([]string, 0, len(clientIDs)+len(followed))
	for _, id := range s.ClientIDs() {
		if slices.Contains(clientIDs, id) || slices.ContainsFunc(followed, func(choice Followed) bool { return choice.Now.Client == id }) {
			clients = append(clients, id)
		}
	}
	return clients, true
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
