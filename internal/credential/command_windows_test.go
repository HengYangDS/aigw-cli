//go:build windows

package credential

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsCommandParserPreservesNativeShortAncestorSpelling(t *testing.T) {
	root := t.TempDir()
	ancestor := filepath.Join(root, "native reader ancestor")
	if err := os.Mkdir(ancestor, 0o700); err != nil {
		t.Fatal(err)
	}
	input, err := windows.UTF16PtrFromString(`\\?\` + ancestor)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	length, err := windows.GetShortPathName(input, &buffer[0], 32768)
	if err != nil || length == 0 || length >= 32768 {
		t.Fatalf("native fixture cannot observe its short ancestor: %v", err)
	}
	short := strings.TrimPrefix(windows.UTF16ToString(buffer[:length]), `\\?\`)
	path := filepath.Join(short, "credential", "aigw")
	command, err := Command(path, "claude", "projection", "windows")
	if err != nil {
		t.Fatal(err)
	}
	observed, err := ExecutableFromCommand(command, "claude", "projection", "windows")
	if err != nil || observed != path {
		t.Fatalf("parser changed native ancestor spelling: observed=%q want=%q error=%v", observed, path, err)
	}
	rendered, err := Command(observed, "claude", "projection", "windows")
	if err != nil || rendered != command {
		t.Fatalf("captured credential command changed after parsing: %v", err)
	}
}
