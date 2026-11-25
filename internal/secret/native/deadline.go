package native

import (
	"context"
	"fmt"
	"time"
)

// DeadlineError says where a native credential operation stood when its
// deadline ended it. It carries no Token or item data.
type DeadlineError struct {
	// Limit is the deadline, counted from the operation's start.
	Limit time.Duration
	// Created is when the runner saw that the operating system had created
	// the subprocess, and Started when the runner had started it inside the
	// process group that owns it, each counted from the operation's
	// start. Each is negative
	// when the operation returned before the step ended; a time of Limit or
	// later says that the step ended only once the deadline had passed.
	Created, Started time.Duration
}

// Answering reports whether the subprocess was started within the deadline,
// so that only its answer was missing.
func (failure *DeadlineError) Answering() bool {
	return failure.Started >= 0 && failure.Started < failure.Limit
}

// Error names the step that missed the deadline, the time of each step that
// ended within it and, when known, the time at which the missed step ended.
func (failure *DeadlineError) Error() string {
	const subject = "native credential subprocess "
	limit, created, started := rounded(failure.Limit), rounded(failure.Created), rounded(failure.Started)
	switch {
	case failure.Created < 0:
		return subject + "was not created within " + limit
	case failure.Created >= failure.Limit:
		return fmt.Sprintf("%swas not created within %s; its creation ended after %s", subject, limit, created)
	case failure.Started < 0:
		return fmt.Sprintf("%swas created after %s and was not started within %s", subject, created, limit)
	case failure.Started >= failure.Limit:
		return fmt.Sprintf("%swas created after %s and was not started within %s; its start ended after %s", subject, created, limit, started)
	default:
		return fmt.Sprintf("%swas created after %s, was started after %s and did not answer within %s", subject, created, started, limit)
	}
}

// Evidence is Error as a sentence for a problem's evidence line.
func (failure *DeadlineError) Evidence() string { return "The " + failure.Error() + "." }

// Unwrap keeps the failure a deadline for every caller that tests for one.
func (*DeadlineError) Unwrap() error { return context.DeadlineExceeded }

// rounded prints a time cut to the millisecond, so that a time just below the
// limit never prints as the limit, and a shorter one as one millisecond.
func rounded(elapsed time.Duration) string {
	return max(elapsed.Truncate(time.Millisecond), time.Millisecond).String()
}
