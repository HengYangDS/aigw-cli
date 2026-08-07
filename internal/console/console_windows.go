//go:build windows

package console

import (
	"os"

	"golang.org/x/sys/windows"
)

func EnableVirtualTerminal() bool {
	var mode uint32
	handle := windows.Handle(os.Stdout.Fd())
	return enableVirtualTerminalMode(
		func(mode *uint32) error { return windows.GetConsoleMode(handle, mode) },
		func(mode uint32) error { return windows.SetConsoleMode(handle, mode) },
		mode,
	)
}

func enableVirtualTerminalMode(read func(*uint32) error, write func(uint32) error, mode uint32) bool {
	if err := read(&mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return write(mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
