package codex

import (
	"errors"
	"os"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
)

// The committee's experiment 1 on GitLab issue 1, kept as a regression: Codex's
// program lies in an editor extension's directory, which no discovery finds,
// so Codex declares that extensions directory. A sync produces Codex's model
// table with the program of the extension's newest version and a check
// passes. Then the extension's version goes. Each later sync keeps the
// declaration, writes none of Codex's files, its model table or AIGW's record,
// and refuses Codex with the fix that names another declaration; a check
// reports the same problem with the same fix, never a sync. Before, the sync
// withdrew the table, marked the record stale and succeeded, and the check
// then sent the operator back to that sync, which changed nothing.
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
	declared := fixture.cfg.Clients[ID]
	_, want := adapter.ClientProgram(fixture.cfg, ID, discovery.Result{}, fixture.deps.Host)
	if want == nil || want.NextAction() != adapter.ProgramRepair((codexAdapter{}).Spec()) {
		t.Fatalf("the declaration resolves to %v, want a stop with the program's repair", want)
	}
	for attempt := range 2 {
		kept, err := fixture.sync(t)
		if binding := kept.Clients[ID]; binding.Program != declared.Program || binding.Extensions != declared.Extensions {
			t.Fatalf("sync %d replaced the declaration with %+v", attempt+2, binding)
		}
		missing, stopped := errors.AsType[*adapter.StopError](err)
		if !stopped || missing.Error() != want.Error() || missing.NextAction() != want.NextAction() {
			t.Fatalf("sync %d = %v, want Codex stopped with %v", attempt+2, err, want)
		}
		if after := fixture.files(t); !slices.EqualFunc(after, before, slices.Equal) {
			t.Fatalf("sync %d changed Codex's files", attempt+2)
		}
		status := fixture.inspect(t, kept)
		if status.Ready || status.Issue != want.Error() || status.RepairAction != want.NextAction() {
			t.Fatalf("check %d = %+v, want the stop and its fix", attempt+2, status)
		}
	}
}
