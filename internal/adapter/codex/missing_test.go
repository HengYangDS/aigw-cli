package codex

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/terminal"
)

// projectedCodexTarget projects AIGW into a Codex configuration file in its own
// folder and returns the file.
func projectedCodexTarget(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "home", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCodexFixture(t, path, "model = \"gpt-native\"\n")
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
		t.Fatal(err)
	}
	return path
}

// A configuration file that was deleted with its folder, record and all,
// holds nothing to withdraw: dropping it plans an absent target and writes
// nothing, and Codex moves to another file.
func TestDroppingADeletedTargetHasNothingToWithdraw(t *testing.T) {
	gone := projectedCodexTarget(t)
	if err := os.RemoveAll(filepath.Dir(gone)); err != nil {
		t.Fatal(err)
	}
	next := projectedCodexTarget(t)
	plans, err := planCodexReconciliationTransition([]TargetRef{HomeTarget(gone), HomeTarget(next)}, []TargetRef{HomeTarget(next)}, atomicTestRuntime(), atomicTestRuntime(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range plans {
		if plan.Target == gone && (plan.Action != ProjectionActionAbsent || plan.ChangesState()) {
			t.Fatalf("the deleted file's plan = %#v", plan)
		}
	}
	receipt, err := ReconcileConfigs([]TargetRef{HomeTarget(gone)}, nil, atomicTestRuntime(), atomicTestRuntime(), false)
	if err != nil || len(receipt.committed) != 0 {
		t.Fatalf("withdrawing a deleted file = %d writes, %v", len(receipt.committed), err)
	}
	if _, err := os.Stat(filepath.Dir(gone)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("withdrawal created the deleted folder: %v", err)
	}
}

// A deleted configuration file whose record AIGW keeps is refused, because
// the record can restore what AIGW changed; the fix names the record. A file
// that the configuration keeps but that is gone is refused too, because AIGW
// does not create a Codex home, and its fix moves Codex to another file.
func TestAMissingTargetIsRefusedWithItsInspection(t *testing.T) {
	recorded := projectedCodexTarget(t)
	if err := os.Remove(recorded); err != nil {
		t.Fatal(err)
	}
	_, err := ReconcileConfigs([]TargetRef{HomeTarget(recorded)}, nil, atomicTestRuntime(), atomicTestRuntime(), false)
	problem := terminal.ProblemFor(err)
	if want := "Restore the Codex config, or delete config.toml.aigw-state.json in its folder if the config is gone for good, then run the same command again."; problem.Fix != want {
		t.Fatalf("a deleted file with its record = %q, fix %q", problem.Title, problem.Fix)
	}
	if target, ok := errors.AsType[*adapter.TargetError](err); !ok || target.Desired {
		t.Fatalf("the refusal does not name the dropped file: %v", err)
	}
	kept := filepath.Join(t.TempDir(), "gone", "config.toml")
	_, err = ReconcileConfigs([]TargetRef{HomeTarget(kept)}, []TargetRef{HomeTarget(kept)}, atomicTestRuntime(), atomicTestRuntime(), false)
	problem = terminal.ProblemFor(err)
	if problem.Fix != "aigw client edit codex --target <file>" {
		t.Fatalf("a kept file that is gone = %q, fix %q", problem.Title, problem.Fix)
	}
	if target, ok := errors.AsType[*adapter.TargetError](err); !ok || !target.Desired {
		t.Fatalf("the refusal does not name the kept file: %v", err)
	}
}

// Codex's default home is created again by a sync, so only a configuration
// file elsewhere that is gone names the move to another file.
func TestAMissingTargetsRepairFollowsItsHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home", "config.toml")
	deps := adapter.Dependencies{Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(HomeDefault), ConfigPath: home, AutoManaged: true}}}}}
	if got := missingTargetRepair(deps, home); got != "aigw sync" {
		t.Fatalf("the default home = %q", got)
	}
	if got := missingTargetRepair(deps, filepath.Join(t.TempDir(), "config.toml")); got != "aigw client edit codex --target <file>" {
		t.Fatalf("another file = %q", got)
	}
}
