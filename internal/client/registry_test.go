package client

import (
	"bytes"
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

type rollbackFunc func() error

func (rollback rollbackFunc) Rollback() error { return rollback() }

type failingProjectionAdapter struct {
	id          string
	events      *[]string
	applyErr    error
	rollbackErr error
	nilReceipt  bool
	onPlan      func()
	onApply     func()
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
	if adapter.onPlan != nil {
		adapter.onPlan()
	}
	return nil, nil
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

func (failingProjectionAdapter) Verify(context.Context, Dependencies, configuration.Config, configuration.Runtime) (Verification, error) {
	return Verification{}, nil
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
	if err := registry.Apply(t.Context(), Dependencies{}, cfg, cfg, "first"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"plan:first", "apply:first"}) {
		t.Fatalf("scoped projection touched another client: %v", events)
	}
	events = nil
	if _, err := registry.Plan(Dependencies{}, cfg, cfg, "unknown"); err == nil {
		t.Fatal("unknown planning client accepted")
	}
	if err := registry.Apply(t.Context(), Dependencies{}, cfg, cfg, "unknown"); err == nil {
		t.Fatal("unknown projection client accepted")
	}
	if len(events) != 0 {
		t.Fatalf("invalid selection touched clients: %v", events)
	}
}

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

func (adapter *recordingAdapter) Inspect(_ context.Context, _ Dependencies, _ configuration.Config, _ configuration.Runtime) Status {
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
	if err := registry.Apply(context.Background(), Dependencies{}, cfg, cfg); err != nil {
		t.Fatal(err)
	}
	if changed := registry.ChangedClients(configuration.NewConfig(), cfg); !reflect.DeepEqual(changed, []string{"future"}) {
		t.Fatalf("changed client scope = %v, want the admitted future client", changed)
	}
	if status := registry.Inspect(context.Background(), Dependencies{}, cfg, "future", configuration.Runtime{}); !status.Ready {
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
	want := []string{"discovery", "converge", "plan", "plan", "apply", "projection-change", "status", "verify", "withdraw"}
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
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec()}, second, first)
	if err != nil {
		t.Fatal(err)
	}

	err = registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
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

	err = registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
	if !errors.Is(err, applyFailure) || !errors.Is(err, firstRollbackFailure) || !errors.Is(err, secondRollbackFailure) {
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
			err = registry.Apply(ctx, Dependencies{}, configuration.NewConfig(), configuration.NewConfig())
			if !errors.Is(err, wantErr) || (conflict != nil && !errors.Is(err, conflict)) {
				t.Errorf("Apply() error = %v; want %v and any compensation conflict", err, wantErr)
			}
			if !reflect.DeepEqual(events, want) {
				t.Errorf("events = %v, want %v", events, want)
			}
		})
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
	if status := registry.Inspect(context.Background(), Dependencies{}, cfg, "unknown", configuration.Runtime{}); status.Ready || !strings.Contains(status.Issue, "no admitted operational adapter") {
		t.Fatalf("unknown inspection = %#v", status)
	}
	_, err = registry.Verify(context.Background(), Dependencies{}, cfg, "unknown", configuration.Runtime{})
	assertUnknown("verify", err)
	assertUnknown("withdraw", registry.Withdraw(&cfg, "unknown"))
}

func TestRegistryRollbackIgnoresAdaptersWithoutReceipts(t *testing.T) {
	events := []string{}
	failure := errors.New("third adapter failed")
	first := failingProjectionAdapter{id: "first", events: &events, nilReceipt: true}
	second := failingProjectionAdapter{id: "second", events: &events}
	third := failingProjectionAdapter{id: "third", events: &events, applyErr: failure}
	registry, err := NewRegistry([]configuration.ClientSpec{first.Spec(), second.Spec(), third.Spec()}, first, second, third)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configuration.NewConfig()); !errors.Is(err, failure) {
		t.Fatalf("Apply() error = %v", err)
	}
	want := []string{"plan:first", "plan:second", "plan:third", "apply:first", "apply:second", "apply:third", "rollback:second"}
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

func TestRegistryPreparesEveryClientBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	codexTarget := filepath.Join(dir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	originalCodex := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(codexTarget, originalCodex, 0o600); err != nil {
		t.Fatal(err)
	}
	claudeSettings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudeSettings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeSettings, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	before.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	before.Routes[configuration.ClientClaude] = "claude"
	before.Routes[configuration.ClientCodex] = "codex"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{codexTarget}}
	deps := Dependencies{
		Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID: "codex-home-default", Authority: "aigw", ConfigPath: codexTarget, Present: true, AutoManaged: true,
		}}}},
		ClaudeSettingsPath: claudeSettings,
		AIGWExecutable:     "/opt/aigw",
	}

	err := DefaultRegistry().Apply(t.Context(), deps, before, after)
	if err == nil || !strings.Contains(err.Error(), "parse Claude settings") {
		t.Fatalf("Apply() error = %v", err)
	}
	got, readErr := os.ReadFile(codexTarget)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, originalCodex) {
		t.Fatalf("Codex changed before Claude preparation completed:\n%s", got)
	}
	for _, path := range []string{codexTarget + ".aigw-state.json", filepath.Join(filepath.Dir(codexTarget), "models.json")} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("Codex artifact %s exists after preparation failure: %v", path, statErr)
		}
	}
}
