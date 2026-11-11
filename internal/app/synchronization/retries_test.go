package synchronization

import (
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// Retry budgets belong to the client's provider: selecting another model of
// the provider keeps them and its forwarding, and selecting another
// provider's model clears both, without a refusal.
func TestSelectionKeepsRetryBudgetsWithinTheProvider(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero, two := uint64(0), uint64(2)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &two}
	cfg.Clients[codex.ID] = binding
	if err := cfg.SetForwardingEndpoint(codex.ID, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	model := cfg.Models["gpt"]
	model.Family, model.UpstreamModel = "other-model", "other-model"
	cfg.Models["model-only"] = model
	cfg.Families["other-model"] = domain.Family{Label: "Other model"}
	cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{Responses: "https://other.test/v1"}}
	cfg.Models["other"] = domain.Model{Label: "Other", Provider: "other", Family: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.Normalize()
	for _, test := range []struct {
		name, route string
		kept        bool
	}{
		{"another model of the provider", "model-only", true},
		{"another provider", "other", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := cfg.Clone()
			after, runtime, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, codex.ID, test.route, "")
			if err != nil {
				t.Fatal(err)
			}
			selected := after.Clients[codex.ID]
			if kept := selected.RetryPolicy != nil && reflect.DeepEqual(runtime.RetryPolicy, binding.RetryPolicy); kept != test.kept {
				t.Fatalf("retry budgets = %#v, want them kept: %t", selected.RetryPolicy, test.kept)
			}
			if forwarded := selected.ForwardingEndpoint != ""; forwarded != test.kept {
				t.Fatalf("forwarding = %q, want it kept: %t", selected.ForwardingEndpoint, test.kept)
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
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Responses: "https://one.test/v1"}}
	cfg.Families["gpt"] = domain.Family{Label: "GPT"}
	cfg.Models["one"] = domain.Model{Label: "One", Provider: "one", Family: "gpt", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.SetSelectedModel(codex.ID, "one", domain.ProtocolResponses)
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
				account.Endpoints.Responses = "https://replacement.test/v1"
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
