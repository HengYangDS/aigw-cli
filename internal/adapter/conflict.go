package adapter

import (
	"errors"
	"path/filepath"
)

// ConflictError reports that a client file holds something that AIGW refuses
// to overwrite, such as settings that it manages and that were changed
// outside it. Every surface that reports the refusal gives NextAction, the one
// action that can succeed on the refused state.
type ConflictError struct {
	message string
	next    string
	cause   error
}

// NewConflictError returns the refusal message, which the action next
// resolves, and the failure behind it, if there is one.
func NewConflictError(message, next string, cause error) *ConflictError {
	return &ConflictError{message: message, next: next, cause: cause}
}

// UndoOutsideEdit is the recovery from an outside edit to the settings that
// AIGW manages for product: synchronization succeeds only after the edit is
// undone.
func UndoOutsideEdit(product string) string {
	return "Undo the outside edit to the AIGW-managed " + product + " settings, then run `aigw sync`"
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
		"Restore "+fileName+", or delete "+recordName+" in its folder"+also+" if "+fileName+" is gone for good, then run `aigw sync`",
		nil,
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

// NextAction returns the action that resolves the conflict.
func (e *ConflictError) NextAction() string { return e.next }

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
