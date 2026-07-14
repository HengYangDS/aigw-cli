//go:build !windows

package cli

func enableWindowsVirtualTerminal() bool { return false }
