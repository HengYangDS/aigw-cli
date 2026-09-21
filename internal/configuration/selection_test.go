package configuration

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeResolutionPreservesConfiguration(t *testing.T) {
	cfg := validConfig()
	want := validConfig()
	resolved, err := cfg.ResolveRuntime(ClientClaude, "")
	if err != nil || resolved.AccountID != "dmx" || resolved.Endpoint != cfg.Accounts["dmx"].Endpoints.Anthropic {
		t.Fatalf("selected Account capability = %#v, %v", resolved, err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatal("runtime resolution changed the supplied configuration")
	}
	empty := Config{}
	if _, err := empty.ResolveRuntime(ClientClaude, ""); err == nil || !reflect.DeepEqual(empty, Config{}) {
		t.Fatalf("empty configuration resolution = %#v, %v", empty, err)
	}
}

func TestResolveRuntimeClassifiesAnUnselectedRoute(t *testing.T) {
	cfg := NewConfig()
	_, err := cfg.ResolveRuntime(ClientClaude, "")
	var routeErr *RuntimeBindingUnselectedError
	if !errors.As(err, &routeErr) || routeErr.Client != ClientClaude {
		t.Fatalf("unselected route error = %#v, %v", routeErr, err)
	}
}

func BenchmarkResolveRuntime(b *testing.B) {
	cfg := validConfig()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := cfg.ResolveRuntime(ClientClaude, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func TestResolveRuntimeReturnsAccountEndpointAndModel(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6"] = Profile{Label: "GPT-5.6", Account: "dmx", Model: "gpt-5.6"}
	cfg.Profiles["claude-opus"] = Profile{Label: "Claude Opus", Account: "dmx", Model: "claude-opus"}
	cfg.SetSelectedProfile(ClientCodex, "gpt-5.6")
	cfg.SetSelectedProfile(ClientClaude, "claude-opus")
	got, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "gpt-5.6" || got.AccountID != "dmx" || got.Model != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v", got)
	}
	got, err = cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "claude-opus" || got.Model != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestClientForProfileRequiresCanonicalScope(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["codex"] = Account{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Accounts["shared"] = Account{Label: "Shared", Endpoints: Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Accounts["hermes"] = Account{Label: "Hermes", Endpoints: Endpoints{OpenAIChatCompletions: "https://hermes.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "codex", Model: "gpt-5.6"}
	cfg.Profiles["shared"] = Profile{Label: "Shared", Account: "shared", Model: "gpt-5.6"}
	cfg.Profiles["hermes"] = Profile{Label: "Hermes", Account: "hermes", Model: "model"}

	client, err := cfg.ClientForProfile("hermes")
	if err != nil || client != ClientHermes {
		t.Fatalf("ClientForProfile(hermes) = %q, %v", client, err)
	}
	if _, err := cfg.ClientForProfile("codex"); err == nil || !strings.Contains(err.Error(), "compatible with 2 clients") {
		t.Fatalf("multi-client profile error = %v", err)
	}
	if _, err := cfg.ClientForProfile("shared"); err == nil || !strings.Contains(err.Error(), "compatible with 3 clients") {
		t.Fatalf("unscoped profile error = %v", err)
	}
	if _, err := cfg.ClientForProfile("missing"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}
}

func TestResolveRuntimeRejectsUnknownProfileAccountAndEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Model: "gpt-test"}
	cfg.SetSelectedProfile(ClientCodex, "codex")

	if _, err := cfg.ResolveRuntime(ClientCodex, "missing-profile"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}

	orphan := cfg
	orphan.Profiles = map[string]Profile{"codex": {Label: "Codex", Account: "missing-account", Model: "gpt-test"}}
	orphan.SetSelectedProfile(ClientCodex, "codex")
	_, err := orphan.ResolveRuntime(ClientCodex, "")
	var accountErr *RuntimeProfileUnknownAccountError
	if !errors.As(err, &accountErr) || accountErr.ProfileID != "codex" || accountErr.AccountID != "missing-account" {
		t.Fatalf("unknown account error = %v", err)
	}

	claudeProfile := cfg.Profiles["codex"]
	claudeProfile.Model = "claude-test"
	cfg.Profiles["claude"] = claudeProfile
	_, err = cfg.ResolveRuntime(ClientClaude, "claude")
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.AccountID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}
}

func TestSelectProfilesForConnectedAccountsKeepsCapabilityAndChoosesUsableProfiles(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["beta"] = Account{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test", OpenAIResponses: "https://beta.test/v1"}}
	cfg.Profiles["alpha-claude"] = Profile{Label: "Alpha Claude", Account: "alpha", Model: "claude-test"}
	cfg.Profiles["alpha-codex"] = Profile{Label: "Alpha Codex", Account: "alpha", Model: "gpt-test"}
	cfg.Profiles["beta-claude"] = Profile{Label: "Beta Claude", Account: "beta", Model: "claude-test"}
	cfg.Profiles["beta-codex"] = Profile{Label: "Beta Codex", Account: "beta", Model: "gpt-test"}
	cfg.SetRecommendedProfile(ClientClaude, "alpha-claude")
	cfg.SetRecommendedProfile(ClientCodex, "alpha-codex")

	one, err := cfg.SelectProfilesForConnectedAccounts([]string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if one.SelectedProfile(ClientClaude) != "beta-claude" || one.SelectedProfile(ClientCodex) != "beta-codex" {
		t.Fatalf("one connected Account bindings = %#v", one.Clients)
	}
	if len(one.Accounts) != 2 || len(one.Profiles) != 4 {
		t.Fatalf("route selection discarded catalogue capability: %#v", one)
	}
	if !one.Clients[ClientClaude].Enabled || !one.Clients[ClientCodex].Enabled {
		t.Fatalf("explicit setup selection did not retain deferred activation intent: %#v", one.Clients)
	}

	both, err := cfg.SelectProfilesForConnectedAccounts([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if both.SelectedProfile(ClientClaude) != cfg.RecommendedProfile(ClientClaude) || both.SelectedProfile(ClientCodex) != cfg.RecommendedProfile(ClientCodex) {
		t.Fatalf("usable recommendations changed: bindings=%#v recommendations=%#v", both.Clients, cfg.Recommendations)
	}

	if _, err := cfg.SelectProfilesForConnectedAccounts([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("unknown connected Account error = %v", err)
	}
}

func TestSelectProfilesForConnectedAccountsPreservesRecommendedModels(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1"}}
	cfg.Profiles["preferred-claude"] = Profile{Label: "Preferred Claude", Account: "preferred", Model: "claude-fable-5"}
	cfg.Profiles["preferred-codex"] = Profile{Label: "Preferred Codex", Account: "preferred", Model: "gpt-5.6-sol"}
	cfg.Profiles["connected-claude"] = Profile{Label: "Connected Claude", Account: "connected", Model: "claude-fable-5"}
	cfg.Profiles["connected-codex-luna"] = Profile{Label: "Connected Codex Luna", Account: "connected", Model: "gpt-5.6-luna"}
	cfg.Profiles["connected-codex-sol"] = Profile{Label: "Connected Codex Sol", Account: "connected", Model: "gpt-5.6-sol"}
	cfg.SetRecommendedProfile(ClientClaude, "preferred-claude")
	cfg.SetRecommendedProfile(ClientCodex, "preferred-codex")

	selected, err := cfg.SelectProfilesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedProfile(ClientCodex) != "connected-codex-sol" {
		t.Fatalf("Codex binding lost the recommended model: %#v", selected.Clients)
	}
	if selected.SelectedProfile(ClientClaude) != "connected-claude" {
		t.Fatalf("Claude profile = %q", selected.SelectedProfile(ClientClaude))
	}
}

func TestSelectProfilesForConnectedAccountsDoesNotSubstituteAnotherRecommendedModel(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Profiles["preferred"] = Profile{Label: "Preferred", Account: "preferred", Model: "gpt-preferred"}
	cfg.Profiles["different"] = Profile{Label: "Different", Account: "connected", Model: "gpt-different"}
	cfg.SetRecommendedProfile(ClientCodex, "preferred")

	selected, err := cfg.SelectProfilesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedProfile(ClientCodex) != "" {
		t.Fatalf("unavailable recommendation was replaced by a different model: %#v", selected.Clients)
	}
}

func TestSelectProfilesPreservesUnavailableExplicitChoice(t *testing.T) {
	cfg := validConfig()
	cfg.SetRecommendedProfile(ClientCodex, "backup")
	chosen, err := cfg.SelectProfilesForConnectedAccounts(nil)
	if err != nil || !reflect.DeepEqual(chosen.Clients, cfg.Clients) {
		t.Fatalf("unavailable explicit choices changed: %#v, %v", chosen.Clients, err)
	}
}

func TestSelectProfilesPrefersUsableRecommendationOverIdentifierOrder(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Profiles["alpha"] = Profile{Label: "Alpha", Account: "team", Model: "first-model"}
	cfg.Profiles["zeta"] = Profile{Label: "Zeta", Account: "team", Model: "recommended-model"}
	cfg.SetRecommendedProfile(ClientClaude, "zeta")
	deferred, err := cfg.SelectProfilesForConnectedAccounts(nil)
	if err != nil || len(deferred.Clients) != 0 || deferred.RecommendedProfile(ClientClaude) != "zeta" {
		t.Fatalf("deferred recommendation = %#v, %v", deferred, err)
	}
	selected, err := deferred.SelectProfilesForConnectedAccounts([]string{"team"})
	if err != nil || selected.SelectedProfile(ClientClaude) != "zeta" {
		t.Fatalf("recommendation lost to identifier order: %#v, %v", selected.Clients, err)
	}
}

func TestSelectedAccountIDsReturnsUniqueStableActiveAccounts(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["optional"] = Account{Label: "Optional", Endpoints: Endpoints{OpenAIResponses: "https://optional.test/v1"}}
	cfg.Profiles["alpha-claude"] = Profile{Label: "Alpha Claude", Account: "alpha", Model: "claude-test"}
	cfg.Profiles["alpha-codex"] = Profile{Label: "Alpha Codex", Account: "alpha", Model: "gpt-test"}
	cfg.Profiles["optional"] = Profile{Label: "Optional", Account: "optional", Model: "gpt-optional"}
	cfg.SetSelectedProfile(ClientClaude, "alpha-claude")
	cfg.SetSelectedProfile(ClientCodex, "alpha-codex")

	if got := cfg.SelectedAccountIDs(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("SelectedAccountIDs() = %#v", got)
	}
}

func TestSelectProfilesForConnectedAccountsSelectsACompatibleProfileForAnUnselectedClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["claude-only"] = Account{Label: "Claude only", Endpoints: Endpoints{Anthropic: "https://claude.test"}}
	cfg.Accounts["codex"] = Account{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Profiles["claude"] = Profile{Label: "Claude", Account: "claude-only", Model: "claude-test"}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "codex", Model: "gpt-test"}
	cfg.SetSelectedProfile(ClientCodex, "codex")

	got, err := cfg.SelectProfilesForConnectedAccounts([]string{"claude-only"}, ClientClaude)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedProfile(ClientClaude) != "claude" {
		t.Fatalf("selected bindings = %#v", got.Clients)
	}
}

func TestSelectProfilesForConnectedAccountsDefaultsToRecommendedClients(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	cfg.Profiles["shared"] = Profile{Label: "Shared", Account: "team", Model: "model"}
	cfg.SetRecommendedProfile(ClientCodex, "shared")

	selected, err := cfg.SelectProfilesForConnectedAccounts([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedProfile(ClientCodex) != "shared" || !selected.Clients[ClientCodex].Enabled {
		t.Fatalf("recommended Codex intent = %#v", selected.Clients[ClientCodex])
	}
	if _, exists := selected.Clients[ClientClaude]; exists {
		t.Fatalf("unrecommended Claude was selected: %#v", selected.Clients[ClientClaude])
	}
	if _, exists := selected.Clients[ClientHermes]; exists {
		t.Fatalf("new client admission changed setup intent: %#v", selected.Clients[ClientHermes])
	}
}

func TestSelectProfilesForConnectedAccountsSkipsProfilesWithoutTheClientEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Profiles["generic"] = Profile{Label: "Generic", Account: "team", Model: "claude-test"}
	cfg.Profiles["invalid-codex"] = Profile{Label: "Invalid Codex", Account: "team", Model: "gpt-test"}

	selected, err := cfg.SelectProfilesForConnectedAccounts([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedProfile(ClientCodex) != "" {
		t.Fatalf("profile without Responses endpoint selected for Codex: %#v", selected.Clients)
	}
}

func TestEndpointForRejectsUnknownClientAndMissingProtocolEndpoint(t *testing.T) {
	account := Account{ID: "dmx", Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://dmx.test"}}
	if _, err := account.EndpointFor("gemini"); err == nil || !strings.Contains(err.Error(), "unknown client") {
		t.Fatalf("unknown client error = %v", err)
	}
	if _, err := account.EndpointFor(ClientCodex); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
		t.Fatalf("missing OpenAI endpoint error = %v", err)
	}
	if endpoint, err := account.EndpointFor(ClientClaude); err != nil || endpoint != "https://dmx.test" {
		t.Fatalf("EndpointFor = %q, %v", endpoint, err)
	}
}

func TestResolveAccountRejectsProfileWithUnknownAccount(t *testing.T) {
	cfg := NewConfig()
	cfg.Profiles["orphan"] = Profile{Label: "Orphan", Account: "missing"}
	_, _, err := cfg.ResolveAccount("orphan")
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("orphan profile error = %v", err)
	}
}

func TestDomainErrorTextAndProfileSelectionBranches(t *testing.T) {
	if got := endpointProtocolName(ProtocolAnthropic); got != "Anthropic" {
		t.Fatalf("Anthropic protocol name = %q", got)
	}
	if got := endpointProtocolName(ProtocolOpenAIResponses); got != "OpenAI Responses" {
		t.Fatalf("Responses protocol name = %q", got)
	}
	if got := (&RuntimeProfileClientMismatchError{ProfileID: "one", ExpectedClient: "codex", ActualClient: "claude"}).Error(); !strings.Contains(got, "for codex") {
		t.Fatalf("mismatch error = %q", got)
	}
	if got := (&RuntimeProfileUnknownAccountError{ProfileID: "one", AccountID: "missing"}).Error(); !strings.Contains(got, "unknown account") {
		t.Fatalf("account error = %q", got)
	}
	if got := (&RuntimeMissingEndpointError{AccountID: "one", Protocol: EndpointProtocol("future")}).Error(); !strings.Contains(got, "future endpoint") {
		t.Fatalf("endpoint error = %q", got)
	}

	cfg := NewConfig()
	cfg.Accounts["generic"] = Account{Label: "Generic", Endpoints: Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["a-skip"] = Profile{Label: "Skip", Account: "generic"}
	cfg.Profiles["b-endpoint"] = Profile{Label: "Endpoint", Account: "generic", Model: "claude-endpoint"}
	if got := cfg.FirstProfileForClient(ClientCodex); got != "" {
		t.Fatalf("Codex profile = %q", got)
	}
	if got := cfg.FirstProfileForClient(ClientClaude); got != "a-skip" {
		t.Fatalf("Claude profile = %q", got)
	}
}
