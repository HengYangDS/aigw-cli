package synchronization

import (
	"errors"
	"slices"

	"aigw-cli/internal/adapter"
)

// Stops is what one invocation's last transition, or the preview of one,
// stopped: each client that its plan stopped, with why, in admission order,
// with what the transition wrote besides, so that a report says only what is
// known. A stopped client's files are not written, apart from the withdrawal
// of a file that the transition drops; the rest of the transition is. The
// zero value is ready. A transition without one refuses (see
// errNoStopsRecord): a stop that nothing records would let its command succeed
// falsely.
type Stops struct {
	stopped            []*adapter.StopError
	written, withdrawn []string
	preview            bool
}

// record keeps what projection stops and writes, of a transition or, with
// preview, of the preview of one. A later transition of the same invocation,
// such as the run after its own preview, replaces it.
func (stops *Stops) record(projection Projection, preview bool) {
	stopped := func(client string) bool {
		return slices.ContainsFunc(projection.Stopped, func(stop *adapter.StopError) bool { return stop.Client() == client })
	}
	stops.stopped, stops.preview = projection.Stopped, preview
	stops.written, stops.withdrawn = nil, nil
	for _, plan := range projection.Targets {
		switch {
		case !plan.ChangesState:
		case stopped(plan.Client) && !slices.Contains(stops.withdrawn, plan.Client):
			stops.withdrawn = append(stops.withdrawn, plan.Client)
		case !stopped(plan.Client) && !slices.Contains(stops.written, plan.Client):
			stops.written = append(stops.written, plan.Client)
		}
	}
}

// Stopped returns the clients that the invocation's last transition, or the
// preview of it, stopped.
func (stops *Stops) Stopped() []*adapter.StopError {
	if stops == nil {
		return nil
	}
	return stops.stopped
}

// Written returns the clients whose files that transition wrote, or its
// preview would write; withdrawn the stopped clients of which it withdrew a
// file that it drops; and preview whether it was a preview.
func (stops *Stops) Written() (written, withdrawn []string, preview bool) {
	if stops == nil {
		return nil, nil, false
	}
	return stops.written, stops.withdrawn, stops.preview
}

// errNoStopsRecord refuses a transition, or its preview, that has no record of
// the clients that it stops, before it plans or writes anything.
var errNoStopsRecord = errors.New("synchronization has no record of the clients that it stops (Synchronizer.Stops); the invocation must give one")
