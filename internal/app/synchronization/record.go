package synchronization

import (
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// recordsAlone names, in admission order, the clients of which a transition from
// before to after writes AIGW's record alone. AIGW's record of a client's file
// follows the selection that the file shows at every commit: a record that
// stayed behind reads the client's next pick of the recorded model as no
// choice at all, and the next synchronization writes the saved model over it.
// So every enabled client that keeps such a record is considered at every
// transition, whatever the command names and whether or not it reconciles:
//
//   - a client among written has its files written, and its record with them;
//   - a client among left keeps a choice that the transition did not save,
//     because a file of it is refused; its record must stay behind that choice,
//     or the choice would be lost;
//   - any other client has its record brought to the selection that before,
//     the configuration that the command worked from, names for it, when its
//     files show that selection and the ordinary projection of before would be
//     admitted. That needs no Token and writes no client file. It covers a
//     choice whose provider has no Token, a selection that waits for its Token
//     while the client picks another model, and a file that shows the saved
//     selection beside a record of an earlier one.
//
// A client whose file is missing, unreadable or edited outside is left exactly
// as it is: the command that writes its files refuses it under its own name.
// So is a client whose dependencies cannot be resolved, such as a credential
// reader that is not there: a command that names nothing of the client is not
// refused for it, and the command that writes its files reports the failure.
func (s Synchronizer) recordsAlone(before, after domain.Config, written, left []string) []string {
	var recorded []string
	for _, client := range s.ClientIDs() {
		if !before.Clients[client].Enabled || !after.Clients[client].Enabled ||
			slices.Contains(written, client) || slices.Contains(left, client) {
			continue
		}
		// The same dependencies as a projection of before would get, so that
		// the record is brought up exactly when that projection is admitted.
		dependencies, err := s.clientDependencies([]string{client}, before)
		if err != nil {
			// Left as it is; see the last paragraph above.
			continue
		}
		dependencies.AuthorizedSelectionClient = ""
		// A record needs no Token, so deciding it asks the credential store
		// nothing: which other providers are connected does not change
		// whether the client's files are admitted.
		dependencies.Secrets = nil
		if s.registry().RecordPending(dependencies, before, client) {
			recorded = append(recorded, client)
		}
	}
	return recorded
}

// admitsWaiting refuses a transition that names a client whose files the
// ordinary projection would refuse, although the client is not written now
// because its provider has no Token that AIGW can use. A client that waits
// for its Token is planned all the same, with the dependencies that its
// projection would get and without asking the credential store, so the same
// files are refused with a Token and without one: a missing file, a missing
// record, an outside edit, files that select different models. Without this
// a command that names the client would save its change, and a choice that
// the configuration follows with it, over files that nothing can record. A
// waiting client that the plan stops is returned with its stop.
func (s Synchronizer) admitsWaiting(before, after domain.Config, clientIDs, ready []string) ([]*adapter.StopError, error) {
	named := clientIDs
	if len(named) == 0 {
		named = s.ClientIDs()
	}
	var stopped []*adapter.StopError
	for _, client := range named {
		if slices.Contains(ready, client) || (!before.Clients[client].Enabled && !after.Clients[client].Enabled) {
			continue
		}
		dependencies, err := s.clientDependencies([]string{client}, before, after)
		if err != nil {
			return nil, err
		}
		dependencies.Secrets = nil
		_, stops, err := s.registry().PlanStopping(dependencies, before, after, client)
		if err != nil {
			return nil, err
		}
		stopped = append(stopped, stops...)
	}
	return stopped, nil
}

// withdrawnClients names, in admission order, the enabled clients among named
// (or among every client when none is named) that a transition from before to
// after leaves without a Token that AIGW can use, and whose files it drops. A
// file that a client no longer has cannot keep AIGW's settings, and its
// withdrawal needs no Token; but a client that is written writes every file
// that it keeps and restores the ones it drops, so only a client that is not
// written is named here. A client that loses no file is not named: its files
// wait for its Token.
func (s Synchronizer) withdrawnClients(before, after domain.Config, clientIDs, ready []string) []string {
	named := clientIDs
	if len(named) == 0 {
		named = s.ClientIDs()
	}
	var withdrawn []string
	for _, client := range named {
		if slices.Contains(ready, client) || !after.Clients[client].Enabled {
			continue
		}
		if len(platform.DroppedFiles(before.Clients[client].Targets, after.Clients[client].Targets)) == 0 {
			continue
		}
		withdrawn = append(withdrawn, client)
	}
	return s.inAdmissionOrder(withdrawn)
}

// restoredClients returns, in admission order, the clients whose selection a
// restore of restored decides: those whose projection changes from before,
// the configuration that the command worked from, and those whose stored
// binding the restore replaces although the client's own file already chose
// what it restores. Without the second kind a rollback to the model that the
// client chose would reconcile nothing and leave the replaced selection
// stored.
func (s Synchronizer) restoredClients(before, restored domain.Config) []string {
	named := s.registry().ChangedClients(before, restored)
	if stored, read := s.stored(before); read {
		named = slices.Concat(named, s.registry().ChangedClients(stored, restored))
	}
	return s.inAdmissionOrder(named)
}

// reconciled returns the clients that projection plans, those whose files it
// writes, those whose files it withdraws alone, and those of which it writes
// AIGW's record alone, in admission order when it holds more than one kind.
func (s Synchronizer) reconciled(projection Projection) []string {
	if len(projection.Recorded) == 0 && len(projection.Withdrawn) == 0 {
		return projection.Clients
	}
	return s.inAdmissionOrder(slices.Concat(projection.Clients, projection.Withdrawn, projection.Recorded))
}

// projectionDependencies are the client adapters' dependencies for a
// projection of the transition from before to after: the credential reader
// that the clients whose files it writes name, the clients whose files it
// withdraws alone, and the clients of which the adapters write AIGW's record
// alone.
func (s Synchronizer) projectionDependencies(projection Projection, before, after domain.Config) (adapter.Dependencies, error) {
	var configurations []domain.Config
	if len(projection.Clients) > 0 {
		configurations = []domain.Config{before, after}
	}
	dependencies, err := s.clientDependencies(projection.Clients, configurations...)
	dependencies.RecordOnly = projection.Recorded
	dependencies.WithdrawOnly = projection.Withdrawn
	return dependencies, err
}
