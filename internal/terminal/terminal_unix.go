//go:build !windows

package terminal

// EnableVirtualTerminal makes no mode change on Unix and reports false.
func EnableVirtualTerminal() bool { return false }
