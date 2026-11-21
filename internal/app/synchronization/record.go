package synchronization

import (
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// recordedAlone returns, in admission order, the clients among clientIDs, or
// among every client when none is named, whose own choice a transition to
// after records without writing their files: the transition starts from a
// configuration that follows the choice, after keeps the client enabled, and
// the client is not among ready, because the provider of the selection that
// after saves for it has no Token that AIGW can use. AIGW's record of such a
// client's file is brought to the model that the file shows, which needs no
// Token, so that the record never stays behind a choice; the file itself
// waits for the Token, as any selection's does.
func recordedAlone(after domain.Config, followed []Followed, clientIDs, ready []string) []string {
	var recorded []string
	for _, choice := range followed {
		client := choice.Now.Client
		named := len(clientIDs) == 0 || slices.Contains(clientIDs, client)
		if named && after.Clients[client].Enabled && !slices.Contains(ready, client) {
			recorded = append(recorded, client)
		}
	}
	return recorded
}

// reconciled returns the clients that projection plans, those whose files it
// writes and those of which it writes AIGW's record alone, in admission order
// when it holds both kinds.
func (s Synchronizer) reconciled(projection Projection) []string {
	if len(projection.Recorded) == 0 {
		return projection.Clients
	}
	return s.inAdmissionOrder(slices.Concat(projection.Clients, projection.Recorded))
}

// projectionDependencies are the client adapters' dependencies for a
// projection of the transition from before to after: the credential reader
// that the clients whose files it writes name, and the clients of which the
// adapters write AIGW's record alone.
func (s Synchronizer) projectionDependencies(projection Projection, before, after domain.Config) (adapter.Dependencies, error) {
	var configurations []domain.Config
	if len(projection.Clients) > 0 {
		configurations = []domain.Config{before, after}
	}
	dependencies, err := s.clientDependencies(projection.Clients, configurations...)
	dependencies.RecordOnly = projection.Recorded
	return dependencies, err
}
