package codex

// ConflictError reports that settings AIGW manages in a Codex target were
// changed outside AIGW. AIGW refuses to overwrite such an edit, so every
// surface that reports the refusal gives NextAction, the one action that can
// succeed on the refused state.
type ConflictError struct {
	message string
	cause   error
}

func newConflictError(message string, cause error) *ConflictError {
	return &ConflictError{message: message, cause: cause}
}

// Error names the refused setting and that AIGW kept the edit.
func (e *ConflictError) Error() string { return e.message }

// Unwrap returns the failure behind the conflict, such as the schema decoding
// error of a reconstructed provider fragment. Its offsets are not document
// syntax positions, so the message does not repeat it.
func (e *ConflictError) Unwrap() error { return e.cause }

// ProjectionConflict marks the refusal as a conflict with an outside edit
// rather than a syntax error.
func (e *ConflictError) ProjectionConflict() bool { return true }

// NextAction returns the recovery: synchronization succeeds only after the
// outside edit is undone.
func (e *ConflictError) NextAction() string {
	return "Undo the outside edit to the AIGW-managed Codex settings, then run `aigw sync`"
}
