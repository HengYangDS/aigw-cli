package synchronization

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

func TestRetryPolicyRequiresExplicitConnectionChange(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero := uint64(0)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &zero}
	cfg.Clients[codex.ID] = binding
	cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	cfg.Models["other"] = domain.Model{Label: "Other", Provider: "other", Family: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Normalize()
	_, _, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, codex.ID, "other", "", domain.ConnectionChange{})
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("a different connection silently inherited policy: %v", err)
	}
	if cfg.Clients[codex.ID].Model != binding.Model {
		t.Fatal("refused selection changed its input")
	}
}

func TestRetryPolicySelectionPreservesOnlyTheSameConnection(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero, two := uint64(0), uint64(2)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &two}
	cfg.Clients[codex.ID] = binding
	model := cfg.Models["gpt"]
	model.Family, model.UpstreamModel = "other-model", "other-model"
	cfg.Models["model-only"] = model
	cfg.Families["other-model"] = domain.Family{Label: "Other model"}
	cfg.Normalize()
	forwarding := "http://127.0.0.1:8792/v1"
	for _, test := range []struct {
		name, route string
		change      domain.ConnectionChange
		want        *domain.RetryPolicy
	}{
		{"model preserves", "model-only", domain.ConnectionChange{}, binding.RetryPolicy},
		{"partial merges", "gpt", domain.ConnectionChange{RetryPolicy: &domain.RetryPolicy{RequestMaxRetries: &two}}, &domain.RetryPolicy{RequestMaxRetries: &two, StreamMaxRetries: &two}},
		{"native removes", "gpt", domain.ConnectionChange{NativeRetries: true}, nil},
		{"changed resets omitted", "gpt", domain.ConnectionChange{ForwardingEndpoint: &forwarding, RetryPolicy: &domain.RetryPolicy{RequestMaxRetries: &zero}}, &domain.RetryPolicy{RequestMaxRetries: &zero}},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := cfg.Clone()
			after, runtime, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, codex.ID, test.route, "", test.change)
			if err != nil || !reflect.DeepEqual(runtime.RetryPolicy, test.want) || !reflect.DeepEqual(after.Clients[codex.ID].RetryPolicy, test.want) {
				t.Fatalf("selection policy = %#v, want %#v: %v", runtime.RetryPolicy, test.want, err)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("selection mutated its input")
			}
		})
	}
}

func retryTransitionFixture(t *testing.T) domain.Config {
	t.Helper()
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Families["gpt"] = domain.Family{Label: "GPT"}
	cfg.Models["one"] = domain.Model{Label: "One", Provider: "one", Family: "gpt", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedModel(codex.ID, "one", domain.ProtocolOpenAIResponses)
	zero := uint64(0)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[codex.ID] = binding
	cfg.Normalize()
	return cfg
}

// A provider edit keeps a client's retry budgets: a new label changes no
// connection, and a new endpoint for the client's protocol moves the
// provider's upstream, to which the budgets belong.
func TestRetryConnectionTransitionGenericProviderCommit(t *testing.T) {
	for _, change := range []string{"label", "endpoint"} {
		t.Run(change, func(t *testing.T) {
			cfg := retryTransitionFixture(t)
			configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
			if err := configStore.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := configStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			after := before.Clone()
			account := after.Providers["one"]
			if change == "label" {
				account.Label = "Renamed"
			} else {
				account.Endpoints.OpenAIResponses = "https://replacement.test/v1"
			}
			after.Providers["one"] = account
			if err := (Synchronizer{Registry: builtinClients, Config: configStore}).Commit(t.Context(), before, after, "provider edit"); err != nil {
				t.Fatal(err)
			}
			observed, err := configStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.Clients[codex.ID].RetryPolicy, observed.Clients[codex.ID].RetryPolicy) {
				t.Fatalf("retry budgets = %#v, want %#v", observed.Clients[codex.ID].RetryPolicy, before.Clients[codex.ID].RetryPolicy)
			}
		})
	}
}

// A move to another provider is a new connection, which a retry policy still
// refuses without explicit intent, even when it keeps the protocol.
func TestRetryConnectionTransitionRefusesAnotherProvider(t *testing.T) {
	cfg := retryTransitionFixture(t)
	cfg.Providers["two"] = domain.Provider{Label: "Two", Endpoints: domain.Endpoints{OpenAIResponses: "https://two.test/v1"}}
	cfg.Models["two"] = domain.Model{Label: "Two", Provider: "two", Family: "gpt", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	after := cfg.Clone()
	binding := after.Clients[codex.ID]
	binding.Model = "two"
	after.Clients[codex.ID] = binding
	if err := validateRetryConnectionChange(cfg, after, codex.ID, domain.ConnectionChange{}); err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("a move to another provider kept its budgets: %v", err)
	}
}

func TestRetryConnectionTransitionProviderAddAdmission(t *testing.T) {
	cfg := retryTransitionFixture(t)
	acquired := false
	account := domain.Provider{Label: "Other", Endpoints: domain.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	route := domain.Model{Label: "Other", Family: "gpt", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	err := (Synchronizer{Registry: builtinClients}).ConnectProvider(t.Context(), cfg, "other", codex.ID, account, route, func() (string, error) { acquired = true; return "", fmt.Errorf("diagnostic token source reached") })
	if err == nil || acquired || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("new connection admission err=%v acquired=%v", err, acquired)
	}
}

func TestPrepareSelectionRefusesNativeAndExplicitRetriesTogether(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	before := cfg.Clone()
	zero := uint64(0)
	change := domain.ConnectionChange{NativeRetries: true, RetryPolicy: &domain.RetryPolicy{RequestMaxRetries: &zero}}
	if _, _, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, codex.ID, "gpt", "", change); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("contradictory retry intent = %v", err)
	}
	if !reflect.DeepEqual(cfg, before) {
		t.Fatal("refused selection mutated its input")
	}
}
