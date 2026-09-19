//go:build !windows

package presentation

// EnableVirtualTerminal makes no mode change on Unix and reports false.
func EnableVirtualTerminal() bool { return false }
