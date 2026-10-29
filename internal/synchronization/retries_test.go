package synchronization

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/configuration"
)

func TestRetryPolicyRequiresExplicitConnectionChange(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero := uint64(0)
	binding := cfg.Clients[configuration.ClientCodex]
	binding.RetryPolicy = &configuration.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &zero}
	cfg.Clients[configuration.ClientCodex] = binding
	cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	cfg.Routes["other"] = configuration.Route{Label: "Other", Account: "other", Model: "gpt-test", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.Normalize()
	_, _, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, configuration.ClientCodex, "other", "", configuration.ConnectionChange{})
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("a different connection silently inherited policy: %v", err)
	}
	if cfg.Clients[configuration.ClientCodex].Route != binding.Route {
		t.Fatal("refused selection changed its input")
	}
}

func TestRetryPolicySelectionPreservesOnlyTheSameConnection(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	zero, two := uint64(0), uint64(2)
	binding := cfg.Clients[configuration.ClientCodex]
	binding.RetryPolicy = &configuration.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &two}
	cfg.Clients[configuration.ClientCodex] = binding
	model := cfg.Routes["gpt"]
	model.Model, model.UpstreamModel = "other-model", "other-model"
	cfg.Routes["model-only"] = model
	cfg.Models["other-model"] = configuration.Model{Label: "Other model"}
	cfg.Normalize()
	forwarding := "http://127.0.0.1:8792/v1"
	for _, test := range []struct {
		name, route string
		change      configuration.ConnectionChange
		want        *configuration.RetryPolicy
	}{
		{"model preserves", "model-only", configuration.ConnectionChange{}, binding.RetryPolicy},
		{"partial merges", "gpt", configuration.ConnectionChange{RetryPolicy: &configuration.RetryPolicy{RequestMaxRetries: &two}}, &configuration.RetryPolicy{RequestMaxRetries: &two, StreamMaxRetries: &two}},
		{"native removes", "gpt", configuration.ConnectionChange{NativeRetries: true}, nil},
		{"changed resets omitted", "gpt", configuration.ConnectionChange{ForwardingEndpoint: &forwarding, RetryPolicy: &configuration.RetryPolicy{RequestMaxRetries: &zero}}, &configuration.RetryPolicy{RequestMaxRetries: &zero}},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := cfg.Clone()
			after, runtime, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, configuration.ClientCodex, test.route, "", test.change)
			if err != nil || !reflect.DeepEqual(runtime.RetryPolicy, test.want) || !reflect.DeepEqual(after.Clients[configuration.ClientCodex].RetryPolicy, test.want) {
				t.Fatalf("selection policy = %#v, want %#v: %v", runtime.RetryPolicy, test.want, err)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("selection mutated its input")
			}
		})
	}
}

func retryTransitionFixture(t *testing.T) configuration.Config {
	t.Helper()
	cfg := configuration.NewConfig(clients.Admission())
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Models["gpt"] = configuration.Model{Label: "GPT"}
	cfg.Routes["one"] = configuration.Route{Label: "One", Account: "one", Model: "gpt", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(configuration.ClientCodex, "one", configuration.ProtocolOpenAIResponses)
	zero := uint64(0)
	binding := cfg.Clients[configuration.ClientCodex]
	binding.RetryPolicy = &configuration.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[configuration.ClientCodex] = binding
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
			oldRuntime, _ := before.ResolveRuntime(configuration.ClientCodex, "")
			newRuntime, _ := observed.ResolveRuntime(configuration.ClientCodex, "")
			t.Logf("commit_error=%v endpoint=%s changed_connection=%v retained_budget=%v", err, newRuntime.Endpoint, oldRuntime.ConnectionFingerprint() != newRuntime.ConnectionFingerprint(), reflect.DeepEqual(before.Clients[configuration.ClientCodex].RetryPolicy, observed.Clients[configuration.ClientCodex].RetryPolicy))
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
	account := configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	route := configuration.Route{Label: "Other", Model: "gpt", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	err := (Synchronizer{Registry: builtinClients}).ConnectAccount(t.Context(), cfg, "other", configuration.ClientCodex, account, route, func() (string, error) { acquired = true; return "", fmt.Errorf("diagnostic token source reached") })
	if err == nil || acquired || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("new connection admission err=%v acquired=%v", err, acquired)
	}
}

func TestPrepareSelectionRefusesNativeAndExplicitRetriesTogether(t *testing.T) {
	cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
	before := cfg.Clone()
	zero := uint64(0)
	change := configuration.ConnectionChange{NativeRetries: true, RetryPolicy: &configuration.RetryPolicy{RequestMaxRetries: &zero}}
	if _, _, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, configuration.ClientCodex, "gpt", "", change); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("contradictory retry intent = %v", err)
	}
	if !reflect.DeepEqual(cfg, before) {
		t.Fatal("refused selection mutated its input")
	}
}
