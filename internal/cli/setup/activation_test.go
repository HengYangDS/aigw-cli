package setup

import (
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// Without a Token, each provider that a recommendation would select is an
// equal choice, and each names the command that stores its Token.
func TestTeamSetupOffersEqualCompatibleProvidersWithoutTokens(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	for _, account := range []string{"ucloud", "dmx"} {
		cfg.Providers[account] = domain.Provider{Label: account, Endpoints: domain.Endpoints{Anthropic: "https://" + account + ".test"}}
		cfg.Models[domain.NewModelRef(account, "fable")] = domain.Model{Label: account, Provider: account, Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	}
	cfg.SetRecommendedModel(claude.ID, domain.NewModelRef("ucloud", "fable"))
	recommendation := cfg.Recommendations[claude.ID]
	recommendation.Alternatives = []domain.ClientSelection{{Model: domain.NewModelRef("dmx", "fable")}}
	cfg.Recommendations[claude.ID] = recommendation
	runtime := invocation.Context{Clients: clients.Registry(), Secrets: secret.NewMemoryStore()}
	team := teamSetup{runtime: runtime, merged: cfg}
	document := team.document(cfg, map[string]string{"dmx": tokenMissing, "ucloud": tokenMissing}, nil)
	for _, token := range document.Tokens {
		if token.State != tokenMissing || token.NextDetail != "aigw provider rotate "+token.Provider {
			t.Errorf("token %+v", token)
		}
	}
	want := "Choose one compatible provider: aigw provider rotate dmx or aigw provider rotate ucloud"
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	if got := activation.NextActionFor(nil); got != want {
		t.Fatalf("zero-Token setup decision = %q, want %q", got, want)
	}
}

// A selected client without its provider's Token waits for the Token before
// its installation, because a client installed later reads it.
func TestTeamSetupReportsTheTokenBeforeTheClient(t *testing.T) {
	cfg := manifestSetupConfig()
	team := teamSetup{runtime: invocation.Context{Clients: clients.Registry(), Secrets: secret.NewMemoryStore()}, merged: cfg}
	spec, _ := cfg.Admission().Spec(codex.ID)
	if item := team.client(cfg, spec, nil, nil); item.State != clientWaitsForToken || item.NextDetail != "aigw provider rotate team" {
		t.Fatalf("client without a Token = %+v", item)
	}
	if item := team.client(cfg, spec, []string{"team"}, nil); item.State != clientWaitsForClient || item.NextDetail != "Install Codex if needed, then run `aigw sync`" {
		t.Fatalf("client without its installation = %+v", item)
	}
}
