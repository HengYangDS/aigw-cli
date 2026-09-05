package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

type failingProjectionAdapter struct {
	id          string
	events      *[]string
	applyErr    error
	rollbackErr error
	nilReceipt  bool
}

func (adapter failingProjectionAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: adapter.id, Label: adapter.id, EndpointProtocol: configuration.ProtocolOpenAIResponses}
}

func (adapter failingProjectionAdapter) Discover(DiscoverySource) discovery.Result {
	return discovery.Result{}
}

func (adapter failingProjectionAdapter) Converge(Dependencies, *configuration.Config, discovery.Result) error {
	return nil
}

func (adapter failingProjectionAdapter) Plan(Dependencies, configuration.Config, configuration.Config) ([]ProjectionPlan, error) {
	*adapter.events = append(*adapter.events, "plan:"+adapter.id)
	return nil, nil
}

func (adapter failingProjectionAdapter) Apply(context.Context, Dependencies, configuration.Config, configuration.Config) (ProjectionReceipt, error) {
	*adapter.events = append(*adapter.events, "apply:"+adapter.id)
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

func (failingProjectionAdapter) CredentialBindingChanged(configuration.Config, configuration.Config) bool {
	return false
}

func (failingProjectionAdapter) UsesCredentialAccount(configuration.Config, string) bool {
	return false
}

func (failingProjectionAdapter) BindCredential(context.Context, Dependencies, configuration.Config, []string) error {
	return nil
}

func (failingProjectionAdapter) Inspect(context.Context, Dependencies, configuration.Config, configuration.Runtime, InspectionOptions) Status {
	return Status{}
}

func (failingProjectionAdapter) Verify(context.Context, Dependencies, configuration.Config, configuration.Runtime) (Verification, error) {
	return Verification{}, nil
}

func (failingProjectionAdapter) Withdraw(*configuration.Config) {}

type recordingAdapter struct {
	calls []string
}

func (adapter *recordingAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: "future", Label: "Future", EndpointProtocol: configuration.ProtocolOpenAIResponses}
}

func (adapter *recordingAdapter) Discover(DiscoverySource) discovery.Result {
	adapter.calls = append(adapter.calls, "discovery")
	return discovery.Result{Executables: map[string]string{adapter.Spec().ID: "/future"}}
}

func (adapter *recordingAdapter) Converge(_ Dependencies, cfg *configuration.Config, _ discovery.Result) error {
	adapter.calls = append(adapter.calls, "converge")
	cfg.Adapters[adapter.Spec().ID] = configuration.AdapterConfig{Enabled: true}
	return nil
}

func (adapter *recordingAdapter) Plan(_ Dependencies, _, _ configuration.Config) ([]ProjectionPlan, error) {
	adapter.calls = append(adapter.calls, "plan")
	return []ProjectionPlan{{Client: adapter.Spec().ID, Target: "/future", Action: "project"}}, nil
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

func (adapter *recordingAdapter) CredentialBindingChanged(_, _ configuration.Config) bool {
	adapter.calls = append(adapter.calls, "credential-change")
	return true
}

func (adapter *recordingAdapter) UsesCredentialAccount(_ configuration.Config, accountID string) bool {
	adapter.calls = append(adapter.calls, "credential-account")
	return accountID == "future"
}

func (adapter *recordingAdapter) BindCredential(_ context.Context, _ Dependencies, _ configuration.Config, _ []string) error {
	adapter.calls = append(adapter.calls, "credential-bind")
	return nil
}

func (adapter *recordingAdapter) Inspect(_ context.Context, _ Dependencies, _ configuration.Config, _ configuration.Runtime, _ InspectionOptions) Status {
	adapter.calls = append(adapter.calls, "status")
	return Status{Ready: true}
}

func (adapter *recordingAdapter) Verify(_ context.Context, _ Dependencies, _ configuration.Config, _ configuration.Runtime) (Verification, error) {
	adapter.calls = append(adapter.calls, "verify")
	return Verification{Version: "verified"}, nil
}

func (adapter *recordingAdapter) Withdraw(cfg *configuration.Config) {
	adapter.calls = append(adapter.calls, "withdraw")
	delete(cfg.Adapters, adapter.Spec().ID)
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
	receipt, err := registry.Apply(context.Background(), Dependencies{}, cfg, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !registry.ProjectionChanged(configuration.NewConfig(), cfg) {
		t.Fatal("projection change was not delegated to the adapter")
	}
	if !registry.CredentialBindingChanged(configuration.NewConfig(), cfg) {
		t.Fatal("credential change was not delegated to the adapter")
	}
	if !registry.UsesCredentialAccount(cfg, "future") {
		t.Fatal("credential account use was not delegated to the adapter")
	}
	if err := registry.BindChangedCredentials(context.Background(), Dependencies{}, configuration.NewConfig(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := registry.BindCredentialsForAccount(context.Background(), Dependencies{}, cfg, "future"); err != nil {
		t.Fatal(err)
	}
	if err := registry.BindCredential(context.Background(), Dependencies{}, cfg, "future", []string{"/future"}); err != nil {
		t.Fatal(err)
	}
	if status := registry.Inspect(context.Background(), Dependencies{}, cfg, "future", configuration.Runtime{}, InspectionOptions{}); !status.Ready {
		t.Fatalf("status = %#v", status)
	}
	if result, err := registry.Verify(context.Background(), Dependencies{}, cfg, "future", configuration.Runtime{}); err != nil || result.Version != "verified" {
		t.Fatalf("verification = %#v, %v", result, err)
	}
	if err := registry.Withdraw(&cfg, "future"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Adapters["future"]; ok {
		t.Fatal("withdraw retained the adapter configuration")
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}

	want := []string{"discovery", "converge", "plan", "plan", "apply", "projection-change", "credential-change", "credential-account", "credential-change", "credential-bind", "credential-account", "credential-bind", "credential-bind", "status", "verify", "withdraw", "rollback"}
	if !reflect.DeepEqual(adapter.calls, want) {
		t.Fatalf("adapter calls = %#v, want %#v", adapter.calls, want)
	}
}

func TestFutureClientAdmissionPreservesBuiltInClientsAndProviderState(t *testing.T) {
	baselineClientIDs := DefaultRegistry().IDs()
	before := configuration.NewConfig()
	before.Accounts["direct"] = configuration.Account{
		Label: "Direct",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://direct.example.test",
			OpenAIResponses: "https://direct.example.test/v1",
		},
	}
	before.Accounts["gateway"] = configuration.Account{
		Label: "External gateway",
		Endpoints: configuration.Endpoints{
			Anthropic:       "http://127.0.0.1:9876",
			OpenAIResponses: "http://127.0.0.1:9876/v1",
		},
	}
	before.Profiles["claude"] = configuration.Profile{Account: "direct", Client: configuration.ClientClaude, Model: "claude-test"}
	before.Profiles["codex"] = configuration.Profile{Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	before.Routes[configuration.ClientClaude] = "claude"
	before.Routes[configuration.ClientCodex] = "codex"
	before.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/clients/claude"}
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/clients/codex", Targets: []string{"/clients/codex.toml"}}
	wantUnchanged := before.Clone()

	future := &recordingAdapter{}
	specs := append(configuration.AdmittedClientSpecs(), future.Spec())
	registry, err := NewRegistry(specs, codexAdapter{}, claudeAdapter{}, future)
	if err != nil {
		t.Fatal(err)
	}
	after, err := registry.Converge(Dependencies{}, before, discovery.Result{}, future.Spec().ID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Adapters[future.Spec().ID].Enabled {
		t.Fatal("future client was not admitted through its adapter")
	}
	delete(after.Adapters, future.Spec().ID)
	if !reflect.DeepEqual(after, wantUnchanged) {
		t.Fatalf("future client changed existing client or Provider state:\n got %#v\nwant %#v", after, wantUnchanged)
	}
	if !reflect.DeepEqual(before, wantUnchanged) {
		t.Fatal("future client admission mutated its input configuration")
	}
	if got := DefaultRegistry().IDs(); !reflect.DeepEqual(got, baselineClientIDs) {
		t.Fatalf("built-in registry changed: got %v, want %v", got, baselineClientIDs)
	}
}

func TestRegistryCompensatesAppliedAdaptersInReverseOrder(t *testing.T) {
	events := []string{}
	failure := errors.New("second adapter failed")
	first := failingProjectionAdapter{id: "first", events: &events}
	second := failingProjectionAdapter{id: "second", events: &events, applyErr: failure}
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, first, second)
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "prior adapters were rolled back") {
		t.Fatalf("Apply() error = %v", err)
	}
	want := []string{"plan:first", "plan:second", "apply:first", "apply:second", "rollback:first"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestRegistryReportsCompensationFailure(t *testing.T) {
	events := []string{}
	applyFailure := errors.New("second adapter failed")
	rollbackFailure := errors.New("first rollback failed")
	first := failingProjectionAdapter{id: "first", events: &events, rollbackErr: rollbackFailure}
	second := failingProjectionAdapter{id: "second", events: &events, applyErr: applyFailure}
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, first, second)
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
	if !errors.Is(err, applyFailure) || !strings.Contains(err.Error(), rollbackFailure.Error()) {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestRegistryRejectsInvalidAdmissionSets(t *testing.T) {
	tests := []struct {
		name     string
		specs    []configuration.ClientSpec
		adapters []Adapter
		want     string
	}{
		{name: "nil adapter", adapters: []Adapter{nil}, want: "adapter is nil"},
		{name: "empty adapter ID", adapters: []Adapter{failingProjectionAdapter{}}, want: "ID is empty"},
		{
			name: "duplicate adapter",
			adapters: []Adapter{
				failingProjectionAdapter{id: "duplicate"},
				failingProjectionAdapter{id: "duplicate"},
			},
			want: "registered more than once",
		},
		{
			name: "duplicate admission",
			specs: []configuration.ClientSpec{
				failingProjectionAdapter{id: "duplicate"}.Spec(),
				failingProjectionAdapter{id: "duplicate"}.Spec(),
			},
			adapters: []Adapter{
				failingProjectionAdapter{id: "duplicate"},
			},
			want: "declared more than once",
		},
		{name: "missing adapter", specs: []configuration.ClientSpec{{ID: "missing"}}, want: "has no operational adapter"},
		{
			name:     "mismatched admission",
			specs:    []configuration.ClientSpec{{ID: "client", Label: "Expected"}},
			adapters: []Adapter{failingProjectionAdapter{id: "client"}},
			want:     "does not match",
		},
		{
			name:     "unadmitted adapter",
			adapters: []Adapter{failingProjectionAdapter{id: "extra"}},
			want:     "unadmitted adapter",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRegistry(test.specs, test.adapters...); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, test.want)
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
	assertUnknown("bind", registry.BindCredential(context.Background(), Dependencies{}, cfg, "unknown", nil))
	if status := registry.Inspect(context.Background(), Dependencies{}, cfg, "unknown", configuration.Runtime{}, InspectionOptions{}); status.Ready || !strings.Contains(status.Issue, "no admitted operational adapter") {
		t.Fatalf("unknown inspection = %#v", status)
	}
	_, err = registry.Verify(context.Background(), Dependencies{}, cfg, "unknown", configuration.Runtime{})
	assertUnknown("verify", err)
	assertUnknown("withdraw", registry.Withdraw(&cfg, "unknown"))
}

func TestRegistryRollbackIgnoresAdaptersWithoutReceipts(t *testing.T) {
	events := []string{}
	first := failingProjectionAdapter{id: "first", events: &events, nilReceipt: true}
	second := failingProjectionAdapter{id: "second", events: &events}
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, first, second)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	want := []string{"plan:first", "plan:second", "apply:first", "apply:second", "rollback:second"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestDefaultRegistryConvergesConfiguredExecutablesConservatively(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	cfg.Profiles["claude"] = configuration.Profile{Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Profiles["codex"] = configuration.Profile{Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: missing}

	after, err := DefaultRegistry().Converge(Dependencies{}, cfg, discovery.Result{}, configuration.ClientClaude)
	if err != nil || after.Adapters[configuration.ClientClaude].Executable != missing {
		t.Fatalf("configured missing executable = %q, %v", after.Adapters[configuration.ClientClaude].Executable, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: loop}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: loop, Targets: []string{"/target"}}
	discovered := discovery.Result{Executables: map[string]string{
		configuration.ClientClaude: "/replacement/claude",
		configuration.ClientCodex:  "/replacement/codex",
	}}
	for _, clientID := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		if _, err := DefaultRegistry().Converge(Dependencies{}, cfg, discovered, clientID); err == nil || !strings.Contains(err.Error(), "inspect configured "+clientID+" executable") {
			t.Fatalf("Converge(%q) error = %v", clientID, err)
		}
	}
}
