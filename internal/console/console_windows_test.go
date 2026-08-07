//go:build windows

package console

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestVirtualTerminalMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        uint32
		readErr     error
		writeErr    error
		want        bool
		wantWritten bool
	}{
		{name: "read failure", readErr: errors.New("read"), want: false},
		{name: "already enabled", mode: windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING, want: true},
		{name: "enable succeeds", want: true, wantWritten: true},
		{name: "enable fails", writeErr: errors.New("write"), want: false, wantWritten: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			written := false
			got := enableVirtualTerminalMode(
				func(mode *uint32) error {
					*mode = test.mode
					return test.readErr
				},
				func(mode uint32) error {
					written = true
					if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
						t.Fatalf("mode = %#x", mode)
					}
					return test.writeErr
				},
				0,
			)
			if got != test.want || written != test.wantWritten {
				t.Fatalf("got=%t written=%t", got, written)
			}
		})
	}
}

func TestEnableVirtualTerminalMatchesStdoutCapability(t *testing.T) {
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

	enabled := EnableVirtualTerminal()
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
