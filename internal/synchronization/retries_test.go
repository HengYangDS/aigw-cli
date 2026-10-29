package synchronization

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/domain"
)

func TestRetryPolicyRequiresExplicitConnectionChange(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero := uint64(0)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &zero}
	cfg.Clients[codex.ID] = binding
	cfg.Accounts["other"] = domain.Account{Label: "Other", Endpoints: domain.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	cfg.Routes["other"] = domain.Route{Label: "Other", Account: "other", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Normalize()
	_, _, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, codex.ID, "other", "", domain.ConnectionChange{})
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("a different connection silently inherited policy: %v", err)
	}
	if cfg.Clients[codex.ID].Route != binding.Route {
		t.Fatal("refused selection changed its input")
	}
}

func TestRetryPolicySelectionPreservesOnlyTheSameConnection(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero, two := uint64(0), uint64(2)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &two}
	cfg.Clients[codex.ID] = binding
	model := cfg.Routes["gpt"]
	model.Model, model.UpstreamModel = "other-model", "other-model"
	cfg.Routes["model-only"] = model
	cfg.Models["other-model"] = domain.Model{Label: "Other model"}
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
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Models["gpt"] = domain.Model{Label: "GPT"}
	cfg.Routes["one"] = domain.Route{Label: "One", Account: "one", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(codex.ID, "one", domain.ProtocolOpenAIResponses)
	zero := uint64(0)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[codex.ID] = binding
	cfg.Normalize()
	return cfg
}

func TestRetryConnectionTransitionGenericAccountCommit(t *testing.T) {
	for _, change := range []string{"label", "endpoint"} {
		t.Run(change, func(t *testing.T) {
			cfg := retryTransitionFixture(t)
			store := configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			after := before.Clone()
			account := after.Accounts["one"]
			if change == "label" {
				account.Label = "Renamed"
			} else {
				account.Endpoints.OpenAIResponses = "https://replacement.test/v1"
			}
			after.Accounts["one"] = account
			err = (Synchronizer{Registry: builtinClients, Config: store}).Commit(t.Context(), before, after, "account edit")
			if change == "label" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			observed, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			oldRuntime, _ := before.ResolveRuntime(codex.ID, "")
			newRuntime, _ := observed.ResolveRuntime(codex.ID, "")
			t.Logf("commit_error=%v endpoint=%s changed_connection=%v retained_budget=%v", err, newRuntime.Endpoint, oldRuntime.ConnectionFingerprint() != newRuntime.ConnectionFingerprint(), reflect.DeepEqual(before.Clients[codex.ID].RetryPolicy, observed.Clients[codex.ID].RetryPolicy))
			if err == nil || !strings.Contains(err.Error(), "retry") {
				t.Fatal("connection changed without explicit retry intent")
			}
			unchanged, snapshotErr := store.CaptureSnapshot()
			if snapshotErr != nil || !reflect.DeepEqual(snapshot, unchanged) {
				t.Fatalf("refusal changed configuration: %v", snapshotErr)
			}
		})
	}
}

func TestRetryConnectionTransitionAccountAddAdmission(t *testing.T) {
	cfg := retryTransitionFixture(t)
	acquired := false
	account := domain.Account{Label: "Other", Endpoints: domain.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	route := domain.Route{Label: "Other", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	err := (Synchronizer{Registry: builtinClients}).ConnectAccount(t.Context(), cfg, "other", codex.ID, account, route, func() (string, error) { acquired = true; return "", fmt.Errorf("diagnostic token source reached") })
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
