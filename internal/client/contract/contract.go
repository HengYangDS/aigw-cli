// Package contract checks the invariants of the client seam against one adapter
// in a private fixture home. Every admitted adapter and the test-only rehearsal
// adapter run it from their tests.
package contract

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

// Fixture is one adapter's projection scenario. Every file that the adapter may
// write lives below Home. Before has no projection for the adapter, After
// enables it, and Runtime is After's selected runtime; its RouteID, when set, is
// the explicit Route that one verification names.
type Fixture struct {
	Adapter      client.Adapter
	Home         string
	Dependencies client.Dependencies
	Before       configuration.Config
	After        configuration.Config
	Runtime      configuration.Runtime
}

// Run checks the seam invariants for the fixtures that newFixture prepares:
//   - Inspect and Verify, before and after a projection, and Plan change no
//     file and leave the caller's configuration as it was;
//   - Apply changes only files that the plans list in their Effects, each plan
//     changes state exactly when Apply changes one of its effects, and the
//     target of a changing plan exists after a projection;
//   - a converged projection plans, applies and rolls back no change;
//   - Rollback restores every regular file's bytes and mode and every symbolic
//     link;
//   - withdrawal is planned and checked the same way, leaves no owned file, and
//     its own receipt restores the projection.
//
// Each invariant runs with the process's home, configuration and temporary
// directories pointed at an empty sentinel that must stay empty, so an adapter
// that bypasses the observed host cannot touch the developer's files.
// Directories that a projection creates may remain, as AIGW leaves them.
func Run(t *testing.T, newFixture func(t *testing.T) Fixture) {
	t.Helper()
	for _, invariant := range invariants {
		t.Run(invariant.name, func(t *testing.T) {
			fixture := newFixture(t)
			sentinelUnchanged := isolateProcessHome(t)
			if err := errors.Join(invariant.check(t.Context(), fixture), sentinelUnchanged()); err != nil {
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

// processHomeVariables name the directories through which code that ignores the
// observed host would reach the developer's home or temporary files.
var processHomeVariables = []string{
	"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
	"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
	"TMPDIR", "TMP", "TEMP",
}

// isolateProcessHome points the process's home and temporary directories at one
// empty sentinel for the rest of the test and returns the check that the
// sentinel is still empty. Temporary files that an operation removes again
// leave it empty.
func isolateProcessHome(t *testing.T) func() error {
	t.Helper()
	sentinel := t.TempDir()
	for _, name := range processHomeVariables {
		t.Setenv(name, sentinel)
	}
	return func() error {
		entries, err := os.ReadDir(sentinel)
		if err != nil {
			return fmt.Errorf("read the process home sentinel: %w", err)
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if len(names) > 0 {
			return fmt.Errorf("the adapter left %v in the process's home or temporary directory instead of the fixture home", names)
		}
		return nil
	}
}

func observationTouchesNothing(ctx context.Context, fixture Fixture) error {
	return observeWithoutEffects(ctx, fixture, "before a projection")
}

// observeWithoutEffects requires Inspect, and Verify both without and with an
// explicit Route, to change no file and leave the caller's configuration as it
// was.
func observeWithoutEffects(ctx context.Context, fixture Fixture, when string) error {
	before, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	caller := fixture.After.Clone()
	fixture.Adapter.Inspect(ctx, fixture.Dependencies, fixture.After, fixture.Runtime)
	if err := unchangedCaller("Inspect", caller, fixture.After); err != nil {
		return fmt.Errorf("%s: %w", when, err)
	}
	for _, route := range []string{"", explicitRoute(fixture)} {
		_, _ = fixture.Adapter.Verify(ctx, fixture.Dependencies, fixture.After, fixture.Runtime, route)
		if err := unchangedCaller("Verify", caller, fixture.After); err != nil {
			return fmt.Errorf("%s: %w", when, err)
		}
	}
	return unchangedFiles(fixture.Home, before, "inspection and verification "+when)
}

// explicitRoute is the Route that one verification names: the fixture's
// selected Route, or a placeholder when the fixture selects none.
func explicitRoute(fixture Fixture) string {
	if fixture.Runtime.RouteID != "" {
		return fixture.Runtime.RouteID
	}
	return "contract"
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
	if err := errors.Join(effectsPredicted(fixture.Home, plans, original, projected), targetsProjected(plans)); err != nil {
		return err
	}
	if err := observeWithoutEffects(ctx, fixture, "after a projection"); err != nil {
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

// targetsProjected requires the target of every plan that changes state to
// exist after a projection, as the plan's printed target promises.
func targetsProjected(plans []client.ProjectionPlan) error {
	var violations []error
	for _, plan := range plans {
		if _, err := os.Stat(plan.Target); plan.ChangesState && err != nil {
			violations = append(violations, fmt.Errorf("the planned target %s was not projected: %w", plan.Target, err))
		}
	}
	return errors.Join(violations...)
}

func convergedProjectionChangesNothing(ctx context.Context, fixture Fixture, projected tree) error {
	converged, err := plan(fixture, fixture.After, fixture.After)
	if err != nil {
		return err
	}
	if changesState(converged) {
		return fmt.Errorf("a converged projection plans changes: %#v", converged)
	}
	receipt, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, fixture.After, fixture.After)
	if err != nil {
		return fmt.Errorf("Apply() of a converged projection = %w", err)
	}
	if err := unchangedFiles(fixture.Home, projected, "applying a converged projection"); err != nil {
		return err
	}
	if receipt == nil {
		return nil
	}
	if err := receipt.Rollback(); err != nil {
		return fmt.Errorf("Rollback() of a converged projection = %w", err)
	}
	return unchangedFiles(fixture.Home, projected, "rolling back a converged projection")
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
	if err := unchangedFiles(fixture.Home, projected, "planning a withdrawal"); err != nil {
		return err
	}
	receipt, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, fixture.After, withdrawn)
	if err != nil {
		return fmt.Errorf("Apply() after Withdraw() = %w", err)
	}
	restored, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	if err := effectsPredicted(fixture.Home, plans, projected, restored); err != nil {
		return fmt.Errorf("withdrawal: %w", err)
	}
	if err := unchangedFiles(fixture.Home, original, "withdrawal"); err != nil {
		return err
	}
	if receipt == nil {
		return fmt.Errorf("Apply() of a withdrawal returned no receipt after plans %#v", plans)
	}
	if err := receipt.Rollback(); err != nil {
		return fmt.Errorf("Rollback() of a withdrawal = %w", err)
	}
	return unchangedFiles(fixture.Home, projected, "rolling back a withdrawal")
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
func effectsPredicted(home string, plans []client.ProjectionPlan, before, after tree) error {
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

// tree records the regular files and symbolic links below a home, by their
// slash-separated paths relative to it.
type tree struct {
	files map[string]transaction.FileSnapshot
	links map[string]string
}

// capture records the bytes and mode of every regular file and the target of
// every symbolic link below home.
func capture(home string) (tree, error) {
	captured := tree{files: map[string]transaction.FileSnapshot{}, links: map[string]string{}}
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			captured.links[filepath.ToSlash(relative)] = target
			return err
		case entry.Type().IsRegular():
			snapshot, err := transaction.CaptureFileSnapshot(path)
			captured.files[filepath.ToSlash(relative)] = snapshot
			return err
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return tree{}, fmt.Errorf("capture %s: %w", home, err)
	}
	return captured, nil
}

func changedPaths(before, after tree) []string {
	changed := map[string]bool{}
	collectChanges(changed, before.files, after.files, transaction.FileSnapshot.Equal)
	collectChanges(changed, before.links, after.links, func(left, right string) bool { return left == right })
	return slices.Sorted(maps.Keys(changed))
}

// collectChanges marks every path that only one side holds or that the two
// sides hold with unequal values.
func collectChanges[V any](changed map[string]bool, before, after map[string]V, equal func(V, V) bool) {
	for path, value := range after {
		if previous, existed := before[path]; !existed || !equal(previous, value) {
			changed[path] = true
		}
	}
	for path := range before {
		if _, kept := after[path]; !kept {
			changed[path] = true
		}
	}
}

func unchangedFiles(home string, want tree, operation string) error {
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
