package synchronization

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
	_, _, err := (Synchronizer{}).PrepareSelection(cfg, configuration.ClientCodex, "other", "", configuration.ConnectionChange{})
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
			after, runtime, err := (Synchronizer{}).PrepareSelection(cfg, configuration.ClientCodex, test.route, "", test.change)
			if err != nil || !reflect.DeepEqual(runtime.RetryPolicy, test.want) || !reflect.DeepEqual(after.Clients[configuration.ClientCodex].RetryPolicy, test.want) {
				t.Fatalf("selection policy = %#v, want %#v: %v", runtime.RetryPolicy, test.want, err)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("selection mutated its input")
			}
		})
	}
}
