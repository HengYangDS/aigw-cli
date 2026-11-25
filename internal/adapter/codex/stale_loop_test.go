package codex

import (
	"errors"
	"os"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
)

// The committee's experiment 1 on GitLab issue 1, kept as a regression: Codex's
// program lies in an editor extension's directory, which no discovery finds.
// A sync produces Codex's model table with it and a check passes. Then the
// extension's directory goes. Each later sync keeps the configured program,
// writes none of Codex's files, its model table or AIGW's record, and refuses
// Codex with the fix that names another program; a check reports the missing
// program with the same fix, never a sync. Before, the sync withdrew the
// table, marked the record stale and succeeded, and the check then sent the
// operator back to that sync, which changed nothing.
func TestAMissingCodexProgramEndsTheStaleLoop(t *testing.T) {
	fixture := newCodexProgram(t, extensionLayout)
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	if status := fixture.inspect(t, fixture.cfg); !status.Ready {
		t.Fatalf("the first check = %+v, want ready", status)
	}
	if err := os.RemoveAll(fixture.pkg); err != nil {
		t.Fatal(err)
	}
	before := fixture.files(t)
	for attempt := range 2 {
		kept, err := fixture.sync(t)
		if kept.Clients[ID].Executable != fixture.program {
			t.Fatalf("sync %d replaced the configured program with %q", attempt+2, kept.Clients[ID].Executable)
		}
		missing, stopped := errors.AsType[*adapter.StopError](err)
		if !stopped || missing.Error() != "Codex executable is missing" || missing.NextAction() != "aigw client edit codex --executable <path>" {
			t.Fatalf("sync %d = %v, want Codex stopped for its missing program", attempt+2, err)
		}
		if after := fixture.files(t); !slices.EqualFunc(after, before, slices.Equal) {
			t.Fatalf("sync %d changed Codex's files", attempt+2)
		}
		status := fixture.inspect(t, kept)
		if status.Ready || status.Issue != "Codex executable is missing" || status.RepairAction != "aigw client edit codex --executable <path>" {
			t.Fatalf("check %d = %+v, want the missing program and its fix", attempt+2, status)
		}
	}
}
