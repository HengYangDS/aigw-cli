package native

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/platform/process"
)

func TestADeadlineNamesTheStepThatMissedItAndEachTime(t *testing.T) {
	const limit = 5 * time.Second
	for _, test := range []struct {
		name             string
		created, started time.Duration
		answering        bool
		want             string
	}{
		{"not created", -1, -1, false, "native credential subprocess was not created within 5s"},
		{"created late", 6200 * time.Millisecond, 6300 * time.Millisecond, false, "native credential subprocess was not created within 5s; its creation ended after 6.2s"},
		{"created at the limit", limit, -1, false, "native credential subprocess was not created within 5s; its creation ended after 5s"},
		{"not started", 1200 * time.Millisecond, -1, false, "native credential subprocess was created after 1.2s and was not started within 5s"},
		{"started late", 1200 * time.Millisecond, 5400 * time.Millisecond, false, "native credential subprocess was created after 1.2s and was not started within 5s; its start ended after 5.4s"},
		{"started at the limit", 1200 * time.Millisecond, limit, false, "native credential subprocess was created after 1.2s and was not started within 5s; its start ended after 5s"},
		{"no answer", 200 * time.Millisecond, 4800 * time.Millisecond, true, "native credential subprocess was created after 200ms, was started after 4.8s and did not answer within 5s"},
		{"no answer, at once", 0, 0, true, "native credential subprocess was created after 1ms, was started after 1ms and did not answer within 5s"},
		{"just below the limit", 4999600 * time.Microsecond, 4999900 * time.Microsecond, true, "native credential subprocess was created after 4.999s, was started after 4.999s and did not answer within 5s"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stood := &DeadlineError{Limit: limit, Created: test.created, Started: test.started}
			if got := stood.Error(); got != test.want {
				t.Fatalf("Error() = %q, want %q", got, test.want)
			}
			if got := stood.Evidence(); got != "The "+test.want+"." {
				t.Fatalf("Evidence() = %q", got)
			}
			if stood.Answering() != test.answering || !errors.Is(stood, context.DeadlineExceeded) {
				t.Fatalf("Answering() = %t, want %t; a deadline: %t", stood.Answering(), test.answering, errors.Is(stood, context.DeadlineExceeded))
			}
		})
	}
}

// stalledRunner reports the steps it is told to, at once or only when the
// deadline has ended, and never answers.
type stalledRunner struct {
	created, started bool
	late             bool
}

func (r stalledRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if r.late {
		<-ctx.Done()
	}
	if r.created {
		plan.OnCreated()
	}
	if r.started {
		if err := plan.OnStart(nil); err != nil {
			return nil, err
		}
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

// The operation's own clock is not forced here, so the assertions hold however
// long the machine stalls: which steps were reported, their order, and a step
// reported after the deadline counting as missed.
func TestADeadlineCarriesTheStepsThatTheRunnerReported(t *testing.T) {
	for _, test := range []struct {
		name   string
		runner stalledRunner
	}{
		{"nothing reported", stalledRunner{}},
		{"created", stalledRunner{created: true}},
		{"created and started", stalledRunner{created: true, started: true}},
		{"both after the deadline", stalledRunner{created: true, started: true, late: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			value, err := execute(ctx, test.runner, process.Plan{Executable: "/owned/aigw", Args: []string{readCommand, "AIGW_TOKEN", "team"}})
			stood, typed := errors.AsType[*DeadlineError](err)
			if value != "" || !typed || !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "AIGW_TOKEN") {
				t.Fatalf("deadline lost its type or carries item data: value=%q error=%v", value, err)
			}
			if (stood.Created >= 0) != test.runner.created || (stood.Started >= 0) != test.runner.started {
				t.Fatalf("steps = created %v, started %v; reported: %+v", stood.Created, stood.Started, test.runner)
			}
			if test.runner.started && stood.Started < stood.Created {
				t.Fatalf("started %v before created %v", stood.Started, stood.Created)
			}
			// The limit is the caller's earlier deadline, counted from the
			// operation's start, however late that start was.
			if stood.Limit == 0 || stood.Limit > 20*time.Millisecond {
				t.Fatalf("limit = %v, want the caller's earlier deadline of at most 20ms", stood.Limit)
			}
			if test.runner.late && (stood.Created < stood.Limit || stood.Answering() || !strings.Contains(err.Error(), "was not created within")) {
				t.Fatalf("steps reported after the deadline count as reached: %v (%+v)", err, stood)
			}
		})
	}
}

func TestACancelledCallerIsNotADeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := execute(ctx, stalledRunner{created: true, started: true}, process.Plan{Executable: "/owned/aigw", Args: []string{readCommand, "AIGW_TOKEN", "team"}})
	if _, typed := errors.AsType[*DeadlineError](err); typed || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v, want the caller's cancellation", err)
	}
}
