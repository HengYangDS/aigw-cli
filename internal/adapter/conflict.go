package adapter

import (
	"errors"
	"path/filepath"
	"strings"

	"aigw-cli/internal/platform"
)

// ConflictError reports that a client file holds something that AIGW refuses
// to overwrite, such as settings that it manages and that were changed
// outside it. Every surface that reports the refusal gives NextAction, the one
// action that can succeed on the refused state, and a command that refuses
// before aigw.toml exists gives the fix that the refusal carries for that
// state, through BeforeConfiguration.
type ConflictError struct {
	message string
	fix     Fix
	cause   error
}

// Fix is the action that resolves a refusal in each state of a home.
// Configured holds while aigw.toml exists, where `aigw sync` runs.
// Unconfigured holds before it exists, where `aigw sync` refuses and a command
// that needs no configuration, such as the refused `aigw setup`, runs again.
// Each refusal states both, so neither depends on how the other is worded.
type Fix struct {
	Configured, Unconfigured string
}

// ThenRun is the fix whose steps end with `aigw sync` while aigw.toml exists,
// and with the refused command again before it does.
func ThenRun(steps string) Fix {
	return Fix{Configured: steps + ", then run `aigw sync`", Unconfigured: steps + ", then run the same command again."}
}

// Rerun is a fix that ends with the refused command, which holds in either
// state.
func Rerun(fix string) Fix {
	return Fix{Configured: fix, Unconfigured: fix}
}

// NewConflictError returns the refusal message, which fix resolves, and the
// failure behind it, if there is one.
func NewConflictError(message string, fix Fix, cause error) *ConflictError {
	return &ConflictError{message: message, fix: fix, cause: cause}
}

// UndoOutsideEdit is the recovery from an outside edit to the settings that
// AIGW manages for product: synchronization, or the refused command before a
// configuration exists, succeeds only after the edit is undone.
func UndoOutsideEdit(product string) Fix {
	return ThenRun("Undo the outside edit to the AIGW-managed " + product + " settings")
}

// unconfiguredConflictError is a refusal that names the fix that it carries
// for a home without aigw.toml.
type unconfiguredConflictError struct {
	err  error
	next string
}

func (e unconfiguredConflictError) Error() string { return e.err.Error() }

func (e unconfiguredConflictError) Unwrap() error { return e.err }

// NextAction returns the fix that holds without aigw.toml.
func (e unconfiguredConflictError) NextAction() string { return e.next }

// BeforeConfiguration returns err with the fix that its refusal carries for a
// home without aigw.toml, where `aigw sync` refuses: a command that refused a
// client file there, such as `aigw setup`, names that fix. An error that is
// no conflict, or whose fix is the same in both states, is returned as it is.
func BeforeConfiguration(err error) error {
	conflict, refused := errors.AsType[*ConflictError](err)
	if !refused || conflict.fix.Unconfigured == conflict.fix.Configured {
		return err
	}
	return unconfiguredConflictError{err: err, next: conflict.fix.Unconfigured}
}

// MissingWithRecord refuses a client file that does not exist while AIGW's
// record of it remains in its folder. The record can restore what AIGW changed
// there, so the operator decides: restoring the file lets AIGW undo its
// changes, and deleting the record says that the file is gone for good. also
// is a clause for anything else that must go with the record. Files are named
// by base name, so the refusal shows no private folder.
func MissingWithRecord(product, file, record, also string) *ConflictError {
	fileName, recordName := filepath.Base(file), filepath.Base(record)
	return NewConflictError(
		product+" file "+fileName+" does not exist, but AIGW's record of it, "+recordName+", remains in its folder",
		ThenRun("Restore "+fileName+", or delete "+recordName+" in its folder"+also+" if "+fileName+" is gone for good"),
		nil,
	)
}

// UnreadableRecord refuses AIGW's own record of a client file that cannot be
// used as AIGW wrote it. Only the record holds the settings that AIGW replaced
// in that file. Without it, a synchronization refuses the file or takes AIGW's
// own values for the operator's, and a withdrawal leaves them in place, so the
// one step that keeps those settings is the record restored.
func UnreadableRecord(record string, cause error) *ConflictError {
	name := filepath.Base(record)
	return NewConflictError(
		"AIGW's record "+name+" cannot be read: "+strings.ReplaceAll(cause.Error(), "\n", ": "),
		ThenRun("Restore "+name+" in its folder from a copy of it"),
		cause,
	)
}

// Error names what AIGW refused and that it kept the file.
func (e *ConflictError) Error() string { return e.message }

// Unwrap returns the failure behind the conflict, such as the schema decoding
// error of a reconstructed provider fragment. Its offsets are not document
// syntax positions, so the message does not repeat it.
func (e *ConflictError) Unwrap() error { return e.cause }

// ProjectionConflict marks the refusal as a conflict with the client file's
// content rather than a syntax error of AIGW's own documents.
func (e *ConflictError) ProjectionConflict() bool { return true }

// NextAction returns the action that resolves the conflict while aigw.toml
// exists.
func (e *ConflictError) NextAction() string { return e.fix.Configured }

// UnreadableStatus returns the status of a client file at path that the user
// cannot read, and whether err is that failure. Synchronization cannot read it
// either, so the repair is the one fix of its path condition, rendered for a
// check against home: inside home the file is named ~/…, outside it not at all.
func UnreadableStatus(err error, path, home string) (Status, bool) {
	failure := platform.ClassifyPath(path, platform.FileExpected, err).Owned(false)
	if failure.Condition != platform.PathPermission {
		return Status{}, false
	}
	at := platform.PathContext{Home: home, Check: true}
	return Status{
		Issue:        "AIGW cannot read " + at.Name(failure, path) + ": permission denied",
		RepairAction: platform.PathFix(failure, at),
	}, true
}

// ConflictStatus returns the status of a client file on which synchronization
// refuses with err, and whether err is such a conflict. Advising
// synchronization there would send the operator back to the same refusal.
func ConflictStatus(err error) (Status, bool) {
	conflict, refused := errors.AsType[*ConflictError](err)
	if !refused {
		return Status{}, false
	}
	return Status{Issue: conflict.Error(), RepairAction: conflict.NextAction()}, true
}
