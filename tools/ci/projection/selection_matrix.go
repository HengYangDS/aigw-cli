package projection

import (
	"fmt"
	"time"
)

// SelectionMatrixSlowestRun is the slowest measured run of the exhaustive
// selection tests on a supported native platform: macOS, 655 s. The design of
// the Change selection-matrix-native-platforms gives every run; D5 of the
// Change drop-windows-and-intel-macos derives the bounds from it.
const SelectionMatrixSlowestRun = 655 * time.Second

// SelectionMatrixBound leaves room for a host three times slower than that run,
// rounded up to whole five minutes. The quality graph runs the matrix within
// it, so a hang ends with Go's diagnostics, and the CUE graph receives it to
// derive the budget of every job that runs the matrix.
var SelectionMatrixBound = (3*SelectionMatrixSlowestRun + 5*time.Minute - 1).Truncate(5 * time.Minute)

// selectionMatrixValue is the CUE value that hands a bound to the CI graph, in
// whole minutes.
func selectionMatrixValue(bound time.Duration) string {
	return fmt.Sprintf("package ci\nselectionMatrixBoundMinutes: %d\n", bound/time.Minute)
}
