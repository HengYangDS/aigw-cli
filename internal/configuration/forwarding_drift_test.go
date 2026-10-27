package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const forwardingTestEndpoint = "http://127.0.0.1:8792/v1"

// forwardTo selects Codex's forwarding endpoint, or its direct connection when
// endpoint is empty, for a test fixture.
func forwardTo(t *testing.T, cfg *Config, endpoint string) {
	t.Helper()
	if err := cfg.SetForwardingEndpoint(ClientCodex, endpoint); err != nil {
		t.Fatal(err)
	}
}

func forwardedConfig(t *testing.T) Config {
	t.Helper()
	cfg := validConfig()
	forwardTo(t, &cfg, forwardingTestEndpoint)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the forwarded fixture is invalid: %v", err)
	}
	return cfg
}

func TestForwardingRefusesAChangeThatMovesItsUpstream(t *testing.T) {
	for name, move := range map[string]func(*Config){
		"Account endpoint": func(cfg *Config) {
			account := cfg.Accounts["backup"]
			account.Endpoints.OpenAIResponses = "https://moved.test/v1"
			cfg.Accounts["backup"] = account
		},
		"Account rename": func(cfg *Config) {
			cfg.Accounts["renamed"] = cfg.Accounts["backup"]
			delete(cfg.Accounts, "backup")
			route := cfg.Routes["backup"]
			route.Account = "renamed"
			cfg.Routes["backup"] = route
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := forwardedConfig(t)
			move(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "`aigw use --for codex backup --direct`") {
				t.Fatalf("a moved forwarded upstream was accepted or not explained: %v", err)
			}
			if runtime, err := cfg.ResolveRuntime(ClientCodex, ""); err != nil || runtime.Endpoint == forwardingTestEndpoint {
				t.Fatalf("forwarding followed the moved upstream: %#v, %v", runtime, err)
			}
		})
	}
}

func TestForwardingRefusesAProtocolChange(t *testing.T) {
	cfg := validConfig()
	cfg.Accounts["multi"] = Account{Label: "Multi", Endpoints: Endpoints{OpenAIResponses: "https://multi.test/v1", OpenAIChatCompletions: "https://multi.test/v1"}}
	cfg.Routes["multi"] = testRoute("Multi", "multi", "multi-model", ProtocolOpenAIResponses, ProtocolOpenAIChatCompletions)
	cfg.SetSelectedRoute(ClientHermes, "multi", ProtocolOpenAIResponses)
	if err := cfg.SetForwardingEndpoint(ClientHermes, forwardingTestEndpoint); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	// The same Account URL serves both protocols, so only the protocol moves.
	binding := cfg.Clients[ClientHermes]
	binding.Protocol = ProtocolOpenAIChatCompletions
	cfg.Clients[ClientHermes] = binding
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "`aigw use --for hermes multi --direct`") {
		t.Fatalf("a forwarded protocol change was accepted or not explained: %v", err)
	}
}

func TestForwardingRefusalResetsASelectedRetryPolicy(t *testing.T) {
	cfg := forwardedConfig(t)
	retries := uint64(2)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &retries}
	cfg.Clients[ClientCodex] = binding
	account := cfg.Accounts["backup"]
	account.Endpoints.OpenAIResponses = "https://moved.test/v1"
	cfg.Accounts["backup"] = account
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "`aigw use --for codex backup --direct --native-retries`") {
		t.Fatalf("the advised command would refuse its own connection change: %v", err)
	}
}

func TestForwardingSurvivesASelectionOfTheSameUpstream(t *testing.T) {
	cfg := forwardedConfig(t)
	cfg.Routes["other"] = testRoute("Other", "backup", "other-model", ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ClientCodex, "other", "")
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if runtime, err := cfg.ResolveRuntime(ClientCodex, ""); err != nil || runtime.Endpoint != forwardingTestEndpoint || runtime.Model != "other-model" {
		t.Fatalf("a selection of the same upstream lost forwarding: %#v, %v", runtime, err)
	}
}

func TestDirectSelectionDropsForwardingWithItsUpstream(t *testing.T) {
	cfg := forwardedConfig(t)
	if err := cfg.SetForwardingEndpoint(ClientCodex, ""); err != nil {
		t.Fatal(err)
	}
	if binding := cfg.Clients[ClientCodex]; binding.ForwardingEndpoint != "" || binding.ForwardingUpstreamIdentity != "" || binding.ForwardingProtocol != "" {
		t.Fatalf("the direct selection kept part of the forwarding: %#v", binding)
	}
	account := cfg.Accounts["backup"]
	account.Endpoints.OpenAIResponses = "https://moved.test/v1"
	cfg.Accounts["backup"] = account
	if err := cfg.Validate(); err != nil {
		t.Fatalf("an Account edit after the direct selection was refused: %v", err)
	}
}

func TestForwardedCheckpointKeepsTheShapeARetainedReaderDecodes(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	cfg := forwardedConfig(t)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(context.Background(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.Path() + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Config struct {
			Clients map[string]map[string]json.RawMessage `json:"clients"`
		} `json:"config"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	binding := document.Config.Clients[ClientCodex]
	if _, recorded := binding["forwarding_endpoint"]; !recorded || bytes.Contains(data, []byte("upstream_identity")) || bytes.Contains(data, []byte("forwarding_protocol")) {
		t.Fatalf("the checkpoint changed the binding shape that 0.4.17 decodes: %s", data)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	restored, expected := checkpoint.Config.Clients[ClientCodex], cfg.Clients[ClientCodex]
	if restored.ForwardingEndpoint != expected.ForwardingEndpoint || restored.ForwardingUpstreamIdentity != expected.ForwardingUpstreamIdentity || restored.ForwardingProtocol != expected.ForwardingProtocol {
		t.Fatalf("the restored checkpoint is not bound to its own upstream: %#v, want %#v", restored, expected)
	}
}
