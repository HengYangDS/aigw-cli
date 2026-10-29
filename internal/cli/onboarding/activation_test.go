package onboarding

import (
	"bytes"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

func TestManifestSetupOffersEqualCompatibleAccountsWithoutTokens(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	for _, account := range []string{"ucloud", "dmx"} {
		cfg.Accounts[account] = domain.Account{Label: account, Endpoints: domain.Endpoints{Anthropic: "https://" + account + ".test"}}
		cfg.Routes[account] = domain.Route{Label: account, Account: account, Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	}
	cfg.SetRecommendedRoute(claude.ID, "ucloud")
	recommendation := cfg.Recommendations[claude.ID]
	recommendation.Alternatives = []domain.ClientSelection{{Route: "dmx"}}
	cfg.Recommendations[claude.ID] = recommendation
	output := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Secrets: secret.NewMemoryStore(), Out: output, RenderOut: output}
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
	cfg.SetClientActivation(codex.ID, false, "", nil)
	store := secret.NewMemoryStore()
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	renderSetupClients(invocation.Context{Clients: clients.Registry(), Secrets: store, Out: output, RenderOut: output}, cfg, claude.ID)
	want := "Install Claude if needed, then run `aigw sync`"
	if !strings.Contains(output.String(), want) || strings.Contains(output.String(), "Next: aigw check") {
		t.Fatalf("guided setup admitted verification before projection: %q", output.String())
	}
}

func TestManifestSetupReportsSelectedCredentialBeforeDeferredProjection(t *testing.T) {
	cfg := manifestSetupConfig()
	runtime := invocation.Context{Clients: clients.Registry(), Secrets: secret.NewMemoryStore()}
	result := buildManifestSetupResult(runtime, cfg, []string{"team"}, "", nil, nil)
	want := "run `aigw rotate team`"
	if result.NextAction != want || len(result.DeferredActions) == 0 || result.DeferredActions[0] != want {
		t.Fatalf("selected credential prerequisite is missing or out of order: %+v", result)
	}
}
