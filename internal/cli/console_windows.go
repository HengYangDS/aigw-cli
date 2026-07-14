//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

func enableWindowsVirtualTerminal() bool {
	var mode uint32
	handle := windows.Handle(os.Stdout.Fd())
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
