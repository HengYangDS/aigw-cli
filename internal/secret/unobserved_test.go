package secret

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// A failure to observe a slot is told from a closed set. A backend's own
// diagnostic stays out of it.
func TestObservationFailureNamesItsCauseAndNoPrivateDetail(t *testing.T) {
	cause, fix := ObservationFailure(fmt.Errorf("inspect: %w", unsafeTokenFileError{name: "orphan", message: "must be owner-only"}))
	if cause != "must be owner-only" || !strings.HasPrefix(fix, "Make orphan in AIGW's Token directory "+tokenFileRequirement) {
		t.Fatalf("an unsafe file = %q, %q", cause, fix)
	}
	cause, fix = ObservationFailure(unsafeTokenDirectoryError{message: "must be a directory"})
	if cause != "must be a directory" || !strings.HasPrefix(fix, "Make AIGW's Token directory "+tokenDirectoryRequirement) {
		t.Fatalf("an unsafe directory = %q, %q", cause, fix)
	}
	cause, fix = ObservationFailure(errors.New("open /private/home/tokens: permission denied"))
	if cause != "the credential backend is unavailable" || fix != BackendUnavailableFix {
		t.Fatalf("another failure = %q, %q", cause, fix)
	}
}
