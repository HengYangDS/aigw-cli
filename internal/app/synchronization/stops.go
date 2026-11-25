package synchronization

import (
	"errors"

	"aigw-cli/internal/adapter"
)

// Stops is what one invocation's last transition, or the preview of one,
// stopped: each client that its plan stopped, with why, in admission order.
// A stopped client's files are not written; the rest of the transition is.
// The zero value is ready. A transition without one refuses (see
// errNoStopsRecord): a stop that nothing records would let its command succeed
// falsely.
type Stops struct {
	stopped []*adapter.StopError
}

// record keeps the stops of a transition or its preview. A later transition
// of the same invocation, such as the run after its own preview, replaces
// them.
func (stops *Stops) record(stopped []*adapter.StopError) {
	stops.stopped = stopped
}

// Stopped returns the clients that the invocation's last transition, or the
// preview of it, stopped.
func (stops *Stops) Stopped() []*adapter.StopError {
	if stops == nil {
		return nil
	}
	return stops.stopped
}

// errNoStopsRecord refuses a transition, or its preview, that has no record of
// the clients that it stops, before it plans or writes anything.
var errNoStopsRecord = errors.New("synchronization has no record of the clients that it stops (Synchronizer.Stops); the invocation must give one")
