package client

import (
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	hermesconfig "aigw-cli/internal/hermes/configuration"
)

func TestHermesRouteForAdoptsOnlyOneExactRoute(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic: "https://gateway.test", OpenAIResponses: "https://gateway.test/v1",
	}}
	cfg.Routes["opus"] = qualifiedRoute("Opus", "gateway", "claude-opus-5-5", configuration.ProtocolAnthropic)
	cfg.Routes["sol"] = qualifiedRoute("Sol", "gateway", "gpt-6.1-sol", configuration.ProtocolOpenAIResponses)
	cfg.Normalize()

	routeID, protocol, err := hermesRouteFor(cfg, hermesconfig.Selection{Provider: "aigw-gateway-anthropic", Model: "claude-opus-5-5"})
	if err != nil || routeID != "opus" || protocol != configuration.ProtocolAnthropic {
		t.Fatalf("adopted Route = %q %q, %v", routeID, protocol, err)
	}
	// The model must belong to the selected provider's protocol, not any protocol.
	if _, _, err := hermesRouteFor(cfg, hermesconfig.Selection{Provider: "aigw-gateway-anthropic", Model: "gpt-6.1-sol"}); err == nil {
		t.Fatal("a model of another protocol was adopted")
	}
	if _, _, err := hermesRouteFor(cfg, hermesconfig.Selection{Provider: "aigw-other-anthropic", Model: "claude-opus-5-5"}); err == nil || !strings.Contains(err.Error(), "aigw use --for hermes") {
		t.Fatalf("an unprojected provider was not refused with a selection command: %v", err)
	}

	cfg.Routes["opus-copy"] = qualifiedRoute("Opus copy", "gateway", "claude-opus-5-5", configuration.ProtocolAnthropic)
	cfg.Normalize()
	_, _, err = hermesRouteFor(cfg, hermesconfig.Selection{Provider: "aigw-gateway-anthropic", Model: "claude-opus-5-5"})
	if err == nil {
		t.Fatal("an ambiguous native choice was adopted")
	}
	for _, want := range []string{"`aigw use --for hermes --protocol anthropic opus`", "`aigw use --for hermes --protocol anthropic opus-copy`"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ambiguity refusal %q does not name %s", err, want)
		}
	}
}
