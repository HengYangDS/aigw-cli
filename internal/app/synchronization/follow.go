package synchronization

import (
	"aigw-cli/internal/domain"
)

// Followed is a model that a client's own file selects and that a transition
// records as that client's selection.
type Followed struct {
	// Client is the client's ID and Product its name in messages.
	Client, Product string
	Model           domain.ModelRef
	// Returns is the command that selects the client's earlier model again.
	Returns string
}

// Follow returns cfg with the model that each enabled client chose in its own
// file recorded as that client's selection, and what it followed, in
// admission order. Synchronization starts from it. A choice that AIGW does not
// follow is refused with the command that restores the recorded selection.
func (s Synchronizer) Follow(cfg domain.Config) (domain.Config, []Followed, error) {
	return s.follow(cfg, cfg, s.ClientIDs()...)
}

// follow returns after with the own choice of each named client recorded as
// its selection, and what it followed. A transition that changes a client's
// selection itself, or that selects for it explicitly, is not followed away
// from: its own selection wins.
func (s Synchronizer) follow(before, after domain.Config, clientIDs ...string) (domain.Config, []Followed, error) {
	var followed []Followed
	dependencies, err := s.clientDependencies(nil)
	if err != nil {
		return domain.Config{}, nil, err
	}
	result := after.Clone()
	for _, client := range clientIDs {
		was, now := before.Clients[client], after.Clients[client]
		if client == s.AuthorizedSelectionClient || !now.Enabled || was.Model != now.Model || was.Protocol != now.Protocol {
			continue
		}
		model, earlier, err := s.registry().Follow(dependencies, &result, client)
		if err != nil {
			return domain.Config{}, nil, err
		}
		if !model.IsZero() {
			followed = append(followed, Followed{Client: client, Product: earlier.Product, Model: model, Returns: earlier.Restore()})
		}
	}
	return result, followed, nil
}

// followedTransition is what a transition to after writes for the named
// clients, or for every client when none is named: after with the own choice
// of each credential-ready client followed, the clients that are ready on
// that configuration, and what was followed. The transaction and the preview
// share it, so a preview shows what the run does.
func (s Synchronizer) followedTransition(before, after domain.Config, clientIDs ...string) (domain.Config, []string, []Followed, error) {
	ready, err := s.credentialReadyClients(after, clientIDs...)
	if err != nil || len(ready) == 0 {
		return after, ready, nil, err
	}
	after, followed, err := s.follow(before, after, ready...)
	if err != nil || len(followed) == 0 {
		return after, ready, followed, err
	}
	// A followed model may belong to another provider, whose Token decides
	// anew whether the client's files can be written.
	ready, err = s.credentialReadyClients(after, clientIDs...)
	return after, ready, followed, err
}
