// Package contract checks the invariants of the client seam against one adapter
// in a private fixture home. Every admitted adapter and the test-only rehearsal
// adapter run it from their tests.
package contract

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
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
//   - Inspect, Verify and Plan change no file and leave the caller's
//     configuration as it was;
//   - Apply changes only files that the plans list in their Effects, and each
//     plan changes state exactly when Apply changes one of its effects;
//   - a converged projection plans and applies no change;
//   - Rollback restores every file's bytes and mode;
//   - withdrawal is planned the same way and leaves no owned file behind.
func Run(t *testing.T, newFixture func(t *testing.T) Fixture) {
	t.Helper()
	for _, invariant := range invariants {
		t.Run(invariant.name, func(t *testing.T) {
			if err := invariant.check(t.Context(), newFixture(t)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// invariant is one property of the seam that every adapter keeps.
type invariant struct {
	name  string
	check func(ctx context.Context, fixture Fixture) error
}

var invariants = []invariant{
	{name: "inspection and verification touch nothing", check: observationTouchesNothing},
	{name: "plans predict every change and rollback restores it", check: plansPredictEveryChange},
	{name: "withdrawal leaves no owned file", check: withdrawalLeavesNoOwnedFile},
}

func observationTouchesNothing(ctx context.Context, fixture Fixture) error {
	original, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	caller := fixture.After.Clone()
	fixture.Adapter.Inspect(ctx, fixture.Dependencies, fixture.After, fixture.Runtime)
	if err := unchangedCaller("Inspect", caller, fixture.After); err != nil {
		return err
	}
	_, _ = fixture.Adapter.Verify(ctx, fixture.Dependencies, fixture.After, fixture.Runtime, "")
	if err := unchangedCaller("Verify", caller, fixture.After); err != nil {
		return err
	}
	return unchangedFiles(fixture.Home, original, "inspection and verification")
}

func plansPredictEveryChange(ctx context.Context, fixture Fixture) error {
	original, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	plans, err := plan(fixture, fixture.Before, fixture.After)
	if err != nil {
		return err
	}
	if err := unchangedFiles(fixture.Home, original, "planning"); err != nil {
		return err
	}
	receipt, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, fixture.Before, fixture.After)
	if err != nil {
		return fmt.Errorf("Apply() = %w", err)
	}
	if receipt == nil || !changesState(plans) {
		return fmt.Errorf("Apply() of an enabled projection returned receipt %v after plans %#v", receipt, plans)
	}
	projected, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	if err := effectsPredicted(fixture.Home, plans, original, projected); err != nil {
		return err
	}
	if err := convergedProjectionChangesNothing(ctx, fixture, projected); err != nil {
		return err
	}
	if err := receipt.Rollback(); err != nil {
		return fmt.Errorf("Rollback() = %w", err)
	}
	return unchangedFiles(fixture.Home, original, "rollback")
}

func convergedProjectionChangesNothing(ctx context.Context, fixture Fixture, projected map[string]transaction.FileSnapshot) error {
	converged, err := plan(fixture, fixture.After, fixture.After)
	if err != nil {
		return err
	}
	if changesState(converged) {
		return fmt.Errorf("a converged projection plans changes: %#v", converged)
	}
	if _, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, fixture.After, fixture.After); err != nil {
		return fmt.Errorf("Apply() of a converged projection = %w", err)
	}
	return unchangedFiles(fixture.Home, projected, "applying a converged projection")
}

func withdrawalLeavesNoOwnedFile(ctx context.Context, fixture Fixture) error {
	original, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	if _, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, fixture.Before, fixture.After); err != nil {
		return fmt.Errorf("Apply() = %w", err)
	}
	projected, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	withdrawn := fixture.After.Clone()
	fixture.Adapter.Withdraw(&withdrawn)
	plans, err := plan(fixture, fixture.After, withdrawn)
	if err != nil {
		return err
	}
	if _, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, fixture.After, withdrawn); err != nil {
		return fmt.Errorf("Apply() after Withdraw() = %w", err)
	}
	restored, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	if err := effectsPredicted(fixture.Home, plans, projected, restored); err != nil {
		return fmt.Errorf("withdrawal: %w", err)
	}
	return unchangedFiles(fixture.Home, original, "withdrawal")
}

// plan returns the adapter's plans for the transition and requires planning to
// leave both of the caller's configurations as they were.
func plan(fixture Fixture, before, after configuration.Config) ([]client.ProjectionPlan, error) {
	callerBefore, callerAfter := before.Clone(), after.Clone()
	plans, err := fixture.Adapter.Plan(fixture.Dependencies, before, after)
	if err != nil {
		return nil, fmt.Errorf("Plan() = %w", err)
	}
	return plans, errors.Join(unchangedCaller("Plan", callerBefore, before), unchangedCaller("Plan", callerAfter, after))
}

// unchangedCaller requires an operation to leave the caller's configuration as
// it was; caller is the configuration's clone from before the call.
func unchangedCaller(operation string, caller, current configuration.Config) error {
	if !reflect.DeepEqual(caller, current.Clone()) {
		return fmt.Errorf("%s changed the caller's configuration", operation)
	}
	return nil
}

// effectsPredicted requires Apply to change only files that the plans list in
// their Effects, and each plan to change state exactly when Apply changed one
// of its effects.
func effectsPredicted(home string, plans []client.ProjectionPlan, before, after map[string]transaction.FileSnapshot) error {
	changed := changedPaths(before, after)
	declared := map[string]bool{}
	var violations []error
	for _, plan := range plans {
		effects, err := homePaths(home, plan.Effects)
		if err != nil {
			return err
		}
		touched := slices.ContainsFunc(effects, func(effect string) bool { return slices.Contains(changed, effect) })
		if touched != plan.ChangesState {
			violations = append(violations, fmt.Errorf("the plan for %s states ChangesState %t, but Apply changed %v and the plan's effects are %v", plan.Target, plan.ChangesState, changed, effects))
		}
		for _, effect := range effects {
			declared[effect] = true
		}
	}
	for _, path := range changed {
		if !declared[path] {
			violations = append(violations, fmt.Errorf("Apply changed %s, which no plan lists among its effects", path))
		}
	}
	return errors.Join(violations...)
}

// homePaths names effects relative to home, as capture names files. Both sides
// resolve symbolic links first, so a plan may name a file through either
// spelling of the temporary directory that holds the home.
func homePaths(home string, effects []string) ([]string, error) {
	root := resolve(home)
	paths := make([]string, 0, len(effects))
	for _, effect := range effects {
		relative, err := filepath.Rel(root, resolve(effect))
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("the plan effect %s lies outside the fixture home %s", effect, home)
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	return paths, nil
}

// resolve resolves the symbolic links in the longest existing prefix of path,
// so that a file that does not exist yet resolves like its directory.
func resolve(path string) string {
	prefix, rest := filepath.Clean(path), ""
	for {
		if resolved, err := filepath.EvalSymlinks(prefix); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(prefix)
		if parent == prefix {
			return filepath.Clean(path)
		}
		rest = filepath.Join(filepath.Base(prefix), rest)
		prefix = parent
	}
}

// capture records the bytes and mode of every regular file below home.
func capture(home string) (map[string]transaction.FileSnapshot, error) {
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
		return nil, fmt.Errorf("capture %s: %w", home, err)
	}
	return files, nil
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

func unchangedFiles(home string, want map[string]transaction.FileSnapshot, operation string) error {
	got, err := capture(home)
	if err != nil {
		return err
	}
	if changed := changedPaths(want, got); len(changed) > 0 {
		return fmt.Errorf("%s changed %v", operation, changed)
	}
	return nil
}

func changesState(plans []client.ProjectionPlan) bool {
	return slices.ContainsFunc(plans, func(plan client.ProjectionPlan) bool { return plan.ChangesState })
}
