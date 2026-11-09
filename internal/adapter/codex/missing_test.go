package codex

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/platform"
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
	if want := "Create " + kept + " and the folder that holds it, then run the same command again."; problem.Fix != want {
		t.Fatalf("a kept file that is gone = %q, fix %q", problem.Title, problem.Fix)
	}
	if target, ok := errors.AsType[*adapter.TargetError](err); !ok || !target.Desired {
		t.Fatalf("the refusal does not name the kept file: %v", err)
	}
}

// missing classifies a configured Codex file that a check found absent.
func missing(path string) *platform.PathConditionError {
	return platform.ClassifyPath(path, platform.FileExpected, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist})
}

// Codex's default home is created again by a sync, which a check names. A
// configuration file elsewhere is not, and AIGW creates no Codex file either,
// so the check names the file to create, with its folder when that is gone,
// and the sync that completes the repair.
func TestAMissingTargetsRepairFollowsItsHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "config.toml")
	deps := adapter.Dependencies{Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(HomeDefault), ConfigPath: home, AutoManaged: true}}}}, Host: discovery.Host{Home: root}}
	if got := missingTargetRepair(deps, missing(home)); got != "aigw sync" {
		t.Fatalf("the default home = %q", got)
	}
	// Outside the home directory a check names no path.
	other := filepath.Join(t.TempDir(), "config.toml")
	if got, want := missingTargetRepair(deps, missing(other)), "Create the file outside the home directory, then run `aigw sync`."; got != want {
		t.Fatalf("a file whose folder exists = %q, want %q", got, want)
	}
	gone := filepath.Join(root, "gone", "config.toml")
	if got, want := missingTargetRepair(deps, missing(gone)), "Create ~/gone/config.toml and the folder that holds it, then run `aigw sync`."; got != want {
		t.Fatalf("a file in a missing folder = %q, want %q", got, want)
	}
}

// A deleted file in Codex's default home is created again by a sync only when
// AIGW keeps no record of it: with one, the sync refuses, and the repair names
// the record.
func TestAMissingTargetsRepairNamesARecordThatRemains(t *testing.T) {
	home := filepath.Join(t.TempDir(), "config.toml")
	writeCodexFixture(t, codexStatePath(home), "{}")
	deps := adapter.Dependencies{Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(HomeDefault), ConfigPath: home, AutoManaged: true}}}}}
	want := "Restore config.toml, or delete config.toml.aigw-state.json in its folder if config.toml is gone for good, then run `aigw sync`"
	if got := missingTargetRepair(deps, missing(home)); got != want {
		t.Fatalf("the default home with its record = %q", got)
	}
}

// A missing target that the configuration keeps is the operator's to create:
// its refusal says why, and wraps the classification from which every surface
// renders the fix, here for a command.
func TestAMissingTargetIsTheOperatorsToCreate(t *testing.T) {
	root := t.TempDir()
	refusal := newMissingTargetError(filepath.Join(root, "gone", "config.toml"))
	failure, classified := errors.AsType[*platform.PathConditionError](refusal)
	if !classified || failure.Condition != platform.PathMissingFolder || !failure.OperatorCreates || !failure.Given ||
		refusal.Error() != "Codex config does not exist, and AIGW does not create a Codex home" {
		t.Fatalf("the refusal = %v, %+v", refusal, failure)
	}
	if got, want := platform.PathFix(failure, platform.PathContext{Home: root}), "Create ~/gone/config.toml and the folder that holds it, then run the same command again."; got != want {
		t.Fatalf("the fix = %q, want %q", got, want)
	}
}

// A folder that is a regular file, or a link that leads nowhere, is named to
// move or remove before a folder to create, which no create through it can
// satisfy, and the check then names what the operator creates and the sync.
func TestAMissingTargetsRepairNamesTheFolderItCannotCreate(t *testing.T) {
	root := t.TempDir()
	deps := adapter.Dependencies{Host: discovery.Host{Home: root}}
	file := filepath.Join(root, "work")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := missingTargetRepair(deps, missing(filepath.Join(file, "config.toml"))), "Move the file ~/work, then create ~/work/config.toml and the folder that holds it, then run `aigw sync`."; got != want {
		t.Fatalf("a file where the folder belongs = %q, want %q", got, want)
	}
	link := filepath.Join(root, "dlink")
	if err := os.Symlink(filepath.Join(root, "nowhere"), link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	got := missingTargetRepair(deps, missing(filepath.Join(link, "config.toml")))
	if want := "Remove the link ~/dlink, then create ~/dlink/config.toml and the folder that holds it, then run `aigw sync`."; got != want {
		t.Fatalf("a link where the folder belongs = %q, want %q", got, want)
	}
}
