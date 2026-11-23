package cli_test

import (
	"errors"
	"os/exec"
	"testing"
)

// A command that probes a client starts its executable. The fixture that
// stands for a client's executable is a program that the operating system
// starts and that exits without output, so a probe fails as it does for a
// client that is not there. A file that is not a program makes Windows take
// its invalid-image path, which on hosted Windows Server 2025 ended the
// whole test binary with a fail fast (0xc0000409 in KERNEL32) when several
// tests started such files at once.
func TestAnExecutableFixtureStartsAndSaysNothing(t *testing.T) {
	command := exec.Command(executableFixture(t, "codex"), "--version")
	output, err := command.Output()
	if exit, exited := errors.AsType[*exec.ExitError](err); !exited || exit.ExitCode() == 0 {
		t.Fatalf("the fixture did not start and fail as a program: %v", err)
	}
	if len(output) != 0 {
		t.Fatalf("the fixture wrote %q", output)
	}
}
