//go:build windows

package cli

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestEnableWindowsVirtualTerminalMatchesStdoutCapability(t *testing.T) {
	handle := windows.Handle(os.Stdout.Fd())
	var before uint32
	beforeErr := windows.GetConsoleMode(handle, &before)
	if beforeErr == nil {
		t.Cleanup(func() {
			if err := windows.SetConsoleMode(handle, before); err != nil {
				t.Errorf("restore stdout console mode: %v", err)
			}
		})
	}

	enabled := enableWindowsVirtualTerminal()
	if beforeErr != nil {
		if enabled {
			t.Fatal("virtual terminal enabled for non-console stdout")
		}
	} else {
		if !enabled {
			t.Fatal("virtual terminal was not enabled for console stdout")
		}
		var after uint32
		if err := windows.GetConsoleMode(handle, &after); err != nil {
			t.Fatalf("read enabled stdout console mode: %v", err)
		}
		if after&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
			t.Fatalf("stdout console mode = %#x, want virtual terminal processing", after)
		}
	}
}
