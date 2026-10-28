package contract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

const fixtureID = "fixture"

// fixtureAdapter projects one target file and, optionally, a companion file.
// Its flags break one seam promise at a time: writesTarget and writesCompanion
// choose the files that Apply changes, listsCompanion adds the companion to the
// plan's effects, and mutates names the read-only operation that changes the
// caller's configuration.
type fixtureAdapter struct {
	target, companion             string
	writesTarget, writesCompanion bool
	listsCompanion                bool
	mutates                       string
}

func (fixtureAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: fixtureID, Label: "Fixture"}
}

func (fixtureAdapter) Discover(client.DiscoverySource) discovery.Result { return discovery.Result{} }

func (fixtureAdapter) Converge(client.Dependencies, *configuration.Config, discovery.Result) error {
	return nil
}

// mutate changes another client's binding through the caller's configuration
// when operation is the one that the fixture breaks.
func (adapter fixtureAdapter) mutate(operation string, cfg configuration.Config) {
	if adapter.mutates == operation {
		binding := cfg.Clients["foreign"]
		binding.Executable += "/mutated"
		cfg.Clients["foreign"] = binding
	}
}

func (adapter fixtureAdapter) Inspect(_ context.Context, _ client.Dependencies, cfg configuration.Config, _ configuration.Runtime) client.Status {
	adapter.mutate("Inspect", cfg)
	return client.Status{Ready: true}
}

func (adapter fixtureAdapter) Verify(_ context.Context, _ client.Dependencies, cfg configuration.Config, _ configuration.Runtime, _ string) (client.Verification, error) {
	adapter.mutate("Verify", cfg)
	return client.Verification{Scope: fixtureID}, nil
}

func (fixtureAdapter) AdoptSelection(client.Dependencies, *configuration.Config) (string, error) {
	return "", nil
}

func (fixtureAdapter) ProjectionChanged(before, after configuration.Config) bool {
	return before.Clients[fixtureID].Enabled != after.Clients[fixtureID].Enabled
}

func (adapter fixtureAdapter) Plan(_ client.Dependencies, before, after configuration.Config) ([]client.ProjectionPlan, error) {
	adapter.mutate("Plan", after)
	effects := []string{adapter.target}
	if adapter.listsCompanion {
		effects = append(effects, adapter.companion)
	}
	return []client.ProjectionPlan{{Client: fixtureID, Target: adapter.target, Action: "project", Effects: effects, ChangesState: adapter.ProjectionChanged(before, after)}}, nil
}

func (adapter fixtureAdapter) Apply(_ context.Context, _ client.Dependencies, before, after configuration.Config) (client.ProjectionReceipt, error) {
	switch {
	case !adapter.ProjectionChanged(before, after):
		return nil, nil
	case !after.Clients[fixtureID].Enabled:
		return nil, adapter.restore()
	}
	for path, writes := range map[string]bool{adapter.target: adapter.writesTarget, adapter.companion: adapter.writesCompanion} {
		if writes {
			if err := os.WriteFile(path, []byte("projected"), 0o600); err != nil {
				return nil, err
			}
		}
	}
	return fixtureReceipt{adapter}, nil
}

// restore returns the target to its original bytes and removes the companion.
func (adapter fixtureAdapter) restore() error {
	if err := os.Remove(adapter.companion); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(adapter.target, []byte("original"), 0o600)
}

func (fixtureAdapter) Withdraw(cfg *configuration.Config) { delete(cfg.Clients, fixtureID) }

type fixtureReceipt struct{ adapter fixtureAdapter }

func (receipt fixtureReceipt) Rollback() error { return receipt.adapter.restore() }

// newFixture places the adapter's files in a private home. Before binds only a
// foreign client, and After also enables the fixture client.
func newFixture(t *testing.T, adapter fixtureAdapter) Fixture {
	t.Helper()
	home := t.TempDir()
	adapter.target, adapter.companion = filepath.Join(home, "planned.txt"), filepath.Join(home, "companion.txt")
	if err := os.WriteFile(adapter.target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := configuration.NewConfig()
	before.SetClientActivation("foreign", true, "/fixture/foreign", nil)
	after := before.Clone()
	after.SetClientActivation(fixtureID, true, "/fixture/client", nil)
	return Fixture{Adapter: adapter, Home: home, Before: before, After: after}
}

// Each case keeps or breaks one seam promise. refused maps every invariant that
// must reject the case to a phrase of its reason; the other invariants pass.
func TestInvariantsRefuseEveryBrokenPromise(t *testing.T) {
	observation, plans, withdrawal := invariants[0].name, invariants[1].name, invariants[2].name
	for _, test := range []struct {
		name    string
		adapter fixtureAdapter
		refused map[string]string
	}{
		{name: "an honest projection", adapter: fixtureAdapter{writesTarget: true}},
		{name: "an honest projection with a listed companion", adapter: fixtureAdapter{writesTarget: true, writesCompanion: true, listsCompanion: true}},
		{
			name:    "an unlisted companion beside the target",
			adapter: fixtureAdapter{writesTarget: true, writesCompanion: true},
			refused: map[string]string{plans: "which no plan lists", withdrawal: "which no plan lists"},
		},
		{
			name:    "an unlisted companion instead of the target",
			adapter: fixtureAdapter{writesCompanion: true},
			refused: map[string]string{plans: "states ChangesState true", withdrawal: "which no plan lists"},
		},
		{
			name:    "a promised change that never happens",
			adapter: fixtureAdapter{},
			refused: map[string]string{plans: "states ChangesState true", withdrawal: "states ChangesState true"},
		},
		{
			name:    "Inspect changes the caller's configuration",
			adapter: fixtureAdapter{writesTarget: true, mutates: "Inspect"},
			refused: map[string]string{observation: "Inspect changed the caller's configuration"},
		},
		{
			name:    "Verify changes the caller's configuration",
			adapter: fixtureAdapter{writesTarget: true, mutates: "Verify"},
			refused: map[string]string{observation: "Verify changed the caller's configuration"},
		},
		{
			name:    "Plan changes the caller's configuration",
			adapter: fixtureAdapter{writesTarget: true, mutates: "Plan"},
			refused: map[string]string{plans: "Plan changed the caller's configuration", withdrawal: "Plan changed the caller's configuration"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, invariant := range invariants {
				err := invariant.check(t.Context(), newFixture(t, test.adapter))
				want, refused := test.refused[invariant.name]
				if refused != (err != nil) || refused && !strings.Contains(err.Error(), want) {
					t.Errorf("%s: %v, want refusal %q", invariant.name, err, want)
				}
			}
		})
	}
}
