package client

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

type rollbackFunc func() error

func (rollback rollbackFunc) Rollback() error { return rollback() }

type failingProjectionAdapter struct {
	id          string
	events      *[]string
	applyErr    error
	rollbackErr error
	nilReceipt  bool
	plans       []ProjectionPlan
	onPlan      func()
	onApply     func()
}

func (adapter failingProjectionAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: adapter.id, Label: adapter.id, EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolOpenAIResponses}}
}

func (adapter failingProjectionAdapter) Discover(DiscoverySource) discovery.Result {
	return discovery.Result{}
}

func (adapter failingProjectionAdapter) Converge(Dependencies, *configuration.Config, discovery.Result) error {
	return nil
}

func (adapter failingProjectionAdapter) Plan(Dependencies, configuration.Config, configuration.Config) ([]ProjectionPlan, error) {
	*adapter.events = append(*adapter.events, "plan:"+adapter.id)
	if adapter.onPlan != nil {
		adapter.onPlan()
	}
	return adapter.plans, nil
}

func (adapter failingProjectionAdapter) Apply(context.Context, Dependencies, configuration.Config, configuration.Config) (ProjectionReceipt, error) {
	*adapter.events = append(*adapter.events, "apply:"+adapter.id)
	if adapter.onApply != nil {
		adapter.onApply()
	}
	if adapter.applyErr != nil {
		return nil, adapter.applyErr
	}
	if adapter.nilReceipt {
		return nil, nil
	}
	return rollbackFunc(func() error {
		*adapter.events = append(*adapter.events, "rollback:"+adapter.id)
		return adapter.rollbackErr
	}), nil
}

func (failingProjectionAdapter) ProjectionChanged(configuration.Config, configuration.Config) bool {
	return false
}

func (failingProjectionAdapter) Inspect(context.Context, Dependencies, configuration.Config, configuration.Runtime) Status {
	return Status{}
}

func (failingProjectionAdapter) Verify(context.Context, Dependencies, configuration.Config, configuration.Runtime, string) (Verification, error) {
	return Verification{}, nil
}

func (failingProjectionAdapter) AdoptSelection(Dependencies, *configuration.Config) (string, error) {
	return "", nil
}

func (failingProjectionAdapter) Withdraw(*configuration.Config) {}

func TestRegistryScopesProjectionToTheRequestedClient(t *testing.T) {
	var events []string
	first := failingProjectionAdapter{id: "first", events: &events}
	second := failingProjectionAdapter{id: "second", events: &events, applyErr: errors.New("unselected client")}
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, first, second)
	if err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	if _, _, err := registry.Apply(t.Context(), Dependencies{}, cfg, cfg, "first"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"plan:first", "apply:first"}) {
		t.Fatalf("scoped projection touched another client: %v", events)
	}
}

type recordingAdapter struct {
	calls []string
}

func (adapter *recordingAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: "future", Label: "Future", EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolOpenAIResponses}}
}

func (adapter *recordingAdapter) Discover(DiscoverySource) discovery.Result {
	adapter.calls = append(adapter.calls, "discovery")
	return discovery.Result{Executables: map[string]string{adapter.Spec().ID: "/future"}}
}

func (adapter *recordingAdapter) Converge(_ Dependencies, cfg *configuration.Config, _ discovery.Result) error {
	adapter.calls = append(adapter.calls, "converge")
	cfg.SetClientActivation(adapter.Spec().ID, true, "", nil)
	return nil
}

func (adapter *recordingAdapter) Plan(_ Dependencies, _, _ configuration.Config) ([]ProjectionPlan, error) {
	adapter.calls = append(adapter.calls, "plan")
	return []ProjectionPlan{{Client: adapter.Spec().ID, Target: "/future", Action: "project", ChangesState: true}}, nil
}

func (adapter *recordingAdapter) Apply(_ context.Context, _ Dependencies, _, _ configuration.Config) (ProjectionReceipt, error) {
	adapter.calls = append(adapter.calls, "apply")
	return rollbackFunc(func() error {
		adapter.calls = append(adapter.calls, "rollback")
		return nil
	}), nil
}

func (adapter *recordingAdapter) ProjectionChanged(_, _ configuration.Config) bool {
	adapter.calls = append(adapter.calls, "projection-change")
	return true
}

func (adapter *recordingAdapter) Inspect(_ context.Context, _ Dependencies, _ configuration.Config, _ configuration.Runtime) Status {
	adapter.calls = append(adapter.calls, "status")
	return Status{Ready: true}
}

func (adapter *recordingAdapter) Verify(_ context.Context, _ Dependencies, _ configuration.Config, _ configuration.Runtime, _ string) (Verification, error) {
	adapter.calls = append(adapter.calls, "verify")
	return Verification{Version: "verified"}, nil
}

func (adapter *recordingAdapter) AdoptSelection(Dependencies, *configuration.Config) (string, error) {
	adapter.calls = append(adapter.calls, "adopt")
	return "", nil
}

func (adapter *recordingAdapter) Withdraw(cfg *configuration.Config) {
	adapter.calls = append(adapter.calls, "withdraw")
	delete(cfg.Clients, adapter.Spec().ID)
}

func TestRegistryCarriesOneAdapterThroughItsCompleteLifecycle(t *testing.T) {
	adapter := &recordingAdapter{}
	registry, err := NewRegistry([]configuration.ClientSpec{adapter.Spec()}, adapter)
	if err != nil {
		t.Fatal(err)
	}

	cfg := configuration.NewConfig()
	if discovered := registry.Discover(nil); discovered.Executable("future") != "/future" {
		t.Fatalf("discovered = %#v", discovered)
	}
	if _, err := registry.Converge(Dependencies{}, cfg, discovery.Result{}, "future"); err != nil {
		t.Fatal(err)
	}
	if plans, err := registry.Plan(Dependencies{}, cfg, cfg); err != nil || len(plans) != 1 || plans[0].Client != "future" {
		t.Fatalf("plans = %#v, %v", plans, err)
	}
	if _, _, err := registry.Apply(context.Background(), Dependencies{}, cfg, cfg); err != nil {
		t.Fatal(err)
	}
	if changed := registry.ChangedClients(configuration.NewConfig(), cfg); !reflect.DeepEqual(changed, []string{"future"}) {
		t.Fatalf("changed client scope = %v, want the admitted future client", changed)
	}
	if status := registry.Inspect(context.Background(), Dependencies{}, cfg, "future", configuration.Runtime{}); !status.Ready {
		t.Fatalf("status = %#v", status)
	}
	if result, err := registry.Verify(context.Background(), Dependencies{}, cfg, "future", configuration.Runtime{}, ""); err != nil || result.Version != "verified" {
		t.Fatalf("verification = %#v, %v", result, err)
	}
	if err := registry.Withdraw(&cfg, "future"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Clients["future"]; ok {
		t.Fatal("withdraw retained the adapter configuration")
	}
	want := []string{"discovery", "converge", "plan", "plan", "apply", "projection-change", "status", "verify", "withdraw"}
	if !reflect.DeepEqual(adapter.calls, want) {
		t.Fatalf("adapter calls = %#v, want %#v", adapter.calls, want)
	}
}

func TestRegistryCompensatesAppliedAdaptersInReverseOrder(t *testing.T) {
	for _, nilReceipt := range []bool{false, true} {
		t.Run(fmt.Sprintf("nil-receipt=%t", nilReceipt), func(t *testing.T) {
			var events []string
			failure := errors.New("third adapter failed")
			first := failingProjectionAdapter{id: "first", events: &events, nilReceipt: nilReceipt}
			second := failingProjectionAdapter{id: "second", events: &events}
			third := failingProjectionAdapter{id: "third", events: &events, applyErr: failure}
			registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec(), third.Spec()}, third, second, first)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = registry.Apply(t.Context(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), "prior adapters were rolled back") {
				t.Fatalf("Apply() error = %v", err)
			}
			want := []string{"plan:first", "plan:second", "plan:third", "apply:first", "apply:second", "apply:third", "rollback:second"}
			if !nilReceipt {
				want = append(want, "rollback:first")
			}
			if !reflect.DeepEqual(events, want) {
				t.Fatalf("events = %#v, want %#v", events, want)
			}
		})
	}
}

func TestRegistryReturnsOneRollbackForSuccessfulProjection(t *testing.T) {
	for _, state := range []string{"converged", "changed-target", "changed-config"} {
		t.Run(state, func(t *testing.T) {
			var events []string
			first := failingProjectionAdapter{id: "first", events: &events, plans: []ProjectionPlan{{Client: "first", ChangesState: state == "changed-target"}}}
			second := failingProjectionAdapter{id: "second", events: &events}
			registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, first, second)
			if err != nil {
				t.Fatal(err)
			}
			before, after := configuration.NewConfig(), configuration.NewConfig()
			if state == "changed-config" {
				after.Accounts["new"] = configuration.Account{Label: "New"}
			}
			receipt, changed, err := registry.Apply(t.Context(), Dependencies{}, before, after)
			if err != nil || receipt == nil {
				t.Fatalf("successful projection receipt = %v, %v", receipt, err)
			}
			if changed != (state == "changed-target") {
				t.Fatalf("native projection changed=%t for %s", changed, state)
			}
			want := []string{"plan:first", "plan:second"}
			if state != "converged" {
				want = append(want, "apply:first", "apply:second", "rollback:second", "rollback:first")
			}
			if err := receipt.Rollback(); err != nil || !reflect.DeepEqual(events, want) {
				t.Fatalf("rollback = %v, events = %v; want %v", err, events, want)
			}
		})
	}
}

func TestRegistryReportsCompensationFailure(t *testing.T) {
	events := []string{}
	applyFailure := errors.New("third adapter failed")
	firstRollbackFailure := errors.New("first rollback failed")
	secondRollbackFailure := errors.New("second rollback failed")
	first := failingProjectionAdapter{id: "first", events: &events, rollbackErr: firstRollbackFailure}
	second := failingProjectionAdapter{id: "second", events: &events, rollbackErr: secondRollbackFailure}
	third := failingProjectionAdapter{id: "third", events: &events, applyErr: applyFailure}
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec(), third.Spec()}, first, second, third)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
	if !errors.Is(err, applyFailure) || !errors.Is(err, firstRollbackFailure) || !errors.Is(err, secondRollbackFailure) || !errors.Is(err, ErrProjectionRollbackFailed) {
		t.Fatalf("Apply() error = %v", err)
	}
	want := []string{"plan:first", "plan:second", "plan:third", "apply:first", "apply:second", "apply:third", "rollback:second", "rollback:first"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestRegistryCancellationStopsNewWritesAndCompensatesPriorAdapters(t *testing.T) {
	for _, phase := range []string{"before planning", "during planning", "between adapters", "compensation conflict", "last adapter"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var events []string
			first := failingProjectionAdapter{id: "first", events: &events}
			second := failingProjectionAdapter{id: "second", events: &events}
			var conflict error
			var want []string
			wantErr := context.Canceled
			switch phase {
			case "before planning":
				cancel()
			case "during planning":
				second.onPlan = cancel
				want = []string{"plan:first", "plan:second"}
			case "last adapter":
				second.onApply = cancel
				want = []string{"plan:first", "plan:second", "apply:first", "apply:second"}
				wantErr = nil
			default:
				first.onApply = cancel
				want = []string{"plan:first", "plan:second", "apply:first", "rollback:first"}
				if phase == "compensation conflict" {
					conflict = errors.New("projection changed after apply")
					first.rollbackErr = conflict
				}
			}
			registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, first, second)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = registry.Apply(ctx, Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
			if !errors.Is(err, wantErr) || (conflict != nil && !errors.Is(err, conflict)) {
				t.Errorf("Apply() error = %v; want %v and any compensation conflict", err, wantErr)
			}
			if !reflect.DeepEqual(events, want) {
				t.Errorf("events = %v, want %v", events, want)
			}
		})
	}
}

func TestRegistryRejectsUnknownClientOperations(t *testing.T) {
	adapter := &recordingAdapter{}
	registry, err := NewRegistry([]configuration.ClientSpec{adapter.Spec()}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	assertUnknown := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "no admitted operational adapter") {
			t.Fatalf("%s error = %v", name, err)
		}
	}
	_, err = registry.Converge(Dependencies{}, cfg, discovery.Result{}, "unknown")
	assertUnknown("converge", err)
	_, err = registry.Plan(Dependencies{}, cfg, cfg, "unknown")
	assertUnknown("plan", err)
	_, _, err = registry.Apply(t.Context(), Dependencies{}, cfg, cfg, "unknown")
	assertUnknown("apply", err)
	if status := registry.Inspect(context.Background(), Dependencies{}, cfg, "unknown", configuration.Runtime{}); status.Ready || !strings.Contains(status.Issue, "no admitted operational adapter") {
		t.Fatalf("unknown inspection = %#v", status)
	}
	_, err = registry.Verify(context.Background(), Dependencies{}, cfg, "unknown", configuration.Runtime{}, "")
	assertUnknown("verify", err)
	assertUnknown("withdraw", registry.Withdraw(&cfg, "unknown"))
	if len(adapter.calls) != 0 {
		t.Fatalf("unknown client operations touched an adapter: %v", adapter.calls)
	}
}

// hostValidatingAdapter refuses every host with its error, or none without one.
type hostValidatingAdapter struct {
	failingProjectionAdapter
	err error
}

func (adapter hostValidatingAdapter) ValidateHost(discovery.Host) error { return adapter.err }

func TestRegistryRefusesAHostWithTheFirstRefusalInAdmissionOrder(t *testing.T) {
	events := []string{}
	plain := failingProjectionAdapter{id: "plain", events: &events}
	for _, test := range []struct {
		name          string
		first, second error
		want          string
	}{
		{name: "every adapter derives its native paths"},
		{name: "the later adapter refuses", second: errors.New("SECOND_HOME is not set"), want: "SECOND_HOME is not set"},
		{name: "both adapters refuse", first: errors.New("FIRST_HOME is not set"), second: errors.New("SECOND_HOME is not set"), want: "FIRST_HOME is not set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			first := hostValidatingAdapter{failingProjectionAdapter{id: "first", events: &events}, test.first}
			second := hostValidatingAdapter{failingProjectionAdapter{id: "second", events: &events}, test.second}
			registry, err := NewRegistry([]configuration.ClientSpec{plain.Spec(), first.Spec(), second.Spec()}, plain, first, second)
			if err != nil {
				t.Fatal(err)
			}
			err = registry.ValidateHost(discovery.Host{GOOS: "windows"})
			if (err == nil) != (test.want == "") || err != nil && err.Error() != test.want {
				t.Fatalf("ValidateHost() = %v, want %q", err, test.want)
			}
		})
	}
	if len(events) != 0 {
		t.Fatalf("host validation called other adapter methods: %v", events)
	}
}
