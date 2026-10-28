// Package contract checks the invariants of the client seam against one adapter
// in a private fixture home. Every admitted adapter and the test-only rehearsal
// adapter run it from their tests.
package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

// Fixture is one adapter's projection scenario. Every file that the adapter may
// write lives below Home. Before has no projection for the adapter, After
// enables it, and Runtime is After's selected runtime.
type Fixture struct {
	Adapter      client.Adapter
	Home         string
	Dependencies client.Dependencies
	Before       configuration.Config
	After        configuration.Config
	Runtime      configuration.Runtime
}

// Run checks the seam invariants for the fixtures that newFixture prepares:
// Inspect and Verify touch nothing, plans predict exactly whether Apply changes
// state and name targets that Apply projects, a converged projection plans and
// applies no change, Rollback restores every file's bytes and mode, and
// withdrawal leaves no owned file behind.
func Run(t *testing.T, newFixture func(t *testing.T) Fixture) {
	t.Helper()
	t.Run("inspection and verification touch nothing", func(t *testing.T) {
		fixture := newFixture(t)
		original := capture(t, fixture.Home)
		fixture.Adapter.Inspect(t.Context(), fixture.Dependencies, fixture.After, fixture.Runtime)
		_, _ = fixture.Adapter.Verify(t.Context(), fixture.Dependencies, fixture.After, fixture.Runtime, "")
		requireUnchanged(t, original, capture(t, fixture.Home), "inspection and verification")
	})
	t.Run("plans predict every change and rollback restores it", func(t *testing.T) {
		fixture := newFixture(t)
		original := capture(t, fixture.Home)
		plans, err := fixture.Adapter.Plan(fixture.Dependencies, fixture.Before, fixture.After)
		if err != nil {
			t.Fatalf("Plan() = %v", err)
		}
		requireUnchanged(t, original, capture(t, fixture.Home), "planning")
		receipt, err := fixture.Adapter.Apply(t.Context(), fixture.Dependencies, fixture.Before, fixture.After)
		if err != nil {
			t.Fatalf("Apply() = %v", err)
		}
		projected := capture(t, fixture.Home)
		if changed := changedPaths(original, projected); len(changed) == 0 || receipt == nil || !changesState(plans) {
			t.Fatalf("Apply() of an enabled projection changed %v with receipt %v after plans %#v", changed, receipt, plans)
		}
		for _, plan := range plans {
			if _, err := os.Stat(plan.Target); plan.ChangesState && err != nil {
				t.Errorf("planned target %s was not projected: %v", plan.Target, err)
			}
		}
		converged, err := fixture.Adapter.Plan(fixture.Dependencies, fixture.After, fixture.After)
		if err != nil || changesState(converged) {
			t.Fatalf("a converged projection plans changes: %#v, %v", converged, err)
		}
		if _, err := fixture.Adapter.Apply(t.Context(), fixture.Dependencies, fixture.After, fixture.After); err != nil {
			t.Fatalf("Apply() of a converged projection = %v", err)
		}
		requireUnchanged(t, projected, capture(t, fixture.Home), "applying a converged projection")
		if err := receipt.Rollback(); err != nil {
			t.Fatalf("Rollback() = %v", err)
		}
		requireUnchanged(t, original, capture(t, fixture.Home), "rollback")
	})
	t.Run("withdrawal leaves no owned file", func(t *testing.T) {
		fixture := newFixture(t)
		original := capture(t, fixture.Home)
		if _, err := fixture.Adapter.Apply(t.Context(), fixture.Dependencies, fixture.Before, fixture.After); err != nil {
			t.Fatalf("Apply() = %v", err)
		}
		withdrawn := fixture.After.Clone()
		fixture.Adapter.Withdraw(&withdrawn)
		if _, err := fixture.Adapter.Apply(t.Context(), fixture.Dependencies, fixture.After, withdrawn); err != nil {
			t.Fatalf("Apply() after Withdraw() = %v", err)
		}
		requireUnchanged(t, original, capture(t, fixture.Home), "withdrawal")
	})
}

// capture records the bytes and mode of every regular file below home.
func capture(t *testing.T, home string) map[string]transaction.FileSnapshot {
	t.Helper()
	files := map[string]transaction.FileSnapshot{}
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return err
		}
		snapshot, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = snapshot
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("capture %s: %v", home, err)
	}
	return files
}

func changedPaths(before, after map[string]transaction.FileSnapshot) []string {
	var changed []string
	for path, snapshot := range after {
		if previous, existed := before[path]; !existed || !previous.Equal(snapshot) {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, kept := after[path]; !kept {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func requireUnchanged(t *testing.T, want, got map[string]transaction.FileSnapshot, operation string) {
	t.Helper()
	if changed := changedPaths(want, got); len(changed) > 0 {
		t.Fatalf("%s changed %v", operation, changed)
	}
}

func changesState(plans []client.ProjectionPlan) bool {
	return slices.ContainsFunc(plans, func(plan client.ProjectionPlan) bool { return plan.ChangesState })
}
