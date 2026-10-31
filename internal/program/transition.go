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

// Transition is the outcome of a program update or rollback that completed.
type Transition struct {
	// State is TransitionUpdated, TransitionCurrent or TransitionRestored.
	State string
	// Version is the active program's version afterward, without a leading v.
	Version string
	// Summary says what happened, for people.
	Summary string
}
