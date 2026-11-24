package synchronization

import (
	"errors"
	"fmt"
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
// and no other change is lost to a choice. Following reads the client's file
// and AIGW's record of it, and no Token: a choice is followed whether or not
// its provider has one.
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
		followed = append(followed, Followed{
			Now: now, Earlier: earlier, Returns: earlier.Restore(),
			ForwardingCleared: was.ForwardingEndpoint != "" && is.ForwardingEndpoint == "",
			RetriesCleared:    was.RetryPolicy != nil && is.RetryPolicy == nil,
		})
		working = candidate
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
// the clients that it reconciles, the choices that the command's
// configuration follows, whether what is stored and the clients' files still
// give that configuration, and whether the stored configuration changes.
type transition struct {
	after     domain.Config
	clients   []string
	reconcile bool
	followed  []Followed
	// left are the clients whose own choice the transition does not save,
	// because a file of theirs is refused: the choice stays in the file, and
	// AIGW's record of it stays behind the choice.
	left    []string
	matched bool
	saves   bool
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
	if settled.after, settled.clients, settled.left, settled.reconcile, err = s.recorded(before, after, stored, followed, reconcile, clientIDs); err != nil {
		return settled, err
	}
	same, err := snapshot.MatchesConfiguration(settled.after, nil)
	settled.saves = !same
	return settled, err
}

// recorded settles what a transition from before to after does with the
// choices that before follows, and returns the configuration that it saves
// and the clients that it reconciles.
//
// A choice is saved only in the transition that also brings AIGW's record of
// the client's file to it: a record that stayed behind would read the
// client's next choice of the earlier model as no choice at all, and the
// next synchronization would write the saved one over it. So a client that
// the command changes, or that can be planned, joins the reconciled clients.
// Its files are written when the provider of the selection that the
// transition saves for it has a Token that AIGW can use, and AIGW's record of
// them alone otherwise, which needs none. A client that the command names
// is not judged here: the plan refuses the whole command when the ordinary
// projection would refuse the client's files, with a Token or without one
// (see admitsWaiting). One that the command does not touch and that cannot
// be planned, because a file of it is refused, is saved as stored, and its
// choice stays in its file: it is returned among left, so that no record is
// brought up over the choice.
func (s Synchronizer) recorded(before, after, stored domain.Config, followed []Followed, reconcile bool, clientIDs []string) (domain.Config, []string, []string, bool, error) {
	if len(followed) == 0 {
		return after, clientIDs, nil, reconcile, nil
	}
	var left []string
	refused := map[string]error{}
	named := slices.Clone(clientIDs)
	if reconcile && len(named) == 0 {
		named = slices.Clone(s.ClientIDs())
	}
	settled := after.Clone()
	for _, choice := range followed {
		client := choice.Now.Client
		switch {
		case slices.Contains(named, client):
		case !reflect.DeepEqual(before.Clients[client], settled.Clients[client]):
			named = append(named, client)
		default:
			refusal := s.planRefusal(before, settled, client)
			if refusal == nil {
				named = append(named, client)
				continue
			}
			settled.Clients[client] = stored.Clients[client]
			left, refused[client] = append(left, client), refusal
		}
	}
	// A client that is left keeps its stored selection. A command that
	// removes the model of that selection is refused in the words of what
	// keeps the client's choice from being recorded, which name the step, and
	// not in the validator's words about a model that the command removed.
	for _, client := range left {
		model := settled.Clients[client].Model
		if _, held := settled.Models[model]; held || model.IsZero() {
			continue
		}
		return domain.Config{}, nil, nil, false, &PreflightError{Err: fmt.Errorf(
			"%s keeps its selection %s, which cannot be removed while the model chosen in %s is not recorded: %w",
			s.registry().Admission().Label(client), model, s.registry().Admission().Label(client), refused[client])}
	}
	// What is saved may hold a client as stored beside the command's own
	// change.
	if err := validateTransition(settled); err != nil {
		return domain.Config{}, nil, nil, false, &PreflightError{Err: err}
	}
	return settled, s.inAdmissionOrder(named), left, len(named) > 0, nil
}

// errNothingPlanned says that a plan of a client holds nothing for it.
var errNothingPlanned = errors.New("nothing can be written or recorded for the client")

// planRefusal returns why the named client cannot be planned for the
// transition, or nil when its files, or AIGW's record of them alone, can be
// planned.
func (s Synchronizer) planRefusal(before, after domain.Config, client string) error {
	projection, err := planned(s, before, after, true, []string{client}, nil)
	if err == nil && !slices.Contains(s.reconciled(projection), client) {
		return errNothingPlanned
	}
	return err
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

// SameSelection reports whether left and right select the same model on the
// same protocol for client, as each configuration resolves it: a selection
// that names no protocol is the one that its model resolves to, so a stored
// selection without a protocol equals a followed one that names it. Two
// configurations in which the client selects nothing select the same.
func SameSelection(left, right domain.Config, client string) bool {
	was, wasErr := left.ResolveRuntime(client, domain.ModelRef{})
	is, isErr := right.ResolveRuntime(client, domain.ModelRef{})
	if wasErr != nil || isErr != nil {
		return wasErr != nil && isErr != nil && left.Clients[client].Model == right.Clients[client].Model
	}
	return was.Model == is.Model && was.Protocol == is.Protocol
}

// keeps returns the choices that before follows and that a transition to
// after keeps as the clients' selections: not the client that the transition
// selects for, and not a client whose selection it replaces or that it
// withdraws. A choice whose earlier model after no longer holds has no step
// that returns to it.
func (s Synchronizer) keeps(followed []Followed, before, after domain.Config) []Followed {
	var kept []Followed
	for _, choice := range followed {
		client := choice.Now.Client
		if client == s.AuthorizedSelectionClient || !after.Clients[client].Enabled || !SameSelection(before, after, client) {
			continue
		}
		if _, held := after.Models[choice.Earlier.Model]; !held {
			choice.Returns = ""
		}
		kept = append(kept, choice)
	}
	return kept
}
