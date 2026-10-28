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

	"aigw-cli/internal/client"
	"aigw-cli/internal/client/contract"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/transaction"
)

const rehearsalID = "rehearsal"

// rehearsalAdapter is a test-only client that is never admitted. It shows that
// a client needs only its own adapter, its registration in the composition root
// and its fixtures. It projects its bound Route into one file of its home.
type rehearsalAdapter struct{ home string }

func (rehearsalAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: rehearsalID, Label: "Rehearsal", EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolOpenAIResponses}}
}

func (adapter rehearsalAdapter) target() string {
	return filepath.Join(adapter.home, ".rehearsal", "route.json")
}

func (adapter rehearsalAdapter) Discover(source client.DiscoverySource) discovery.Result {
	return discovery.Result{
		Executables: map[string]string{rehearsalID: source.Executable(rehearsalID)},
		Surfaces:    []discovery.Surface{{ID: "rehearsal-home", Product: "Rehearsal", Authority: "aigw", ConfigPath: adapter.target(), Present: source.FilePresent(adapter.target())}},
	}
}

func (rehearsalAdapter) Converge(client.Dependencies, *configuration.Config, discovery.Result) error {
	return nil
}

// projection returns the bytes that cfg projects, or none without an enabled binding.
func (rehearsalAdapter) projection(cfg configuration.Config) ([]byte, error) {
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
func (adapter rehearsalAdapter) change(cfg configuration.Config) (current transaction.FileSnapshot, desired []byte, changes bool, err error) {
	if desired, err = adapter.projection(cfg); err != nil {
		return transaction.FileSnapshot{}, nil, false, err
	}
	if current, err = transaction.CaptureFileSnapshot(adapter.target()); err != nil {
		return transaction.FileSnapshot{}, nil, false, err
	}
	return current, desired, current.Exists != (desired != nil) || desired != nil && !bytes.Equal(current.Data, desired), nil
}

func (adapter rehearsalAdapter) Plan(_ client.Dependencies, before, after configuration.Config) ([]client.ProjectionPlan, error) {
	if !before.Clients[rehearsalID].Enabled && !after.Clients[rehearsalID].Enabled {
		return nil, nil
	}
	_, desired, changes, err := adapter.change(after)
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
	return []client.ProjectionPlan{{Client: rehearsalID, Target: adapter.target(), Action: action, ChangesState: changes}}, nil
}

type rehearsalReceipt struct {
	path                string
	preimage, postimage transaction.FileSnapshot
}

func (receipt rehearsalReceipt) Rollback() error {
	return transaction.RestoreFileAtomicIfPostimage(receipt.path, receipt.preimage, receipt.postimage)
}

func (adapter rehearsalAdapter) Apply(_ context.Context, _ client.Dependencies, before, after configuration.Config) (client.ProjectionReceipt, error) {
	if !before.Clients[rehearsalID].Enabled && !after.Clients[rehearsalID].Enabled {
		return nil, nil
	}
	current, desired, changes, err := adapter.change(after)
	if err != nil || !changes {
		return nil, err
	}
	var written transaction.FileSnapshot
	if desired == nil {
		written, err = transaction.RemoveFileIfUnchanged(adapter.target(), current)
	} else {
		written, err = transaction.WriteFileAtomicIfUnchanged(adapter.target(), current, desired, 0o600)
	}
	if err != nil {
		return nil, err
	}
	return rehearsalReceipt{path: adapter.target(), preimage: current, postimage: written}, nil
}

func (adapter rehearsalAdapter) ProjectionChanged(before, after configuration.Config) bool {
	previous, previousErr := adapter.projection(before)
	current, currentErr := adapter.projection(after)
	return previousErr != nil || currentErr != nil || !bytes.Equal(previous, current)
}

func (adapter rehearsalAdapter) Inspect(_ context.Context, _ client.Dependencies, cfg configuration.Config, _ configuration.Runtime) client.Status {
	if _, _, changes, err := adapter.change(cfg); err != nil || changes {
		return client.Status{Issue: "Rehearsal projection differs from the selected Route", RepairAction: "aigw sync"}
	}
	return client.Status{Ready: true}
}

func (rehearsalAdapter) Verify(context.Context, client.Dependencies, configuration.Config, configuration.Runtime, string) (client.Verification, error) {
	return client.Verification{}, errors.New("rehearsal verification runs no client")
}

func (rehearsalAdapter) AdoptSelection(client.Dependencies, *configuration.Config) (string, error) {
	return "", nil
}

func (rehearsalAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, rehearsalID)
}

func rehearsalConfiguration() (configuration.Config, configuration.Config) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["rehearsal"] = qualifiedRoute("", "gateway", "rehearsal-model", configuration.ProtocolOpenAIResponses)
	after := before.Clone()
	after.Clients[rehearsalID] = configuration.ClientBinding{Route: "rehearsal", Enabled: true}
	return before, after
}

func TestRehearsalAdapterKeepsTheSeamContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Fixture {
		home := t.TempDir()
		before, after := rehearsalConfiguration()
		return contract.Fixture{Adapter: rehearsalAdapter{home: home}, Home: home, Before: before, After: after}
	})
}

// A client joins the built-in set through its registration in the composition
// root, and synchronizing it plans and changes only its own projection.
func TestRehearsalClientJoinsTheBuiltInSet(t *testing.T) {
	rehearsal := rehearsalAdapter{home: t.TempDir()}
	registry, err := client.NewRegistry(append(configuration.AdmittedClientSpecs(), rehearsal.Spec()), append(adapters(), rehearsal)...)
	if err != nil {
		t.Fatal(err)
	}
	if want := append(Registry().IDs(), rehearsalID); !slices.Equal(registry.IDs(), want) {
		t.Fatalf("registered clients = %q, want %q", registry.IDs(), want)
	}
	before, after := rehearsalConfiguration()
	if changed := registry.ChangedClients(before, after); !slices.Equal(changed, []string{rehearsalID}) {
		t.Fatalf("changed clients = %q, want only the rehearsal client", changed)
	}
	plans, err := registry.Plan(client.Dependencies{}, before, after, rehearsalID)
	if err != nil || len(plans) != 1 || plans[0].Target != rehearsal.target() || !plans[0].ChangesState {
		t.Fatalf("rehearsal plans = %#v, %v", plans, err)
	}
	if _, _, err := registry.Apply(t.Context(), client.Dependencies{}, before, after, rehearsalID); err != nil {
		t.Fatal(err)
	}
	if status := registry.Inspect(t.Context(), client.Dependencies{}, after, rehearsalID, configuration.Runtime{}); !status.Ready {
		t.Fatalf("rehearsal status = %#v", status)
	}
}
