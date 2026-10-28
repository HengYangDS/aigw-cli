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
//   - Inspect and Verify, before and after a projection, and every Plan and
//     Apply leave the caller's configuration as it was, and Inspect, Verify and
//     Plan change nothing below Home;
//   - Apply changes only files that the plans list in their Effects, each plan
//     changes state exactly when Apply changes one of its effects, and the
//     target of a changing plan exists after a projection and, when it is a
//     file, is listed among the plan's effects;
//   - a converged projection plans, applies and rolls back no change;
//   - Rollback restores every regular file's bytes and mode, every symbolic
//     link and every other entry below Home; only directories that the
//     projection created may remain;
//   - withdrawal is planned and checked the same way, leaves no owned file, and
//     its own receipt restores the projection; it may remove the directories
//     that the projection created.
//
// Each invariant runs with HOME, USERPROFILE, APPDATA, LOCALAPPDATA,
// XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME, XDG_CACHE_HOME,
// XDG_RUNTIME_DIR, TMPDIR, TMP and TEMP, and after the fixture is prepared also
// the working directory, pointed at an empty sentinel that must stay empty, and
// the fixture may not redirect them. That refuses an adapter which writes
// through those variables or a relative path instead of the observed host.
// Client-specific variables, values read before Run starts, the operating
// system's user database and Windows known folders are not covered.
func Run(t *testing.T, newFixture func(t *testing.T) Fixture) {
	t.Helper()
	for _, property := range invariants {
		t.Run(property.name, func(t *testing.T) {
			if err := runInvariant(t, property, newFixture); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// runInvariant isolates the process home before it prepares the fixture, so
// that an adapter which captures the home while it is constructed captures the
// sentinel, and then checks one invariant.
func runInvariant(t *testing.T, property invariant, newFixture func(t *testing.T) Fixture) error {
	t.Helper()
	sentinel, sentinelUnchanged := isolateProcessHome(t)
	fixture := newFixture(t)
	if err := isolationHolds(sentinel); err != nil {
		return err
	}
	t.Chdir(sentinel)
	return errors.Join(property.check(t.Context(), fixture), sentinelUnchanged())
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
	"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR",
	"TMPDIR", "TMP", "TEMP",
}

// isolationHolds requires every redirected variable to still name the
// sentinel, so that a fixture cannot undo the isolation.
func isolationHolds(sentinel string) error {
	for _, name := range processHomeVariables {
		if value := os.Getenv(name); value != sentinel {
			return fmt.Errorf("the fixture set %s to %q, out of the process home sentinel", name, value)
		}
	}
	return nil
}

// isolateProcessHome points the process's home and temporary directories at one
// empty sentinel for the rest of the test. It returns the sentinel and the check
// that the sentinel is still empty; temporary files that an operation removes
// again leave it empty. The sentinel is created before any variable changes, so
// the test's later temporary directories stay outside it.
func isolateProcessHome(t *testing.T) (string, func() error) {
	t.Helper()
	sentinel := t.TempDir()
	for _, name := range processHomeVariables {
		t.Setenv(name, sentinel)
	}
	return sentinel, func() error {
		entries, err := os.ReadDir(sentinel)
		if err != nil {
			return fmt.Errorf("read the process home sentinel: %w", err)
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if len(names) > 0 {
			return fmt.Errorf("the adapter left %v in the process's home, temporary or working directory instead of the fixture home", names)
		}
		return nil
	}
}

func observationTouchesNothing(ctx context.Context, fixture Fixture) error {
	return observeWithoutEffects(ctx, fixture, "before a projection")
}

// observeWithoutEffects requires Inspect, and Verify both without and with an
// explicit Route, to change nothing below Home and leave the caller's
// configuration as it was.
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
	receipt, err := apply(ctx, fixture, fixture.Before, fixture.After)
	if err != nil {
		return err
	}
	if receipt == nil || !changesState(plans) {
		return fmt.Errorf("Apply() of an enabled projection returned receipt %v after plans %#v", receipt, plans)
	}
	projected, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	created := createdDirectories(original, projected)
	if err := errors.Join(effectsPredicted(fixture.Home, plans, original, projected, created), targetsProjected(fixture.Home, plans)); err != nil {
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
	return restoredFiles(fixture.Home, original, created, "rollback")
}

// targetsProjected requires the target of every plan that changes state to
// exist after a projection and, when it is a regular file, to be listed among
// the plan's effects, as the plan's printed target promises.
func targetsProjected(home string, plans []client.ProjectionPlan) error {
	var violations []error
	for _, plan := range plans {
		if !plan.ChangesState {
			continue
		}
		info, err := os.Stat(plan.Target)
		if err != nil {
			violations = append(violations, fmt.Errorf("the planned target %s was not projected: %w", plan.Target, err))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		target, err := homePaths(home, []string{plan.Target})
		if err != nil {
			violations = append(violations, fmt.Errorf("the planned target %s lies outside the fixture home %s", plan.Target, home))
			continue
		}
		// An effect outside the home is reported by effectsPredicted.
		if effects, err := homePaths(home, plan.Effects); err == nil && !slices.Contains(effects, target[0]) {
			violations = append(violations, fmt.Errorf("the planned target %s is a file that the plan's effects do not list", plan.Target))
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
	receipt, err := apply(ctx, fixture, fixture.After, fixture.After)
	if err != nil {
		return fmt.Errorf("a converged projection: %w", err)
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
	if _, err := apply(ctx, fixture, fixture.Before, fixture.After); err != nil {
		return err
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
	receipt, err := apply(ctx, fixture, fixture.After, withdrawn)
	if err != nil {
		return fmt.Errorf("withdrawal: %w", err)
	}
	restored, err := capture(fixture.Home)
	if err != nil {
		return err
	}
	created := createdDirectories(original, projected)
	if err := effectsPredicted(fixture.Home, plans, projected, restored, created); err != nil {
		return fmt.Errorf("withdrawal: %w", err)
	}
	if err := restoredFiles(fixture.Home, original, created, "withdrawal"); err != nil {
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

// apply runs Apply for the transition and requires it to leave both of the
// caller's configurations as they were.
func apply(ctx context.Context, fixture Fixture, before, after configuration.Config) (client.ProjectionReceipt, error) {
	callerBefore, callerAfter := before.Clone(), after.Clone()
	receipt, err := fixture.Adapter.Apply(ctx, fixture.Dependencies, before, after)
	if err != nil {
		return nil, fmt.Errorf("Apply() = %w", err)
	}
	return receipt, errors.Join(unchangedCaller("Apply", callerBefore, before), unchangedCaller("Apply", callerAfter, after))
}

// unchangedCaller requires an operation to leave the caller's configuration as
// it was; caller is the configuration's clone from before the call.
func unchangedCaller(operation string, caller, current configuration.Config) error {
	if !reflect.DeepEqual(caller, current.Clone()) {
		return fmt.Errorf("%s changed the caller's configuration", operation)
	}
	return nil
}

// effectsPredicted requires Apply to change only entries that the plans list in
// their Effects, and each plan to change state exactly when Apply changed one
// of its effects. The directories in created, which the projection created, are
// not effects: Apply may create them and a withdrawal may remove them.
func effectsPredicted(home string, plans []client.ProjectionPlan, before, after tree, created map[string]bool) error {
	changed := changedPaths(before, after, created)
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

// homePaths names effects relative to home, as capture names entries. The
// effect's directory and the home resolve symbolic links first, so a plan may
// name a file through either spelling of the temporary directory that holds
// the home; the effect's own name is kept, so a listed symbolic link is that
// link and not its target.
func homePaths(home string, effects []string) ([]string, error) {
	root := resolve(home)
	paths := make([]string, 0, len(effects))
	for _, effect := range effects {
		relative, err := filepath.Rel(root, filepath.Join(resolve(filepath.Dir(effect)), filepath.Base(effect)))
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("the plan effect %s lies outside the fixture home %s", effect, home)
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	return paths, nil
}

// resolve resolves the symbolic links in the longest existing prefix of path,
// so that a directory that does not exist yet resolves like its parent.
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

// tree records every entry below a home by its slash-separated path relative to
// it: regular files by bytes and mode, symbolic links by target, and every other
// entry, directories included, by type and permission bits.
type tree struct {
	files map[string]transaction.FileSnapshot
	links map[string]string
	nodes map[string]fs.FileMode
}

func capture(home string) (tree, error) {
	captured := tree{files: map[string]transaction.FileSnapshot{}, links: map[string]string{}, nodes: map[string]fs.FileMode{}}
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == home {
			return err
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			captured.links[name] = target
			return err
		case entry.Type().IsRegular():
			snapshot, err := transaction.CaptureFileSnapshot(path)
			captured.files[name] = snapshot
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		captured.nodes[name] = info.Mode().Type() | info.Mode().Perm()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return tree{}, fmt.Errorf("capture %s: %w", home, err)
	}
	return captured, nil
}

// createdDirectories names the directories that exist in projected where
// nothing stood in original: the directories that a projection created.
func createdDirectories(original, projected tree) map[string]bool {
	created := map[string]bool{}
	for path, mode := range projected.nodes {
		if mode.IsDir() && !original.holds(path) {
			created[path] = true
		}
	}
	return created
}

// holds reports whether any entry stands at path.
func (captured tree) holds(path string) bool {
	_, file := captured.files[path]
	_, link := captured.links[path]
	_, node := captured.nodes[path]
	return file || link || node
}

// changedPaths lists every entry that changed between two captures. A path in
// exempt may appear or go as a directory: it does not count when one side holds
// nothing there and the other side nothing or a directory. A file, link or
// other entry there, or a directory whose mode changed, still counts.
func changedPaths(before, after tree, exempt map[string]bool) []string {
	changed := map[string]bool{}
	collectChanges(changed, before.files, after.files, transaction.FileSnapshot.Equal)
	collectChanges(changed, before.links, after.links, equalValues[string])
	collectChanges(changed, before.nodes, after.nodes, equalValues[fs.FileMode])
	for path := range exempt {
		if !before.holds(path) && after.onlyDirectory(path) || before.onlyDirectory(path) && !after.holds(path) {
			delete(changed, path)
		}
	}
	return slices.Sorted(maps.Keys(changed))
}

// onlyDirectory reports whether path holds nothing or a directory.
func (captured tree) onlyDirectory(path string) bool {
	_, file := captured.files[path]
	_, link := captured.links[path]
	mode, node := captured.nodes[path]
	return !file && !link && (!node || mode.IsDir())
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

func equalValues[V comparable](left, right V) bool { return left == right }

// unchangedFiles requires every entry below home to match want.
func unchangedFiles(home string, want tree, operation string) error {
	return compareHome(home, want, nil, operation)
}

// restoredFiles requires every entry below home to match want, except that the
// directories in created, which the projection created, may remain or go.
func restoredFiles(home string, want tree, created map[string]bool, operation string) error {
	return compareHome(home, want, created, operation)
}

func compareHome(home string, want tree, exempt map[string]bool, operation string) error {
	got, err := capture(home)
	if err != nil {
		return err
	}
	if changed := changedPaths(want, got, exempt); len(changed) > 0 {
		return fmt.Errorf("%s changed %v", operation, changed)
	}
	return nil
}

func changesState(plans []client.ProjectionPlan) bool {
	return slices.ContainsFunc(plans, func(plan client.ProjectionPlan) bool { return plan.ChangesState })
}
