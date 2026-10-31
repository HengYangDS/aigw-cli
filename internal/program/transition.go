package program

// The states of a completed program transition.
const (
	// TransitionUpdated means that a newer program became active.
	TransitionUpdated = "updated"
	// TransitionCurrent means that the selected program was already active.
	TransitionCurrent = "current"
	// TransitionRestored means that the retained previous program became
	// active.
	TransitionRestored = "restored"
)

// Transition is the outcome of a program update or rollback, or what a
// preview of an update found.
type Transition struct {
	// State is TransitionUpdated, TransitionCurrent or TransitionRestored, or
	// TransitionAvailable for a preview.
	State string
	// Version is the active program's version afterward, or the version that
	// a previewed update would install, without a leading v.
	Version string
	// Summary says what happened, or what a preview found, for people.
	Summary string
}
