package contract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

const (
	fixtureID     = "fixture"
	targetName    = "planned.txt"
	companionName = "companion.txt"
	linkName      = "link"
	existingName  = "existing"
	toolName      = ".tool"
)

// These name the one seam promise that a fixtureAdapter breaks;
// keepsEveryPromise, listsItsCompanion, listsItsLink, removesItsDirectory and
// keepsItsDirectory keep every promise.
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
	removesItsDirectory        = "removes the directory that its projection created"
	keepsItsDirectory          = "keeps the directory that its projection created"
	rollbackCreatesDirectory   = "creates a directory on rollback"
	fixtureRedirectsHome       = "redirects the process home while it is prepared"
	namesOutsideTarget         = "names a target outside the home"
	withdrawalCreatesDirectory = "creates a directory when it withdraws"
	withdrawalChangesItsMode   = "changes the mode of its directory when it withdraws"
	withdrawalChangesListed    = "changes the mode of its listed directory when it withdraws"
	rollbackChangesItsMode     = "changes the mode of its directory on rollback"
	withdrawalRollbackDrops    = "drops its directory when its withdrawal is rolled back"
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

func (fixtureAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{ID: fixtureID, Label: "Fixture"}
}

func (fixtureAdapter) Discover(adapter.DiscoverySource) discovery.Result { return discovery.Result{} }

func (fixtureAdapter) Converge(adapter.Dependencies, *domain.Config, discovery.Result) error {
	return nil
}

func (fixtureAdapter) AdoptSelection(adapter.Dependencies, *domain.Config) (string, error) {
	return "", nil
}

func (fixtureAdapter) ProjectionChanged(before, after domain.Config) bool {
	return enabled(before) != enabled(after)
}

func (fixtureAdapter) Withdraw(cfg *domain.Config) { delete(cfg.Clients, fixtureID) }

func enabled(cfg domain.Config) bool { return cfg.Clients[fixtureID].Enabled }

func (clientAdapter fixtureAdapter) path(name string) string {
	return filepath.Join(clientAdapter.home, name)
}

func (clientAdapter fixtureAdapter) write(name, content string) error {
	return os.WriteFile(clientAdapter.path(name), []byte(content), 0o600)
}

// mutate changes another client's binding through the caller's configuration
// when operation is the promise that the fixture breaks.
func (clientAdapter fixtureAdapter) mutate(operation string, cfg domain.Config) {
	if clientAdapter.breaks == operation {
		binding := cfg.Clients["foreign"]
		binding.Executable += "/mutated"
		cfg.Clients["foreign"] = binding
	}
}

func (clientAdapter fixtureAdapter) Inspect(_ context.Context, _ adapter.Dependencies, cfg domain.Config, _ domain.Runtime) adapter.Status {
	clientAdapter.mutate(inspectMutates, cfg)
	if data, _ := os.ReadFile(clientAdapter.path(targetName)); clientAdapter.breaks == inspectWritesProjected && string(data) == "projected" {
		_ = clientAdapter.write("inspected.txt", "inspected")
	}
	return adapter.Status{Ready: true}
}

func (clientAdapter fixtureAdapter) Verify(_ context.Context, _ adapter.Dependencies, cfg domain.Config, _ domain.Runtime, route string) (adapter.Verification, error) {
	clientAdapter.mutate(verifyMutates, cfg)
	if clientAdapter.breaks == verifyWritesForRoute && route != "" {
		return adapter.Verification{}, clientAdapter.write("verified.txt", route)
	}
	return adapter.Verification{Scope: fixtureID}, nil
}

func (clientAdapter fixtureAdapter) Plan(_ adapter.Dependencies, before, after domain.Config) ([]adapter.ProjectionPlan, error) {
	clientAdapter.mutate(planMutates, after)
	enabling, withdrawing := enabled(after) && !enabled(before), enabled(before) && !enabled(after)
	if err := clientAdapter.planSideEffects(enabling, withdrawing); err != nil {
		return nil, err
	}
	target, effects := clientAdapter.path(targetName), []string{clientAdapter.path(targetName)}
	switch clientAdapter.breaks {
	case listsItsCompanion, withdrawalKeepsCompanion:
		effects = append(effects, clientAdapter.path(companionName))
	case listsItsLink:
		effects = append(effects, clientAdapter.path(linkName))
	case withdrawalChangesListed:
		effects = append(effects, clientAdapter.path(toolName))
	case listsOutsideHome:
		effects = append(effects, clientAdapter.outside)
	case namesMissingTarget:
		target = clientAdapter.path("missing.txt")
	case namesUnlistedTarget:
		target = clientAdapter.path(filepath.Join(existingName, "readme.txt"))
	case namesOutsideTarget:
		target = clientAdapter.outside
	}
	changes := (enabling || withdrawing) && clientAdapter.breaks != statesNoChange ||
		clientAdapter.breaks == plansWhenConverged && enabled(before) && enabled(after)
	return []adapter.ProjectionPlan{{Client: fixtureID, Target: target, Action: "project", Effects: effects, ChangesState: changes}}, nil
}

// planSideEffects performs the writes that the planning promises forbid.
func (clientAdapter fixtureAdapter) planSideEffects(enabling, withdrawing bool) error {
	switch {
	case clientAdapter.breaks == planWrites && enabling:
		return clientAdapter.write("side.txt", "planned")
	case clientAdapter.breaks == planCreatesDirectory && enabling:
		return os.Mkdir(clientAdapter.path("planned-dir"), 0o700)
	case clientAdapter.breaks == withdrawalPlanWrites && withdrawing:
		return clientAdapter.restore()
	}
	return nil
}

func (clientAdapter fixtureAdapter) Apply(_ context.Context, _ adapter.Dependencies, before, after domain.Config) (adapter.ProjectionReceipt, error) {
	clientAdapter.mutate(applyMutates, after)
	switch {
	case enabled(before) && enabled(after):
		return clientAdapter.applyConverged()
	case enabled(before):
		return clientAdapter.withdraw()
	case enabled(after):
		return clientAdapter.project()
	}
	return nil, nil
}

func (clientAdapter fixtureAdapter) applyConverged() (adapter.ProjectionReceipt, error) {
	switch clientAdapter.breaks {
	case convergedWrites:
		return nil, clientAdapter.write(targetName, "converged")
	case convergedRollbackWrites:
		return fixtureReceipt(func() error { return clientAdapter.write("rolled.txt", "rolled") }), nil
	}
	return nil, nil
}

func (clientAdapter fixtureAdapter) project() (adapter.ProjectionReceipt, error) {
	writes := map[string]bool{
		targetName:    clientAdapter.breaks != writesOnlyCompanion && clientAdapter.breaks != writesNothing,
		companionName: slices.Contains([]string{listsItsCompanion, writesUnlistedCompanion, writesOnlyCompanion, withdrawalKeepsCompanion}, clientAdapter.breaks),
	}
	for name, wanted := range writes {
		if wanted {
			if err := clientAdapter.write(name, "projected"); err != nil {
				return nil, err
			}
		}
	}
	if err := clientAdapter.projectExtras(); err != nil {
		return nil, err
	}
	switch clientAdapter.breaks {
	case projectionReturnsNoReceipt:
		return nil, nil
	case rollbackKeeps:
		return fixtureReceipt(func() error { return nil }), nil
	case keepsItsDirectory:
		return fixtureReceipt(clientAdapter.restoreTarget), nil
	case rollbackCreatesDirectory:
		return fixtureReceipt(func() error {
			if err := clientAdapter.restore(); err != nil {
				return err
			}
			return os.Mkdir(clientAdapter.path(".backup"), 0o700)
		}), nil
	case rollbackChangesItsMode:
		return fixtureReceipt(func() error {
			if err := clientAdapter.restoreTarget(); err != nil {
				return err
			}
			return os.Chmod(clientAdapter.path(toolName), 0o755)
		}), nil
	}
	return fixtureReceipt(clientAdapter.restore), nil
}

// projectExtras performs the projection's link, directory, mode and
// out-of-home effects.
func (clientAdapter fixtureAdapter) projectExtras() error {
	switch clientAdapter.breaks {
	case listsItsLink, writesUnlistedLink:
		return os.Symlink(targetName, clientAdapter.path(linkName))
	case removesItsDirectory, keepsItsDirectory, withdrawalChangesItsMode, withdrawalChangesListed,
		withdrawalRollbackDrops, rollbackChangesItsMode:
		return os.Mkdir(clientAdapter.path(toolName), 0o700)
	case rollbackKeepsMode:
		return os.Chmod(clientAdapter.path(targetName), 0o644)
	case changesDirectoryMode:
		return os.Chmod(clientAdapter.path(existingName), 0o755)
	case writesProcessHome:
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(home, "fixture-state"), []byte("projected"), 0o600)
	case capturesProcessHome:
		return os.WriteFile(filepath.Join(clientAdapter.capturedHome, "fixture-state"), []byte("projected"), 0o600)
	case writesRelativePath:
		return os.WriteFile("fixture-state", []byte("projected"), 0o600)
	}
	return nil
}

func (clientAdapter fixtureAdapter) withdraw() (adapter.ProjectionReceipt, error) {
	if clientAdapter.breaks == withdrawalPlanWrites {
		return nil, nil
	}
	if err := clientAdapter.withdrawFiles(); err != nil {
		return nil, err
	}
	switch clientAdapter.breaks {
	case withdrawalReturnsNoReceipt:
		return nil, nil
	case withdrawalRollbackKeeps:
		return fixtureReceipt(func() error { return nil }), nil
	case withdrawalRollbackDrops, keepsItsDirectory:
		return fixtureReceipt(func() error { return clientAdapter.write(targetName, "projected") }), nil
	case withdrawalChangesItsMode, withdrawalChangesListed:
		return fixtureReceipt(func() error {
			if err := clientAdapter.write(targetName, "projected"); err != nil {
				return err
			}
			return os.Chmod(clientAdapter.path(toolName), 0o700)
		}), nil
	}
	return fixtureReceipt(func() error {
		_, err := clientAdapter.project()
		return err
	}), nil
}

// withdrawFiles restores the home as a withdrawal promises, or breaks that
// promise in the way that the fixture names.
func (clientAdapter fixtureAdapter) withdrawFiles() error {
	switch clientAdapter.breaks {
	case withdrawalKeepsCompanion, keepsItsDirectory:
		return clientAdapter.restoreTarget()
	case withdrawalChangesItsMode, withdrawalChangesListed:
		if err := clientAdapter.restoreTarget(); err != nil {
			return err
		}
		return os.Chmod(clientAdapter.path(toolName), 0o755)
	case withdrawalCreatesDirectory:
		if err := clientAdapter.restore(); err != nil {
			return err
		}
		return os.Mkdir(clientAdapter.path(".withdrawn"), 0o700)
	}
	return clientAdapter.restore()
}

// restore returns the target to its original bytes and removes the companion,
// the link and the projection's directory.
func (clientAdapter fixtureAdapter) restore() error {
	for _, name := range []string{companionName, linkName, toolName} {
		if err := os.Remove(clientAdapter.path(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return clientAdapter.restoreTarget()
}

func (clientAdapter fixtureAdapter) restoreTarget() error {
	return clientAdapter.write(targetName, "original")
}

type fixtureReceipt func() error

func (rollback fixtureReceipt) Rollback() error { return rollback() }

// newFixture places the adapter's target and an existing directory holding a
// user's file in a private home. Before binds only a foreign client, and After
// also enables the fixture client.
func newFixture(t *testing.T, breaks string) Fixture {
	t.Helper()
	clientAdapter := fixtureAdapter{home: t.TempDir(), outside: filepath.Join(t.TempDir(), "outside.txt"), breaks: breaks}
	if home, err := os.UserHomeDir(); err == nil {
		clientAdapter.capturedHome = home
	}
	if breaks == fixtureRedirectsHome {
		t.Setenv("HOME", t.TempDir())
	}
	if breaks == namesOutsideTarget {
		if err := os.WriteFile(clientAdapter.outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := clientAdapter.restoreTarget(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(clientAdapter.path(existingName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := clientAdapter.write(filepath.Join(existingName, "readme.txt"), "user"); err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig(testAdmission())
	before.SetClientActivation("foreign", true, "/fixture/foreign", nil)
	after := before.Clone()
	after.SetClientActivation(fixtureID, true, "/fixture/client", nil)
	return Fixture{Adapter: clientAdapter, Home: clientAdapter.home, Before: before, After: after}
}

// Each case keeps or breaks one seam promise. refused maps every invariant that
// must reject the case to a phrase of its reason; the other invariants pass.
// Windows file modes record only the read-only attribute, so the mode cases run
// on macOS and Linux.
func TestInvariantsRefuseEveryBrokenPromise(t *testing.T) {
	observation, plans, withdrawal := invariants[0].name, invariants[1].name, invariants[2].name
	unlisted := "which no plan lists"
	for _, test := range []struct {
		breaks    string
		posixOnly bool
		refused   map[string]string
	}{
		{breaks: keepsEveryPromise},
		{breaks: listsItsCompanion},
		{breaks: listsItsLink},
		{breaks: removesItsDirectory},
		{breaks: keepsItsDirectory},
		{breaks: writesUnlistedCompanion, refused: map[string]string{plans: unlisted, withdrawal: unlisted}},
		{breaks: writesOnlyCompanion, refused: map[string]string{plans: "states ChangesState true", withdrawal: unlisted}},
		{breaks: writesUnlistedLink, refused: map[string]string{plans: "Apply changed link, " + unlisted, withdrawal: "Apply changed link, " + unlisted}},
		{breaks: writesNothing, refused: map[string]string{plans: "states ChangesState true", withdrawal: "states ChangesState true"}},
		{breaks: statesNoChange, refused: map[string]string{plans: "Apply() of an enabled projection", withdrawal: "states ChangesState false"}},
		{breaks: listsOutsideHome, refused: map[string]string{plans: "lies outside the fixture home", withdrawal: "lies outside the fixture home"}},
		{breaks: namesMissingTarget, refused: map[string]string{plans: "was not projected"}},
		{breaks: namesUnlistedTarget, refused: map[string]string{plans: "is a file that the plan's effects do not list"}},
		{breaks: namesOutsideTarget, refused: map[string]string{plans: "the planned target lies outside the fixture home"}},
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
		{breaks: rollbackCreatesDirectory, refused: map[string]string{plans: "rollback changed [.backup]"}},
		{breaks: rollbackKeepsMode, posixOnly: true, refused: map[string]string{plans: "rollback changed [planned.txt]", withdrawal: "withdrawal changed [planned.txt]"}},
		{breaks: changesDirectoryMode, posixOnly: true, refused: map[string]string{plans: "Apply changed existing, " + unlisted, withdrawal: "withdrawal changed [existing]"}},
		{breaks: withdrawalKeepsCompanion, refused: map[string]string{withdrawal: "withdrawal changed [companion.txt]"}},
		{breaks: withdrawalCreatesDirectory, refused: map[string]string{withdrawal: "Apply changed .withdrawn, " + unlisted}},
		{breaks: withdrawalChangesItsMode, posixOnly: true, refused: map[string]string{withdrawal: "Apply changed .tool, " + unlisted}},
		{breaks: withdrawalChangesListed, posixOnly: true, refused: map[string]string{withdrawal: "withdrawal changed [.tool]"}},
		{breaks: rollbackChangesItsMode, posixOnly: true, refused: map[string]string{plans: "rollback changed [.tool]"}},
		{breaks: withdrawalRollbackDrops, refused: map[string]string{withdrawal: "rolling back a withdrawal changed [.tool]"}},
		{breaks: withdrawalRollbackKeeps, refused: map[string]string{withdrawal: "rolling back a withdrawal changed"}},
		{breaks: withdrawalReturnsNoReceipt, refused: map[string]string{withdrawal: "returned no receipt"}},
		{breaks: projectionReturnsNoReceipt, refused: map[string]string{plans: "Apply() of an enabled projection returned receipt <nil>"}},
	} {
		name := test.breaks
		if name == "" {
			name = "keeps every promise"
		}
		t.Run(name, func(t *testing.T) {
			if test.posixOnly && runtime.GOOS == "windows" {
				t.Skip("Windows file modes record only the read-only attribute")
			}
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

// decoyProcessHome points the process home, temporary and working directories
// at a test-owned directory, so that a test of the isolation writes there, not
// into the developer's files, if the isolation itself ever breaks.
func decoyProcessHome(t *testing.T) {
	t.Helper()
	decoy := t.TempDir()
	for _, name := range processHomeVariables {
		t.Setenv(name, decoy)
	}
	t.Chdir(decoy)
}

// Run isolates the process home before it prepares the fixture and runs the
// operations from the sentinel, so an adapter that writes through the process
// home, through a home it captured while it was constructed, or through a
// relative path is refused, as is a fixture that redirects the process home,
// while an honest one passes.
func TestRunRefusesWritesOutsideTheObservedHost(t *testing.T) {
	decoyProcessHome(t)
	for _, test := range []struct {
		breaks string
		want   string
	}{
		{breaks: keepsEveryPromise},
		{breaks: writesProcessHome, want: "instead of the fixture home"},
		{breaks: capturesProcessHome, want: "instead of the fixture home"},
		{breaks: writesRelativePath, want: "instead of the fixture home"},
		{breaks: fixtureRedirectsHome, want: "out of the process home sentinel"},
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
	decoyProcessHome(t)
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
