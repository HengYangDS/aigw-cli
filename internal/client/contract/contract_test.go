package contract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

const (
	fixtureID     = "fixture"
	targetName    = "planned.txt"
	companionName = "companion.txt"
	linkName      = "link"
	existingName  = "existing"
)

// These name the one seam promise that a fixtureAdapter breaks;
// keepsEveryPromise, listsItsCompanion and listsItsLink keep every promise.
const (
	keepsEveryPromise          = ""
	listsItsCompanion          = "lists its companion"
	listsItsLink               = "lists the symbolic link that it creates"
	writesUnlistedCompanion    = "writes an unlisted companion"
	writesOnlyCompanion        = "writes only an unlisted companion"
	writesUnlistedLink         = "creates an unlisted symbolic link"
	writesNothing              = "writes nothing"
	statesNoChange             = "states no change"
	listsOutsideHome           = "lists a file outside the home"
	namesMissingTarget         = "names a target that it never writes"
	namesUnlistedTarget        = "names a target file that its effects do not list"
	inspectMutates             = "Inspect changes the caller's configuration"
	verifyMutates              = "Verify changes the caller's configuration"
	planMutates                = "Plan changes the caller's configuration"
	applyMutates               = "Apply changes the caller's configuration"
	planWrites                 = "writes while it plans a projection"
	planCreatesDirectory       = "creates a directory while it plans a projection"
	withdrawalPlanWrites       = "withdraws while it plans the withdrawal"
	inspectWritesProjected     = "Inspect writes after a projection"
	verifyWritesForRoute       = "Verify writes for an explicit Route"
	plansWhenConverged         = "plans a change when converged"
	convergedWrites            = "writes when converged"
	convergedRollbackWrites    = "rolls back a converged projection by writing"
	rollbackKeeps              = "keeps its projection on rollback"
	rollbackKeepsMode          = "keeps the target's mode on rollback"
	changesDirectoryMode       = "changes the mode of an existing directory"
	withdrawalKeepsCompanion   = "keeps its companion after withdrawal"
	withdrawalRollbackKeeps    = "keeps the withdrawal on rollback"
	withdrawalReturnsNoReceipt = "returns no receipt for its withdrawal"
	projectionReturnsNoReceipt = "returns no receipt for its projection"
	writesProcessHome          = "writes the process home"
	capturesProcessHome        = "captures the process home when it is constructed"
	writesRelativePath         = "writes a path relative to the working directory"
)

// fixtureAdapter projects one target file below home, and a companion file or
// symbolic link for the promises that involve one. outside names a file outside
// home, and capturedHome the process home when the fixture was prepared.
type fixtureAdapter struct {
	home, outside, capturedHome string
	breaks                      string
}

func (fixtureAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: fixtureID, Label: "Fixture"}
}

func (fixtureAdapter) Discover(client.DiscoverySource) discovery.Result { return discovery.Result{} }

func (fixtureAdapter) Converge(client.Dependencies, *configuration.Config, discovery.Result) error {
	return nil
}

func (fixtureAdapter) AdoptSelection(client.Dependencies, *configuration.Config) (string, error) {
	return "", nil
}

func (fixtureAdapter) ProjectionChanged(before, after configuration.Config) bool {
	return enabled(before) != enabled(after)
}

func (fixtureAdapter) Withdraw(cfg *configuration.Config) { delete(cfg.Clients, fixtureID) }

func enabled(cfg configuration.Config) bool { return cfg.Clients[fixtureID].Enabled }

func (adapter fixtureAdapter) path(name string) string { return filepath.Join(adapter.home, name) }

func (adapter fixtureAdapter) write(name, content string) error {
	return os.WriteFile(adapter.path(name), []byte(content), 0o600)
}

// mutate changes another client's binding through the caller's configuration
// when operation is the promise that the fixture breaks.
func (adapter fixtureAdapter) mutate(operation string, cfg configuration.Config) {
	if adapter.breaks == operation {
		binding := cfg.Clients["foreign"]
		binding.Executable += "/mutated"
		cfg.Clients["foreign"] = binding
	}
}

func (adapter fixtureAdapter) Inspect(_ context.Context, _ client.Dependencies, cfg configuration.Config, _ configuration.Runtime) client.Status {
	adapter.mutate(inspectMutates, cfg)
	if data, _ := os.ReadFile(adapter.path(targetName)); adapter.breaks == inspectWritesProjected && string(data) == "projected" {
		_ = adapter.write("inspected.txt", "inspected")
	}
	return client.Status{Ready: true}
}

func (adapter fixtureAdapter) Verify(_ context.Context, _ client.Dependencies, cfg configuration.Config, _ configuration.Runtime, route string) (client.Verification, error) {
	adapter.mutate(verifyMutates, cfg)
	if adapter.breaks == verifyWritesForRoute && route != "" {
		return client.Verification{}, adapter.write("verified.txt", route)
	}
	return client.Verification{Scope: fixtureID}, nil
}

func (adapter fixtureAdapter) Plan(_ client.Dependencies, before, after configuration.Config) ([]client.ProjectionPlan, error) {
	adapter.mutate(planMutates, after)
	enabling, withdrawing := enabled(after) && !enabled(before), enabled(before) && !enabled(after)
	if err := adapter.planSideEffects(enabling, withdrawing); err != nil {
		return nil, err
	}
	target, effects := adapter.path(targetName), []string{adapter.path(targetName)}
	switch adapter.breaks {
	case listsItsCompanion, withdrawalKeepsCompanion:
		effects = append(effects, adapter.path(companionName))
	case listsItsLink:
		effects = append(effects, adapter.path(linkName))
	case listsOutsideHome:
		effects = append(effects, adapter.outside)
	case namesMissingTarget:
		target = adapter.path("missing.txt")
	case namesUnlistedTarget:
		target = adapter.path(filepath.Join(existingName, "readme.txt"))
	}
	changes := (enabling || withdrawing) && adapter.breaks != statesNoChange ||
		adapter.breaks == plansWhenConverged && enabled(before) && enabled(after)
	return []client.ProjectionPlan{{Client: fixtureID, Target: target, Action: "project", Effects: effects, ChangesState: changes}}, nil
}

// planSideEffects performs the writes that the planning promises forbid.
func (adapter fixtureAdapter) planSideEffects(enabling, withdrawing bool) error {
	switch {
	case adapter.breaks == planWrites && enabling:
		return adapter.write("side.txt", "planned")
	case adapter.breaks == planCreatesDirectory && enabling:
		return os.Mkdir(adapter.path("planned-dir"), 0o700)
	case adapter.breaks == withdrawalPlanWrites && withdrawing:
		return adapter.restore()
	}
	return nil
}

func (adapter fixtureAdapter) Apply(_ context.Context, _ client.Dependencies, before, after configuration.Config) (client.ProjectionReceipt, error) {
	adapter.mutate(applyMutates, after)
	switch {
	case enabled(before) && enabled(after):
		return adapter.applyConverged()
	case enabled(before):
		return adapter.withdraw()
	case enabled(after):
		return adapter.project()
	}
	return nil, nil
}

func (adapter fixtureAdapter) applyConverged() (client.ProjectionReceipt, error) {
	switch adapter.breaks {
	case convergedWrites:
		return nil, adapter.write(targetName, "converged")
	case convergedRollbackWrites:
		return fixtureReceipt(func() error { return adapter.write("rolled.txt", "rolled") }), nil
	}
	return nil, nil
}

func (adapter fixtureAdapter) project() (client.ProjectionReceipt, error) {
	writes := map[string]bool{
		targetName:    adapter.breaks != writesOnlyCompanion && adapter.breaks != writesNothing,
		companionName: slices.Contains([]string{listsItsCompanion, writesUnlistedCompanion, writesOnlyCompanion, withdrawalKeepsCompanion}, adapter.breaks),
	}
	for name, wanted := range writes {
		if wanted {
			if err := adapter.write(name, "projected"); err != nil {
				return nil, err
			}
		}
	}
	if err := adapter.projectExtras(); err != nil {
		return nil, err
	}
	switch adapter.breaks {
	case projectionReturnsNoReceipt:
		return nil, nil
	case rollbackKeeps:
		return fixtureReceipt(func() error { return nil }), nil
	}
	return fixtureReceipt(adapter.restore), nil
}

// projectExtras performs the projection's link, mode and out-of-home effects.
func (adapter fixtureAdapter) projectExtras() error {
	switch adapter.breaks {
	case listsItsLink, writesUnlistedLink:
		return os.Symlink(targetName, adapter.path(linkName))
	case rollbackKeepsMode:
		return os.Chmod(adapter.path(targetName), 0o644)
	case changesDirectoryMode:
		return os.Chmod(adapter.path(existingName), 0o755)
	case writesProcessHome:
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(home, "fixture-state"), []byte("projected"), 0o600)
	case capturesProcessHome:
		return os.WriteFile(filepath.Join(adapter.capturedHome, "fixture-state"), []byte("projected"), 0o600)
	case writesRelativePath:
		return os.WriteFile("fixture-state", []byte("projected"), 0o600)
	}
	return nil
}

func (adapter fixtureAdapter) withdraw() (client.ProjectionReceipt, error) {
	if adapter.breaks == withdrawalPlanWrites {
		return nil, nil
	}
	restore := adapter.restore
	if adapter.breaks == withdrawalKeepsCompanion {
		restore = adapter.restoreTarget
	}
	if err := restore(); err != nil {
		return nil, err
	}
	switch adapter.breaks {
	case withdrawalReturnsNoReceipt:
		return nil, nil
	case withdrawalRollbackKeeps:
		return fixtureReceipt(func() error { return nil }), nil
	}
	return fixtureReceipt(func() error {
		_, err := adapter.project()
		return err
	}), nil
}

// restore returns the target to its original bytes and removes the companion
// and the link.
func (adapter fixtureAdapter) restore() error {
	for _, name := range []string{companionName, linkName} {
		if err := os.Remove(adapter.path(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return adapter.restoreTarget()
}

func (adapter fixtureAdapter) restoreTarget() error { return adapter.write(targetName, "original") }

type fixtureReceipt func() error

func (rollback fixtureReceipt) Rollback() error { return rollback() }

// newFixture places the adapter's target and an existing directory holding a
// user's file in a private home. Before binds only a foreign client, and After
// also enables the fixture client.
func newFixture(t *testing.T, breaks string) Fixture {
	t.Helper()
	adapter := fixtureAdapter{home: t.TempDir(), outside: filepath.Join(t.TempDir(), "outside.txt"), breaks: breaks}
	if home, err := os.UserHomeDir(); err == nil {
		adapter.capturedHome = home
	}
	if err := adapter.restoreTarget(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(adapter.path(existingName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := adapter.write(filepath.Join(existingName, "readme.txt"), "user"); err != nil {
		t.Fatal(err)
	}
	before := configuration.NewConfig()
	before.SetClientActivation("foreign", true, "/fixture/foreign", nil)
	after := before.Clone()
	after.SetClientActivation(fixtureID, true, "/fixture/client", nil)
	return Fixture{Adapter: adapter, Home: adapter.home, Before: before, After: after}
}

// Each case keeps or breaks one seam promise. refused maps every invariant that
// must reject the case to a phrase of its reason; the other invariants pass.
func TestInvariantsRefuseEveryBrokenPromise(t *testing.T) {
	observation, plans, withdrawal := invariants[0].name, invariants[1].name, invariants[2].name
	unlisted := "which no plan lists"
	for _, test := range []struct {
		breaks  string
		refused map[string]string
	}{
		{breaks: keepsEveryPromise},
		{breaks: listsItsCompanion},
		{breaks: listsItsLink},
		{breaks: writesUnlistedCompanion, refused: map[string]string{plans: unlisted, withdrawal: unlisted}},
		{breaks: writesOnlyCompanion, refused: map[string]string{plans: "states ChangesState true", withdrawal: unlisted}},
		{breaks: writesUnlistedLink, refused: map[string]string{plans: "Apply changed link, " + unlisted, withdrawal: "Apply changed link, " + unlisted}},
		{breaks: writesNothing, refused: map[string]string{plans: "states ChangesState true", withdrawal: "states ChangesState true"}},
		{breaks: statesNoChange, refused: map[string]string{plans: "Apply() of an enabled projection", withdrawal: "states ChangesState false"}},
		{breaks: listsOutsideHome, refused: map[string]string{plans: "lies outside the fixture home", withdrawal: "lies outside the fixture home"}},
		{breaks: namesMissingTarget, refused: map[string]string{plans: "was not projected"}},
		{breaks: namesUnlistedTarget, refused: map[string]string{plans: "is a file that the plan's effects do not list"}},
		{breaks: inspectMutates, refused: map[string]string{observation: "Inspect changed the caller's configuration", plans: "Inspect changed the caller's configuration"}},
		{breaks: verifyMutates, refused: map[string]string{observation: "Verify changed the caller's configuration", plans: "Verify changed the caller's configuration"}},
		{breaks: planMutates, refused: map[string]string{plans: "Plan changed the caller's configuration", withdrawal: "Plan changed the caller's configuration"}},
		{breaks: applyMutates, refused: map[string]string{plans: "Apply changed the caller's configuration", withdrawal: "Apply changed the caller's configuration"}},
		{breaks: planWrites, refused: map[string]string{plans: "planning changed [side.txt]"}},
		{breaks: planCreatesDirectory, refused: map[string]string{plans: "planning changed [planned-dir]"}},
		{breaks: withdrawalPlanWrites, refused: map[string]string{withdrawal: "planning a withdrawal changed"}},
		{breaks: inspectWritesProjected, refused: map[string]string{plans: "inspection and verification after a projection changed"}},
		{breaks: verifyWritesForRoute, refused: map[string]string{observation: "inspection and verification before a projection changed", plans: "inspection and verification after a projection changed"}},
		{breaks: plansWhenConverged, refused: map[string]string{plans: "a converged projection plans changes"}},
		{breaks: convergedWrites, refused: map[string]string{plans: "applying a converged projection changed"}},
		{breaks: convergedRollbackWrites, refused: map[string]string{plans: "rolling back a converged projection changed"}},
		{breaks: rollbackKeeps, refused: map[string]string{plans: "rollback changed [planned.txt]"}},
		{breaks: rollbackKeepsMode, refused: map[string]string{plans: "rollback changed [planned.txt]", withdrawal: "withdrawal changed [planned.txt]"}},
		{breaks: changesDirectoryMode, refused: map[string]string{plans: "Apply changed existing, " + unlisted, withdrawal: "withdrawal changed [existing]"}},
		{breaks: withdrawalKeepsCompanion, refused: map[string]string{withdrawal: "withdrawal changed [companion.txt]"}},
		{breaks: withdrawalRollbackKeeps, refused: map[string]string{withdrawal: "rolling back a withdrawal changed"}},
		{breaks: withdrawalReturnsNoReceipt, refused: map[string]string{withdrawal: "returned no receipt"}},
		{breaks: projectionReturnsNoReceipt, refused: map[string]string{plans: "Apply() of an enabled projection returned receipt <nil>"}},
	} {
		name := test.breaks
		if name == "" {
			name = "keeps every promise"
		}
		t.Run(name, func(t *testing.T) {
			for _, property := range invariants {
				err := property.check(t.Context(), newFixture(t, test.breaks))
				want, refused := test.refused[property.name]
				if refused != (err != nil) || refused && !strings.Contains(err.Error(), want) {
					t.Errorf("%s: %v, want refusal %q", property.name, err, want)
				}
			}
		})
	}
}

// Run isolates the process home before it prepares the fixture and runs the
// operations from the sentinel, so an adapter that writes through the process
// home, through a home it captured while it was constructed, or through a
// relative path is refused, while an honest one passes.
func TestRunRefusesWritesOutsideTheObservedHost(t *testing.T) {
	for _, test := range []struct {
		breaks string
		want   string
	}{
		{breaks: keepsEveryPromise},
		{breaks: writesProcessHome, want: "instead of the fixture home"},
		{breaks: capturesProcessHome, want: "instead of the fixture home"},
		{breaks: writesRelativePath, want: "instead of the fixture home"},
	} {
		name := test.breaks
		if name == "" {
			name = "keeps every promise"
		}
		t.Run(name, func(t *testing.T) {
			err := runInvariant(t, invariants[1], func(t *testing.T) Fixture { return newFixture(t, test.breaks) })
			if (err == nil) != (test.want == "") || err != nil && !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runInvariant() = %v, want refusal %q", err, test.want)
			}
		})
	}
}

// A temporary directory that an operation removes again leaves the sentinel
// empty, and a file left in the process home is reported by name.
func TestTheProcessHomeSentinelReportsAWrite(t *testing.T) {
	_, sentinelUnchanged := isolateProcessHome(t)
	if err := sentinelUnchanged(); err != nil {
		t.Fatalf("a fresh sentinel = %v, want empty", err)
	}
	temporary, err := os.MkdirTemp("", "probe-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(temporary); err != nil {
		t.Fatal(err)
	}
	if err := sentinelUnchanged(); err != nil {
		t.Fatalf("after a removed temporary directory = %v, want empty", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sentinelUnchanged(); err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Fatalf("after a write to the process home = %v, want a refusal naming settings.json", err)
	}
}
