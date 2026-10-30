package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/contract"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

const rehearsalID = "rehearsal"

// rehearsalAdapter is a test-only client that is never admitted. It shows that
// a client needs only its own adapter, its registration in the composition root
// and its fixtures. It projects its bound Route into one file of its home.
type rehearsalAdapter struct{ home string }

func (rehearsalAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{ID: rehearsalID, Label: "Rehearsal", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}}
}

func (clientAdapter rehearsalAdapter) target() string {
	return filepath.Join(clientAdapter.home, ".rehearsal", "route.json")
}

func (clientAdapter rehearsalAdapter) Discover(source adapter.DiscoverySource) discovery.Result {
	return discovery.Result{
		Executables: map[string]string{rehearsalID: source.Executable(rehearsalID)},
		Surfaces:    []discovery.Surface{{ID: "rehearsal-home", Product: "Rehearsal", Authority: "aigw", ConfigPath: clientAdapter.target(), Present: source.FilePresent(clientAdapter.target())}},
	}
}

func (rehearsalAdapter) Converge(adapter.Dependencies, *domain.Config, discovery.Result) error {
	return nil
}

// projection returns the bytes that cfg projects, or none without an enabled binding.
func (rehearsalAdapter) projection(cfg domain.Config) ([]byte, error) {
	binding := cfg.Clients[rehearsalID]
	if !binding.Enabled {
		return nil, nil
	}
	route, found := cfg.Routes[binding.Route]
	if !found {
		return nil, fmt.Errorf("rehearsal binding names unknown route %q", binding.Route)
	}
	return json.Marshal(map[string]string{"route": binding.Route, "model": route.Model})
}

// change compares the current projection with the one that cfg wants.
func (clientAdapter rehearsalAdapter) change(cfg domain.Config) (current platform.FileSnapshot, desired []byte, changes bool, err error) {
	if desired, err = clientAdapter.projection(cfg); err != nil {
		return platform.FileSnapshot{}, nil, false, err
	}
	if current, err = platform.CaptureFileSnapshot(clientAdapter.target()); err != nil {
		return platform.FileSnapshot{}, nil, false, err
	}
	return current, desired, current.Exists != (desired != nil) || desired != nil && !bytes.Equal(current.Data, desired), nil
}

func (clientAdapter rehearsalAdapter) Plan(_ adapter.Dependencies, before, after domain.Config) ([]adapter.ProjectionPlan, error) {
	if !before.Clients[rehearsalID].Enabled && !after.Clients[rehearsalID].Enabled {
		return nil, nil
	}
	_, desired, changes, err := clientAdapter.change(after)
	if err != nil {
		return nil, err
	}
	action := "unchanged"
	switch {
	case changes && desired == nil:
		action = "withdraw"
	case changes:
		action = "project"
	}
	return []adapter.ProjectionPlan{{Client: rehearsalID, Target: clientAdapter.target(), Action: action, Effects: []string{clientAdapter.target()}, ChangesState: changes}}, nil
}

type rehearsalReceipt struct {
	path                string
	preimage, postimage platform.FileSnapshot
}

func (receipt rehearsalReceipt) Rollback() error {
	return platform.RestoreFileAtomicIfPostimage(receipt.path, receipt.preimage, receipt.postimage)
}

func (clientAdapter rehearsalAdapter) Apply(_ context.Context, _ adapter.Dependencies, before, after domain.Config) (adapter.ProjectionReceipt, error) {
	if !before.Clients[rehearsalID].Enabled && !after.Clients[rehearsalID].Enabled {
		return nil, nil
	}
	current, desired, changes, err := clientAdapter.change(after)
	if err != nil || !changes {
		return nil, err
	}
	var written platform.FileSnapshot
	if desired == nil {
		written, err = platform.RemoveFileIfUnchanged(clientAdapter.target(), current)
	} else {
		written, err = platform.WriteFileAtomicIfUnchanged(clientAdapter.target(), current, desired, 0o600)
	}
	if err != nil {
		return nil, err
	}
	return rehearsalReceipt{path: clientAdapter.target(), preimage: current, postimage: written}, nil
}

func (clientAdapter rehearsalAdapter) ProjectionChanged(before, after domain.Config) bool {
	previous, previousErr := clientAdapter.projection(before)
	current, currentErr := clientAdapter.projection(after)
	return previousErr != nil || currentErr != nil || !bytes.Equal(previous, current)
}

func (clientAdapter rehearsalAdapter) Inspect(_ context.Context, _ adapter.Dependencies, cfg domain.Config, _ domain.Runtime) adapter.Status {
	if _, _, changes, err := clientAdapter.change(cfg); err != nil || changes {
		return adapter.Status{Issue: "Rehearsal projection differs from the selected Route", RepairAction: "aigw sync"}
	}
	return adapter.Status{Ready: true}
}

func (rehearsalAdapter) Verify(context.Context, adapter.Dependencies, domain.Config, domain.Runtime, string) (adapter.Verification, error) {
	return adapter.Verification{}, errors.New("rehearsal verification runs no client")
}

func (rehearsalAdapter) AdoptSelection(adapter.Dependencies, *domain.Config) (string, error) {
	return "", nil
}

func (rehearsalAdapter) Withdraw(cfg *domain.Config) {
	delete(cfg.Clients, rehearsalID)
}

// rehearsalConfiguration returns a configuration pair that the given admission
// governs, before and after the rehearsal client selects its Route.
func rehearsalConfiguration(admission domain.Admission) (domain.Config, domain.Config) {
	before := domain.NewConfig(admission)
	before.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["rehearsal"] = qualifiedRoute("", "gateway", "rehearsal-model", domain.ProtocolOpenAIResponses)
	after := before.Clone()
	after.Clients[rehearsalID] = domain.ClientBinding{Route: "rehearsal", Enabled: true}
	return before, after
}

func TestRehearsalAdapterKeepsTheSeamContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Fixture {
		home := t.TempDir()
		clientAdapter := rehearsalAdapter{home: home}
		before, after := rehearsalConfiguration(domain.NewAdmission(clientAdapter.Spec()))
		return contract.Fixture{Adapter: clientAdapter, Home: home, Before: before, After: after}
	})
}

// A client joins the built-in set through its registration in the composition
// root, and synchronizing it plans and changes only its own projection.
func TestRehearsalClientJoinsTheBuiltInSet(t *testing.T) {
	rehearsal := rehearsalAdapter{home: t.TempDir()}
	registry, err := adapter.NewRegistry(append(adapters(), rehearsal)...)
	if err != nil {
		t.Fatal(err)
	}
	if want := append(Registry().IDs(), rehearsalID); !slices.Equal(registry.IDs(), want) {
		t.Fatalf("registered clients = %q, want %q", registry.IDs(), want)
	}
	before, after := rehearsalConfiguration(registry.Admission())
	if !after.Admission().Admits(rehearsalID) {
		t.Fatal("the configuration does not admit the client that the registry serves")
	}
	if changed := registry.ChangedClients(before, after); !slices.Equal(changed, []string{rehearsalID}) {
		t.Fatalf("changed clients = %q, want only the rehearsal client", changed)
	}
	plans, err := registry.Plan(adapter.Dependencies{}, before, after, rehearsalID)
	if err != nil || len(plans) != 1 || plans[0].Target != rehearsal.target() || !plans[0].ChangesState {
		t.Fatalf("rehearsal plans = %#v, %v", plans, err)
	}
	if _, _, err := registry.Apply(t.Context(), adapter.Dependencies{}, before, after, rehearsalID); err != nil {
		t.Fatal(err)
	}
	if status := registry.Inspect(t.Context(), adapter.Dependencies{}, after, rehearsalID, domain.Runtime{}); !status.Ready {
		t.Fatalf("rehearsal status = %#v", status)
	}
}
