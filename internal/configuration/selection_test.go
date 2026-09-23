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

func TestSetSelectedRouteUsesTheNewRoutesOnlyAdmittedProtocol(t *testing.T) {
	cfg := validConfig()
	cfg.Clients[ClientHermes] = ClientBinding{
		Route:    "dmx",
		Enabled:  true,
		Protocol: ProtocolAnthropic,
	}

	cfg.SetSelectedRoute(ClientHermes, "backup")

	binding := cfg.Clients[ClientHermes]
	if binding.Route != "backup" {
		t.Fatalf("selected Route = %q, want backup", binding.Route)
	}
	if binding.Protocol != ProtocolOpenAIResponses {
		t.Fatalf("selected protocol = %q, want %q", binding.Protocol, ProtocolOpenAIResponses)
	}
	runtime, err := cfg.ResolveRuntime(ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Protocol != ProtocolOpenAIResponses {
		t.Fatalf("runtime protocol = %q, want %q", runtime.Protocol, ProtocolOpenAIResponses)
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
	cfg.Routes["gpt-5.6"] = testRoute("GPT-5.6", "dmx", "gpt-5.6", ProtocolOpenAIResponses)
	cfg.Routes["claude-opus"] = testRoute("Claude Opus", "dmx", "claude-opus", ProtocolAnthropic)
	cfg.SetSelectedRoute(ClientCodex, "gpt-5.6")
	cfg.SetSelectedRoute(ClientClaude, "claude-opus")
	got, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.RouteID != "gpt-5.6" || got.AccountID != "dmx" || got.Model != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v", got)
	}
	got, err = cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.RouteID != "claude-opus" || got.Model != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestClientForRouteRequiresCanonicalScope(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["codex"] = Account{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Accounts["shared"] = Account{Label: "Shared", Endpoints: Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Accounts["hermes"] = Account{Label: "Hermes", Endpoints: Endpoints{OpenAIChatCompletions: "https://hermes.test/v1"}}
	cfg.Routes["codex"] = testRoute("Codex", "codex", "gpt-5.6", ProtocolOpenAIResponses)
	cfg.Routes["shared"] = testRoute("Shared", "shared", "gpt-5.6", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Routes["hermes"] = testRoute("Hermes", "hermes", "model", ProtocolOpenAIChatCompletions)

	client, err := cfg.ClientForRoute("hermes")
	if err != nil || client != ClientHermes {
		t.Fatalf("ClientForRoute(hermes) = %q, %v", client, err)
	}
	if _, err := cfg.ClientForRoute("codex"); err == nil || !strings.Contains(err.Error(), "compatible with 2 clients") {
		t.Fatalf("multi-client route error = %v", err)
	}
	if _, err := cfg.ClientForRoute("shared"); err == nil || !strings.Contains(err.Error(), "compatible with 4 clients") {
		t.Fatalf("unscoped route error = %v", err)
	}
	if _, err := cfg.ClientForRoute("missing"); err == nil || !strings.Contains(err.Error(), "unknown route") {
		t.Fatalf("unknown route error = %v", err)
	}
}

func TestResolveRuntimeRejectsUnknownRouteAccountAndEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Routes["codex"] = testRoute("Codex", "dmx", "gpt-test", ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ClientCodex, "codex")

	if _, err := cfg.ResolveRuntime(ClientCodex, "missing-route"); err == nil || !strings.Contains(err.Error(), "unknown route") {
		t.Fatalf("unknown route error = %v", err)
	}

	orphan := cfg
	orphan.Routes = map[string]Route{"codex": testRoute("Codex", "missing-account", "gpt-test", ProtocolOpenAIResponses)}
	orphan.SetSelectedRoute(ClientCodex, "codex")
	_, err := orphan.ResolveRuntime(ClientCodex, "")
	var accountErr *RuntimeRouteUnknownAccountError
	if !errors.As(err, &accountErr) || accountErr.RouteID != "codex" || accountErr.AccountID != "missing-account" {
		t.Fatalf("unknown account error = %v", err)
	}

	claudeRoute := cfg.Routes["codex"]
	claudeRoute.Model = "claude-test"
	claudeRoute.Interfaces = map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}
	cfg.Routes["claude"] = claudeRoute
	_, err = cfg.ResolveRuntime(ClientClaude, "claude")
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.AccountID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsKeepsCapabilityAndChoosesUsableRoutes(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["beta"] = Account{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test", OpenAIResponses: "https://beta.test/v1"}}
	cfg.Routes["alpha-claude"] = testRoute("Alpha Claude", "alpha", "claude-test", ProtocolAnthropic)
	cfg.Routes["alpha-codex"] = testRoute("Alpha Codex", "alpha", "gpt-test", ProtocolOpenAIResponses)
	cfg.Routes["beta-claude"] = testRoute("Beta Claude", "beta", "claude-test", ProtocolAnthropic)
	cfg.Routes["beta-codex"] = testRoute("Beta Codex", "beta", "gpt-test", ProtocolOpenAIResponses)
	cfg.SetRecommendedRoute(ClientClaude, "alpha-claude")
	cfg.SetRecommendedRoute(ClientCodex, "alpha-codex")
	claudeRecommendation := cfg.Recommendations[ClientClaude]
	claudeRecommendation.Alternatives = []ClientSelection{{Route: "beta-claude"}}
	cfg.Recommendations[ClientClaude] = claudeRecommendation
	codexRecommendation := cfg.Recommendations[ClientCodex]
	codexRecommendation.Alternatives = []ClientSelection{{Route: "beta-codex"}}
	cfg.Recommendations[ClientCodex] = codexRecommendation

	one, err := cfg.SelectRoutesForConnectedAccounts([]string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if one.SelectedRoute(ClientClaude) != "beta-claude" || one.SelectedRoute(ClientCodex) != "beta-codex" {
		t.Fatalf("one connected Account bindings = %#v", one.Clients)
	}
	if len(one.Accounts) != 2 || len(one.Routes) != 4 {
		t.Fatalf("route selection discarded catalogue capability: %#v", one)
	}
	if !one.Clients[ClientClaude].Enabled || !one.Clients[ClientCodex].Enabled {
		t.Fatalf("explicit setup selection did not retain deferred activation intent: %#v", one.Clients)
	}

	both, err := cfg.SelectRoutesForConnectedAccounts([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if both.SelectedRoute(ClientClaude) != cfg.RecommendedRoute(ClientClaude) || both.SelectedRoute(ClientCodex) != cfg.RecommendedRoute(ClientCodex) {
		t.Fatalf("usable recommendations changed: bindings=%#v recommendations=%#v", both.Clients, cfg.Recommendations)
	}

	if _, err := cfg.SelectRoutesForConnectedAccounts([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("unknown connected Account error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsPreservesRecommendedModels(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1"}}
	cfg.Routes["preferred-claude"] = testRoute("Preferred Claude", "preferred", "claude-fable-5", ProtocolAnthropic)
	cfg.Routes["preferred-codex"] = testRoute("Preferred Codex", "preferred", "gpt-5.6-sol", ProtocolOpenAIResponses)
	cfg.Routes["connected-claude"] = testRoute("Connected Claude", "connected", "claude-fable-5", ProtocolAnthropic)
	cfg.Routes["connected-codex-luna"] = testRoute("Connected Codex Luna", "connected", "gpt-5.6-luna", ProtocolOpenAIResponses)
	cfg.Routes["connected-codex-sol"] = testRoute("Connected Codex Sol", "connected", "gpt-5.6-sol", ProtocolOpenAIResponses)
	cfg.SetRecommendedRoute(ClientClaude, "preferred-claude")
	cfg.SetRecommendedRoute(ClientCodex, "preferred-codex")
	claudeRecommendation := cfg.Recommendations[ClientClaude]
	claudeRecommendation.Alternatives = []ClientSelection{{Route: "connected-claude"}}
	cfg.Recommendations[ClientClaude] = claudeRecommendation
	codexRecommendation := cfg.Recommendations[ClientCodex]
	codexRecommendation.Alternatives = []ClientSelection{{Route: "connected-codex-sol"}}
	cfg.Recommendations[ClientCodex] = codexRecommendation

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedRoute(ClientCodex) != "connected-codex-sol" {
		t.Fatalf("Codex binding lost the recommended model: %#v", selected.Clients)
	}
	if selected.SelectedRoute(ClientClaude) != "connected-claude" {
		t.Fatalf("Claude route = %q", selected.SelectedRoute(ClientClaude))
	}
}

func TestSelectRoutesForConnectedAccountsDoesNotSubstituteAnotherRecommendedModel(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Routes["preferred"] = testRoute("Preferred", "preferred", "gpt-preferred", ProtocolOpenAIResponses)
	cfg.Routes["different"] = testRoute("Different", "connected", "gpt-different", ProtocolOpenAIResponses)
	cfg.SetRecommendedRoute(ClientCodex, "preferred")

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedRoute(ClientCodex) != "" {
		t.Fatalf("unavailable recommendation was replaced by a different model: %#v", selected.Clients)
	}
}

func TestSelectRoutesPreservesUnavailableExplicitChoice(t *testing.T) {
	cfg := validConfig()
	cfg.SetRecommendedRoute(ClientCodex, "backup")
	chosen, err := cfg.SelectRoutesForConnectedAccounts(nil)
	if err != nil || !reflect.DeepEqual(chosen.Clients, cfg.Clients) {
		t.Fatalf("unavailable explicit choices changed: %#v, %v", chosen.Clients, err)
	}
}

func TestSelectRoutesPrefersUsableRecommendationOverIdentifierOrder(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["alpha"] = testRoute("Alpha", "team", "first-model", ProtocolAnthropic)
	cfg.Routes["zeta"] = testRoute("Zeta", "team", "recommended-model", ProtocolAnthropic)
	cfg.SetRecommendedRoute(ClientClaude, "zeta")
	deferred, err := cfg.SelectRoutesForConnectedAccounts(nil)
	if err != nil || len(deferred.Clients) != 0 || deferred.RecommendedRoute(ClientClaude) != "zeta" {
		t.Fatalf("deferred recommendation = %#v, %v", deferred, err)
	}
	selected, err := deferred.SelectRoutesForConnectedAccounts([]string{"team"})
	if err != nil || selected.SelectedRoute(ClientClaude) != "zeta" {
		t.Fatalf("recommendation lost to identifier order: %#v, %v", selected.Clients, err)
	}
}

func TestSelectedAccountIDsReturnsUniqueStableActiveAccounts(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["optional"] = Account{Label: "Optional", Endpoints: Endpoints{OpenAIResponses: "https://optional.test/v1"}}
	cfg.Routes["alpha-claude"] = testRoute("Alpha Claude", "alpha", "claude-test", ProtocolAnthropic)
	cfg.Routes["alpha-codex"] = testRoute("Alpha Codex", "alpha", "gpt-test", ProtocolOpenAIResponses)
	cfg.Routes["optional"] = testRoute("Optional", "optional", "gpt-optional", ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ClientClaude, "alpha-claude")
	cfg.SetSelectedRoute(ClientCodex, "alpha-codex")

	if got := cfg.SelectedAccountIDs(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("SelectedAccountIDs() = %#v", got)
	}
}

func TestSelectRoutesForConnectedAccountsSelectsACompatibleRouteForAnUnselectedClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["claude-only"] = Account{Label: "Claude only", Endpoints: Endpoints{Anthropic: "https://claude.test"}}
	cfg.Accounts["codex"] = Account{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Routes["claude"] = testRoute("Claude", "claude-only", "claude-test", ProtocolAnthropic)
	cfg.Routes["codex"] = testRoute("Codex", "codex", "gpt-test", ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ClientCodex, "codex")
	cfg.SetRecommendedRoute(ClientClaude, "claude")

	got, err := cfg.SelectRoutesForConnectedAccounts([]string{"claude-only"}, ClientClaude)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedRoute(ClientClaude) != "claude" {
		t.Fatalf("selected bindings = %#v", got.Clients)
	}
}

func TestSelectRoutesForConnectedAccountsDefaultsToRecommendedClients(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	cfg.Routes["shared"] = testRoute("Shared", "team", "model", ProtocolOpenAIResponses)
	cfg.SetRecommendedRoute(ClientCodex, "shared")

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedRoute(ClientCodex) != "shared" || !selected.Clients[ClientCodex].Enabled {
		t.Fatalf("recommended Codex intent = %#v", selected.Clients[ClientCodex])
	}
	if _, exists := selected.Clients[ClientClaude]; exists {
		t.Fatalf("unrecommended Claude was selected: %#v", selected.Clients[ClientClaude])
	}
	if _, exists := selected.Clients[ClientHermes]; exists {
		t.Fatalf("new client admission changed setup intent: %#v", selected.Clients[ClientHermes])
	}
}

func TestSelectRoutesForConnectedAccountsPreservesRecommendedProtocolForEquivalentRoute(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{
		Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1",
	}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{
		Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1",
	}}
	cfg.Routes["preferred"] = testRoute("Preferred", "preferred", "shared-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Routes["connected"] = testRoute("Connected", "connected", "shared-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{
		Primary:      ClientSelection{Route: "preferred", Protocol: ProtocolAnthropic},
		Alternatives: []ClientSelection{{Route: "connected", Protocol: ProtocolAnthropic}},
	}

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	binding := selected.Clients[ClientHermes]
	if binding.Route != "connected" || binding.Protocol != ProtocolAnthropic {
		t.Fatalf("Hermes binding = %#v, want connected Route with Anthropic protocol", binding)
	}
	runtime, err := selected.ResolveRuntime(ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Endpoint != "https://connected.test" || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("Hermes runtime = %#v", runtime)
	}
}

func TestExplicitRouteSelectionUsesUnboundRecommendationOptions(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	cfg.Routes["recommended"] = testRoute("Recommended", "team", "recommended-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Routes["selected"] = testRoute("Selected", "team", "selected-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Route: "recommended", Protocol: ProtocolAnthropic}}

	runtime, err := cfg.ResolveRuntime(ClientHermes, "selected")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.RouteID != "selected" || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("explicit Hermes runtime = %#v", runtime)
	}
	cfg.SetSelectedRoute(ClientHermes, "selected")
	if binding := cfg.Clients[ClientHermes]; binding.Route != "selected" || binding.Protocol != ProtocolAnthropic {
		t.Fatalf("stored Hermes binding = %#v", binding)
	}
}

func TestSelectRoutesForConnectedAccountsSkipsRoutesWithoutTheClientEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["generic"] = testRoute("Generic", "team", "claude-test", ProtocolAnthropic)
	cfg.Routes["invalid-codex"] = testRoute("Invalid Codex", "team", "gpt-test", ProtocolOpenAIResponses)

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedRoute(ClientCodex) != "" {
		t.Fatalf("route without Responses endpoint selected for Codex: %#v", selected.Clients)
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

func TestResolveAccountRejectsRouteWithUnknownAccount(t *testing.T) {
	cfg := NewConfig()
	cfg.Routes["orphan"] = testRoute("Orphan", "missing", "", ProtocolAnthropic)
	_, _, err := cfg.ResolveAccount("orphan")
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("orphan route error = %v", err)
	}
}

func TestDomainErrorTextAndRouteSelectionBranches(t *testing.T) {
	if got := endpointProtocolName(ProtocolAnthropic); got != "Anthropic" {
		t.Fatalf("Anthropic protocol name = %q", got)
	}
	if got := endpointProtocolName(ProtocolOpenAIResponses); got != "OpenAI Responses" {
		t.Fatalf("Responses protocol name = %q", got)
	}
	if got := (&RuntimeRouteClientMismatchError{RouteID: "one", ExpectedClient: "codex", ActualClient: "claude"}).Error(); !strings.Contains(got, "for codex") {
		t.Fatalf("mismatch error = %q", got)
	}
	if got := (&RuntimeRouteUnknownAccountError{RouteID: "one", AccountID: "missing"}).Error(); !strings.Contains(got, "unknown account") {
		t.Fatalf("account error = %q", got)
	}
	if got := (&RuntimeMissingEndpointError{AccountID: "one", Protocol: EndpointProtocol("future")}).Error(); !strings.Contains(got, "future endpoint") {
		t.Fatalf("endpoint error = %q", got)
	}

	cfg := NewConfig()
	cfg.Accounts["generic"] = Account{Label: "Generic", Endpoints: Endpoints{Anthropic: "https://one.test"}}
	cfg.Routes["a-skip"] = testRoute("Skip", "generic", "", ProtocolAnthropic)
	cfg.Routes["b-endpoint"] = testRoute("Endpoint", "generic", "claude-endpoint", ProtocolAnthropic)
	if got := cfg.FirstRouteForClient(ClientCodex); got != "" {
		t.Fatalf("Codex route = %q", got)
	}
	if got := cfg.FirstRouteForClient(ClientClaude); got != "a-skip" {
		t.Fatalf("Claude route = %q", got)
	}
}
