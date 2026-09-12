//go:build !windows

package console

// EnableVirtualTerminal makes no mode change on Unix and reports false.
func EnableVirtualTerminal() bool { return false }
