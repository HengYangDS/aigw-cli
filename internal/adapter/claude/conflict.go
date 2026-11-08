package claude

import "aigw-cli/internal/adapter"

// outsideEdit refuses settings that AIGW manages and that were changed
// outside it. Undoing the edit is the only way out: AIGW neither overwrites it
// nor adopts it.
func outsideEdit(message string) error {
	return adapter.NewConflictError(message, adapter.UndoOutsideEdit("Claude Code"), nil)
}

// foreignCredential refuses settings that hold a credential before AIGW owns
// them.
func foreignCredential() error {
	return adapter.NewConflictError(
		"Claude credential ownership conflict: remove the plaintext credential or foreign apiKeyHelper before enabling AIGW",
		"Remove the plaintext credential or foreign apiKeyHelper from Claude Code's settings, then run the same command again.",
		nil,
	)
}

// unreadableSettings refuses a settings file that does not parse, because AIGW
// could not preserve what the file holds.
func unreadableSettings(err error) error {
	return adapter.NewConflictError(err.Error(), "Correct the JSON in Claude Code's settings file, then run the same command again.", err)
}
