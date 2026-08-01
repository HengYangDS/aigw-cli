//go:build !windows

package console

func EnableVirtualTerminal() bool { return false }
