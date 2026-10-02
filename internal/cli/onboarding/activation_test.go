package onboarding

import (
	"bytes"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestManifestSetupOffersEqualCompatibleAccountsWithoutTokens(t *testing.T) {
	cfg := configuration.NewConfig()
	for _, account := range []string{"ucloud", "dmx"} {
		cfg.Accounts[account] = configuration.Account{Label: account, Endpoints: configuration.Endpoints{Anthropic: "https://" + account + ".test"}}
		cfg.Routes[account] = configuration.Route{Label: account, Account: account, Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "ucloud")
	recommendation := cfg.Recommendations[configuration.ClientClaude]
	recommendation.Alternatives = []configuration.ClientSelection{{Route: "dmx"}}
	cfg.Recommendations[configuration.ClientClaude] = recommendation
	output := &bytes.Buffer{}
	runtime := invocation.Context{Secrets: secrets.NewMemoryStore(), Out: output, RenderOut: output}
	result := buildManifestSetupResult(runtime, cfg, []string{"ucloud", "dmx"}, "", nil, nil)
	want := "Choose one compatible Account: aigw rotate dmx or aigw rotate ucloud"
	if result.NextAction != want || len(result.DeferredActions) != 1 || result.DeferredActions[0] != want {
		t.Fatalf("zero-Token setup decision = %+v, want %q", result, want)
	}
	renderManifestSetupResult(runtime, result)
	if !strings.Contains(output.String(), want) || strings.Contains(output.String(), "Next: aigw sync") {
		t.Fatalf("zero-Token setup rendered a different or no-op action: %q", output.String())
	}
}

func TestGuidedSetupDefersMissingClientBeforeVerification(t *testing.T) {
	cfg := manifestSetupConfig()
	cfg.SetClientActivation(configuration.ClientCodex, false, "", nil)
	store := secrets.NewMemoryStore()
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	renderSetupClients(invocation.Context{Secrets: store, Out: output, RenderOut: output}, cfg, configuration.ClientClaude)
	want := "Install Claude if needed, then run `aigw sync`"
	if !strings.Contains(output.String(), want) || strings.Contains(output.String(), "Next: aigw check") {
		t.Fatalf("guided setup admitted verification before projection: %q", output.String())
	}
}

func TestManifestSetupReportsSelectedCredentialBeforeDeferredProjection(t *testing.T) {
	cfg := manifestSetupConfig()
	runtime := invocation.Context{Secrets: secrets.NewMemoryStore()}
	result := buildManifestSetupResult(runtime, cfg, []string{"team"}, "", nil, nil)
	want := "run `aigw rotate team`"
	if result.NextAction != want || len(result.DeferredActions) == 0 || result.DeferredActions[0] != want {
		t.Fatalf("selected credential prerequisite is missing or out of order: %+v", result)
	}
}
