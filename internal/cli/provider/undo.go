package provider

import (
	"errors"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"
)

// credentialChange is one credential write that a command made before a later
// step, and how to undo it. undone is the clause that says what the undo did,
// such as "the Token was put back", and kept the one that says what stays
// when it cannot be undone, such as "the Token stays deleted". undo is nil
// when the previous value could not be read first, so nothing can put it
// back.
type credentialChange struct {
	undone string
	kept   string
	undo   func() error
}

// undoOutcome is what undoing changes did: the clauses of the changes undone
// and of those kept, why each kept one could not be undone, and the errors of
// the undos that failed.
type undoOutcome struct {
	undone, kept, reasons []string
	errs                  []error
}

// undoChanges undoes changes, newest first.
func undoChanges(changes []credentialChange) undoOutcome {
	var outcome undoOutcome
	for _, change := range slices.Backward(changes) {
		if change.undo == nil {
			outcome.kept = append(outcome.kept, change.kept)
			outcome.reasons = append(outcome.reasons, sentence(change.kept)+" because it could not be read before the change.")
			continue
		}
		if err := change.undo(); err != nil {
			outcome.kept = append(outcome.kept, change.kept)
			outcome.reasons = append(outcome.reasons, sentence(change.kept)+": "+terminal.ProblemFor(err).Title+".")
			outcome.errs = append(outcome.errs, err)
			continue
		}
		outcome.undone = append(outcome.undone, change.undone)
	}
	return outcome
}

// saveFailure reports a configuration save that failed with err after the
// command changed credentials, which it undoes. When every change is undone,
// the failure is err's own problem, whose impact says that nothing was saved
// and what was put back. Otherwise it names what stays changed, after title,
// and fix is the command that converges from that state.
func saveFailure(err error, changes []credentialChange, title, fix string) error {
	outcome := undoChanges(changes)
	if len(outcome.kept) == 0 {
		return invocation.WithImpact(err, "Nothing was saved"+clauses("; ", outcome.undone)+".")
	}
	return keptFailure("Saving the configuration failed: "+terminal.ProblemFor(err).Title+".", err, outcome, title, fix)
}

// keptFailure is the problem of a write that failed with err, as failed says,
// and whose undo left the kept changes in place: its title is title and what
// stays, its evidence says what failed, what err's own undo did and why each
// undo failed, its impact what was put back, and its fix the command that
// converges. Every undo error stays inspectable with err.
func keptFailure(failed string, err error, outcome undoOutcome, title, fix string) error {
	impact := ""
	if len(outcome.undone) > 0 {
		impact = sentence(strings.Join(outcome.undone, " and ")) + "."
	}
	evidence := strings.Join(slices.DeleteFunc([]string{failed, terminal.ProblemFor(err).Impact, strings.Join(outcome.reasons, " ")},
		func(part string) bool { return part == "" }), " ")
	return terminal.ProblemError(title+", and "+strings.Join(outcome.kept, " and "), evidence, impact, fix, errors.Join(append([]error{err}, outcome.errs...)...))
}

// clauses joins clauses after prefix, or is empty without any.
func clauses(prefix string, parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return prefix + strings.Join(parts, " and ")
}

// sentence capitalizes the first letter of a clause.
func sentence(clause string) string {
	if clause == "" {
		return ""
	}
	return strings.ToUpper(clause[:1]) + clause[1:]
}
