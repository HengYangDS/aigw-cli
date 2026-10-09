package configuration

import (
	"bytes"
	"reflect"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestCodexForwardingRetainsSameUpstreamModelSelection(t *testing.T) {
	for _, selection := range []string{"current-route-new-model", "same-upstream-new-route"} {
		t.Run(selection, func(t *testing.T) {
			cfg := validConfig()
			cfg.Normalize()
			cfg.Clients[ClientHermes] = ClientBinding{Route: "backup", Protocol: ProtocolOpenAIResponses}
			direct, err := cfg.ResolveRuntime(ClientCodex, "")
			if err != nil {
				t.Fatal(err)
			}
			before := cfg.Clone()
			if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/ucloud/v1"); err != nil {
				t.Fatal(err)
			}
			recorded := cfg.Clients[ClientCodex]
			route := cfg.Routes["backup"]
			route.Model = "gpt-next"
			route.UpstreamModel = "gpt-next"
			cfg.Models["gpt-next"] = Model{Label: "GPT next"}
			if selection == "same-upstream-new-route" {
				cfg.Routes["next"] = route
				cfg.SetSelectedRoute(ClientCodex, "next")
			} else {
				cfg.Routes["backup"] = route
				cfg.SetSelectedRoute(ClientCodex, "backup")
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("same-upstream model selection rejected: %v", err)
			}
			binding := cfg.Clients[ClientCodex]
			if binding.ForwardingEndpoint != recorded.ForwardingEndpoint ||
				binding.ForwardingUpstreamIdentity != recorded.ForwardingUpstreamIdentity ||
				binding.ForwardingProtocol != recorded.ForwardingProtocol {
				t.Fatalf("same-upstream selection lost its recorded destination: %+v", binding)
			}
			got, err := cfg.ResolveRuntime(ClientCodex, "")
			if err != nil {
				t.Fatal(err)
			}
			if got.Model != "gpt-next" || got.Endpoint != recorded.ForwardingEndpoint || got.UpstreamEndpoint != direct.Endpoint ||
				got.CredentialProjectionFingerprint(ClientCodex) != direct.CredentialProjectionFingerprint(ClientCodex) {
				t.Fatalf("forwarding changed upstream credential identity: %+v", got)
			}
			hermes, err := cfg.ResolveRuntime(ClientHermes, "")
			if err != nil {
				t.Fatal(err)
			}
			if hermes.Endpoint != direct.Endpoint || !reflect.DeepEqual(cfg.Accounts, before.Accounts) ||
				!reflect.DeepEqual(cfg.Clients[ClientHermes], before.Clients[ClientHermes]) {
				t.Fatal("Codex forwarding changed Accounts or Hermes")
			}
		})
	}
}

func TestCodexForwardingWithdrawsChangedUpstreamSelection(t *testing.T) {
	for _, change := range []string{"different-account-same-endpoint", "same-account-changed-endpoint"} {
		t.Run(change, func(t *testing.T) {
			cfg := validConfig()
			endpoint := cfg.Accounts["backup"].Endpoints.OpenAIResponses
			if change == "different-account-same-endpoint" {
				cfg.Accounts["next"] = Account{Label: "Next", Endpoints: Endpoints{OpenAIResponses: endpoint}}
				cfg.Routes["next"] = testRoute("Next", "next", "gpt-next", ProtocolOpenAIResponses)
			} else {
				cfg.Routes["next"] = testRoute("Next", "backup", "gpt-next", ProtocolOpenAIResponses)
			}
			cfg.Normalize()
			if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/ucloud/v1"); err != nil {
				t.Fatal(err)
			}
			if change == "same-account-changed-endpoint" {
				account := cfg.Accounts["backup"]
				endpoint = "https://replacement.test/v1"
				account.Endpoints.OpenAIResponses = endpoint
				cfg.Accounts["backup"] = account
			}
			cfg.SetSelectedRoute(ClientCodex, "next")
			binding := cfg.Clients[ClientCodex]
			if binding.ForwardingEndpoint != "" || binding.ForwardingUpstreamIdentity != "" || binding.ForwardingProtocol != "" {
				t.Fatalf("changed upstream retained a stale forwarding binding: %+v", binding)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
			runtime, err := cfg.ResolveRuntime(ClientCodex, "")
			if err != nil || runtime.Endpoint != endpoint {
				t.Fatalf("changed upstream was not resolved directly: %+v, %v", runtime, err)
			}
			sidecar, err := encodeForwarding(cfg)
			if err != nil || len(sidecar) != 0 {
				t.Fatalf("withdrawn forwarding still persists: %q, %v", sidecar, err)
			}
		})
	}
}

func TestCodexForwardingRejectsUpstreamDrift(t *testing.T) {
	for _, change := range []string{"account-endpoint", "recorded-protocol", "binding-protocol"} {
		t.Run(change, func(t *testing.T) {
			cfg := validConfig()
			cfg.Normalize()
			binding := cfg.Clients[ClientCodex]
			binding.Protocol = ProtocolOpenAIResponses
			cfg.Clients[ClientCodex] = binding
			if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/ucloud/v1"); err != nil {
				t.Fatal(err)
			}
			recorded := cfg.Clients[ClientCodex]
			sidecar, err := encodeForwarding(cfg)
			if err != nil {
				t.Fatal(err)
			}
			changed := cfg.Clone()
			switch change {
			case "account-endpoint":
				account := changed.Accounts["backup"]
				account.Endpoints.OpenAIResponses = "https://replacement.test/v1"
				changed.Accounts["backup"] = account
			case "recorded-protocol":
				binding := changed.Clients[ClientCodex]
				binding.ForwardingProtocol = ProtocolOpenAIChatCompletions
				changed.Clients[ClientCodex] = binding
			default:
				binding := changed.Clients[ClientCodex]
				binding.Protocol = ProtocolOpenAIChatCompletions
				changed.Clients[ClientCodex] = binding
			}
			beforeRefusal := changed.Clone()
			if err := changed.Validate(); err == nil {
				t.Fatal("direct upstream edit silently rebound the forwarding destination")
			}
			if _, err := encodeConfig(changed); err == nil {
				t.Fatal("configuration encoding accepted a stale upstream binding")
			}
			if _, err := encodeForwarding(changed); err == nil {
				t.Fatal("forwarding encoding accepted a stale upstream binding")
			}
			binding = changed.Clients[ClientCodex]
			if !reflect.DeepEqual(changed, beforeRefusal) || binding.ForwardingEndpoint != recorded.ForwardingEndpoint ||
				binding.ForwardingUpstreamIdentity != recorded.ForwardingUpstreamIdentity {
				t.Fatal("validation or encoding refreshed the recorded upstream")
			}
			if change == "binding-protocol" {
				return // Codex cannot encode a direct Chat Completions binding either.
			}
			direct := changed.Clone()
			direct.Clients[ClientCodex] = direct.Clients[ClientCodex].withoutForwarding()
			data, err := encodeConfig(direct)
			if err != nil {
				t.Fatal(err)
			}
			if change == "recorded-protocol" {
				sidecar = bytes.ReplaceAll(sidecar, []byte("openai_responses"), []byte("openai_chat_completions"))
			}
			configInput := bytes.Clone(data)
			forwardingInput := bytes.Clone(sidecar)
			if _, err := decodeStoreConfig(data, transaction.FileSnapshot{Exists: true, Data: sidecar}); err == nil {
				t.Fatal("stored configuration drift was accepted with the old sidecar")
			}
			if !bytes.Equal(data, configInput) || !bytes.Equal(sidecar, forwardingInput) {
				t.Fatal("failed decoding changed its configuration or forwarding inputs")
			}
		})
	}
}

func TestForwardingDestinationsAreCodexOnly(t *testing.T) {
	for _, client := range []string{ClientClaude, ClientClaudeDesktop, ClientHermes} {
		t.Run(client, func(t *testing.T) {
			cfg := validConfig()
			cfg.Normalize()
			before := cfg.Clone()
			if err := cfg.SetForwardingEndpoint(client, "http://127.0.0.1:8792/ucloud/v1"); err == nil {
				t.Fatalf("accepted forwarding for %s", client)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatalf("refused forwarding changed configuration for %s", client)
			}
		})
	}
}

func TestCodexDirectSelectionWithdrawsCompleteForwarding(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	cfg.Clients[ClientHermes] = ClientBinding{Route: "backup", Protocol: ProtocolOpenAIResponses}
	before, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/ucloud/v1"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetForwardingEndpoint(ClientCodex, ""); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	if binding.ForwardingEndpoint != "" || binding.ForwardingUpstreamIdentity != "" || binding.ForwardingProtocol != "" {
		t.Fatalf("direct selection did not clear the complete binding: %+v", binding)
	}
	after, err := encodeConfig(cfg)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("direct withdrawal changed main configuration: %v", err)
	}
	sidecar, err := encodeForwarding(cfg)
	if err != nil || len(sidecar) != 0 {
		t.Fatalf("direct withdrawal left a forwarding sidecar: %q, %v", sidecar, err)
	}
	got, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil || got.Endpoint != direct.Endpoint ||
		got.CredentialProjectionFingerprint(ClientCodex) != direct.CredentialProjectionFingerprint(ClientCodex) {
		t.Fatalf("direct selection changed the original upstream: %+v, %v", got, err)
	}
}
