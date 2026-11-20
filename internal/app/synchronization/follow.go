package synchronization

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// Followed is a model that a client's own file selects and that the working
// configuration records as that client's selection.
type Followed struct {
	// Client is the client's ID and Product its name in messages.
	Client, Product string
	Model           domain.ModelRef
	// Earlier is the selection that the stored configuration holds.
	Earlier adapter.RecordedSelection
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
		model, earlier, err := s.registry().Follow(dependencies, &candidate, client)
		if err != nil || model.IsZero() || validateTransition(candidate) != nil {
			continue
		}
		working = candidate
		followed = append(followed, Followed{Client: client, Product: earlier.Product, Model: model, Earlier: earlier})
	}
	return working, followed
}

// working is Working as the view that binds a transaction's preimage.
func (s Synchronizer) working(stored domain.Config) domain.Config {
	working, _ := s.Working(stored)
	return working
}
