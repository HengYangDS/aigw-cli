package codex

import "aigw-cli/internal/adapter"

// newConflictError refuses settings that AIGW manages in a Codex target and
// that were changed outside it. Synchronization succeeds only after the edit
// is undone.
func newConflictError(message string, cause error) *adapter.ConflictError {
	return adapter.NewConflictError(message, adapter.UndoOutsideEdit("Codex"), cause)
}
