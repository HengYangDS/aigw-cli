package domain

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
	if err != nil || resolved.ProviderID != "dmx" || resolved.Endpoint != cfg.Providers["dmx"].Endpoints.Anthropic {
		t.Fatalf("selected Account capability = %#v, %v", resolved, err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatal("runtime resolution changed the supplied configuration")
	}
	empty := Config{admission: testAdmission()}
	if _, err := empty.ResolveRuntime(ClientClaude, ""); err == nil || !reflect.DeepEqual(empty, Config{admission: testAdmission()}) {
		t.Fatalf("empty configuration resolution = %#v, %v", empty, err)
	}
}

func TestResolveRuntimeClassifiesAnUnselectedModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	_, err := cfg.ResolveRuntime(ClientClaude, "")
	var routeErr *RuntimeBindingUnselectedError
	if !errors.As(err, &routeErr) || routeErr.client != ClientClaude {
		t.Fatalf("unselected route error = %#v, %v", routeErr, err)
	}
}

func TestSetSelectedModelUsesTheNewModelsOnlyAdmittedProtocol(t *testing.T) {
	cfg := validConfig()
	cfg.Clients[ClientHermes] = ClientBinding{
		Model:    "dmx",
		Enabled:  true,
		Protocol: ProtocolAnthropic,
	}

	cfg.SetSelectedModel(ClientHermes, "backup", "")

	binding := cfg.Clients[ClientHermes]
	if binding.Model != "backup" {
		t.Fatalf("selected Route = %q, want backup", binding.Model)
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

func TestResolveRuntimeReturnsProviderEndpointAndModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["dmx"] = Provider{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Models["gpt-5.6"] = testModel("GPT-5.6", "dmx", "gpt-5.6", ProtocolOpenAIResponses)
	cfg.Models["claude-opus"] = testModel("Claude Opus", "dmx", "claude-opus", ProtocolAnthropic)
	cfg.SetSelectedModel(ClientCodex, "gpt-5.6", "")
	cfg.SetSelectedModel(ClientClaude, "claude-opus", "")
	got, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelID != "gpt-5.6" || got.ProviderID != "dmx" || got.UpstreamModel != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v", got)
	}
	got, err = cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelID != "claude-opus" || got.UpstreamModel != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestResolveRuntimeRejectsUnknownModelProviderAndEndpoint(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["dmx"] = Provider{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Models["codex"] = testModel("Codex", "dmx", "gpt-test", ProtocolOpenAIResponses)
	cfg.SetSelectedModel(ClientCodex, "codex", "")

	if _, err := cfg.ResolveRuntime(ClientCodex, "missing-route"); err == nil || !strings.Contains(err.Error(), "unknown route") {
		t.Fatalf("unknown route error = %v", err)
	}

	orphan := cfg
	orphan.Models = map[string]Model{"codex": testModel("Codex", "missing-account", "gpt-test", ProtocolOpenAIResponses)}
	orphan.SetSelectedModel(ClientCodex, "codex", "")
	_, err := orphan.ResolveRuntime(ClientCodex, "")
	var accountErr *RuntimeModelUnknownProviderError
	if !errors.As(err, &accountErr) || accountErr.ModelID != "codex" || accountErr.ProviderID != "missing-account" {
		t.Fatalf("unknown account error = %v", err)
	}

	claudeRoute := cfg.Models["codex"]
	claudeRoute.Family = "claude-test"
	claudeRoute.Protocols = map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}
	cfg.Models["claude"] = claudeRoute
	_, err = cfg.ResolveRuntime(ClientClaude, "claude")
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.ProviderID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}
}

func TestSelectModelsForConnectedProvidersKeepsCapabilityAndChoosesUsableModels(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["alpha"] = Provider{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Providers["beta"] = Provider{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test", OpenAIResponses: "https://beta.test/v1"}}
	cfg.Models["alpha-claude"] = testModel("Alpha Claude", "alpha", "claude-test", ProtocolAnthropic)
	cfg.Models["alpha-codex"] = testModel("Alpha Codex", "alpha", "gpt-test", ProtocolOpenAIResponses)
	cfg.Models["beta-claude"] = testModel("Beta Claude", "beta", "claude-test", ProtocolAnthropic)
	cfg.Models["beta-codex"] = testModel("Beta Codex", "beta", "gpt-test", ProtocolOpenAIResponses)
	cfg.SetRecommendedModel(ClientClaude, "alpha-claude")
	cfg.SetRecommendedModel(ClientCodex, "alpha-codex")
	claudeRecommendation := cfg.Recommendations[ClientClaude]
	claudeRecommendation.Alternatives = []ClientSelection{{Model: "beta-claude"}}
	cfg.Recommendations[ClientClaude] = claudeRecommendation
	codexRecommendation := cfg.Recommendations[ClientCodex]
	codexRecommendation.Alternatives = []ClientSelection{{Model: "beta-codex"}}
	cfg.Recommendations[ClientCodex] = codexRecommendation

	one, err := cfg.SelectModelsForConnectedProviders([]string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if one.SelectedModel(ClientClaude) != "beta-claude" || one.SelectedModel(ClientCodex) != "beta-codex" {
		t.Fatalf("one connected Account bindings = %#v", one.Clients)
	}
	if len(one.Providers) != 2 || len(one.Models) != 4 {
		t.Fatalf("route selection discarded catalog capability: %#v", one)
	}
	if !one.Clients[ClientClaude].Enabled || !one.Clients[ClientCodex].Enabled {
		t.Fatalf("explicit setup selection did not retain deferred activation intent: %#v", one.Clients)
	}

	both, err := cfg.SelectModelsForConnectedProviders([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if both.SelectedModel(ClientClaude) != cfg.RecommendedModel(ClientClaude) || both.SelectedModel(ClientCodex) != cfg.RecommendedModel(ClientCodex) {
		t.Fatalf("usable recommendations changed: bindings=%#v recommendations=%#v", both.Clients, cfg.Recommendations)
	}

	if _, err := cfg.SelectModelsForConnectedProviders([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("unknown connected Account error = %v", err)
	}
}

func TestSelectModelsForConnectedProvidersPreservesRecommendedModels(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["preferred"] = Provider{Label: "Preferred", Endpoints: Endpoints{Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Providers["connected"] = Provider{Label: "Connected", Endpoints: Endpoints{Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1"}}
	cfg.Models["preferred-claude"] = testModel("Preferred Claude", "preferred", "claude-fable-5", ProtocolAnthropic)
	cfg.Models["preferred-codex"] = testModel("Preferred Codex", "preferred", "gpt-5.6-sol", ProtocolOpenAIResponses)
	cfg.Models["connected-claude"] = testModel("Connected Claude", "connected", "claude-fable-5", ProtocolAnthropic)
	cfg.Models["connected-codex-luna"] = testModel("Connected Codex Luna", "connected", "gpt-5.6-luna", ProtocolOpenAIResponses)
	cfg.Models["connected-codex-sol"] = testModel("Connected Codex Sol", "connected", "gpt-5.6-sol", ProtocolOpenAIResponses)
	cfg.SetRecommendedModel(ClientClaude, "preferred-claude")
	cfg.SetRecommendedModel(ClientCodex, "preferred-codex")
	claudeRecommendation := cfg.Recommendations[ClientClaude]
	claudeRecommendation.Alternatives = []ClientSelection{{Model: "connected-claude"}}
	cfg.Recommendations[ClientClaude] = claudeRecommendation
	codexRecommendation := cfg.Recommendations[ClientCodex]
	codexRecommendation.Alternatives = []ClientSelection{{Model: "connected-codex-sol"}}
	cfg.Recommendations[ClientCodex] = codexRecommendation

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != "connected-codex-sol" {
		t.Fatalf("Codex binding lost the recommended model: %#v", selected.Clients)
	}
	if selected.SelectedModel(ClientClaude) != "connected-claude" {
		t.Fatalf("Claude route = %q", selected.SelectedModel(ClientClaude))
	}
}

func TestSelectModelsForConnectedProvidersDoesNotSubstituteAnotherRecommendedModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["preferred"] = Provider{Label: "Preferred", Endpoints: Endpoints{OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Providers["connected"] = Provider{Label: "Connected", Endpoints: Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Models["preferred"] = testModel("Preferred", "preferred", "gpt-preferred", ProtocolOpenAIResponses)
	cfg.Models["different"] = testModel("Different", "connected", "gpt-different", ProtocolOpenAIResponses)
	cfg.SetRecommendedModel(ClientCodex, "preferred")

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != "" {
		t.Fatalf("unavailable recommendation was replaced by a different model: %#v", selected.Clients)
	}
}

func TestSelectModelsPreservesUnavailableExplicitChoice(t *testing.T) {
	cfg := validConfig()
	cfg.SetRecommendedModel(ClientCodex, "backup")
	chosen, err := cfg.SelectModelsForConnectedProviders(nil)
	if err != nil || !reflect.DeepEqual(chosen.Clients, cfg.Clients) {
		t.Fatalf("unavailable explicit choices changed: %#v, %v", chosen.Clients, err)
	}
}

func TestSelectModelsPrefersUsableRecommendationOverIdentifierOrder(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Models["alpha"] = testModel("Alpha", "team", "first-model", ProtocolAnthropic)
	cfg.Models["zeta"] = testModel("Zeta", "team", "recommended-model", ProtocolAnthropic)
	cfg.SetRecommendedModel(ClientClaude, "zeta")
	deferred, err := cfg.SelectModelsForConnectedProviders(nil)
	if err != nil || len(deferred.Clients) != 0 || deferred.RecommendedModel(ClientClaude) != "zeta" {
		t.Fatalf("deferred recommendation = %#v, %v", deferred, err)
	}
	selected, err := deferred.SelectModelsForConnectedProviders([]string{"team"})
	if err != nil || selected.SelectedModel(ClientClaude) != "zeta" {
		t.Fatalf("recommendation lost to identifier order: %#v, %v", selected.Clients, err)
	}
}

func TestSelectedProviderIDsReturnsUniqueStableActiveProviders(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["alpha"] = Provider{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Providers["optional"] = Provider{Label: "Optional", Endpoints: Endpoints{OpenAIResponses: "https://optional.test/v1"}}
	cfg.Models["alpha-claude"] = testModel("Alpha Claude", "alpha", "claude-test", ProtocolAnthropic)
	cfg.Models["alpha-codex"] = testModel("Alpha Codex", "alpha", "gpt-test", ProtocolOpenAIResponses)
	cfg.Models["optional"] = testModel("Optional", "optional", "gpt-optional", ProtocolOpenAIResponses)
	cfg.SetSelectedModel(ClientClaude, "alpha-claude", "")
	cfg.SetSelectedModel(ClientCodex, "alpha-codex", "")

	if got := cfg.SelectedProviderIDs(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("SelectedAccountIDs() = %#v", got)
	}
}

func TestSelectModelsForConnectedProvidersSelectsACompatibleModelForAnUnselectedClient(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["claude-only"] = Provider{Label: "Claude only", Endpoints: Endpoints{Anthropic: "https://claude.test"}}
	cfg.Providers["codex"] = Provider{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Models["claude"] = testModel("Claude", "claude-only", "claude-test", ProtocolAnthropic)
	cfg.Models["codex"] = testModel("Codex", "codex", "gpt-test", ProtocolOpenAIResponses)
	cfg.SetSelectedModel(ClientCodex, "codex", "")
	cfg.SetRecommendedModel(ClientClaude, "claude")

	got, err := cfg.SelectModelsForConnectedProviders([]string{"claude-only"}, ClientClaude)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(ClientClaude) != "claude" {
		t.Fatalf("selected bindings = %#v", got.Clients)
	}
}

func TestSelectModelsForConnectedProvidersDefaultsToRecommendedClients(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	cfg.Models["shared"] = testModel("Shared", "team", "model", ProtocolOpenAIResponses)
	cfg.SetRecommendedModel(ClientCodex, "shared")

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != "shared" || !selected.Clients[ClientCodex].Enabled {
		t.Fatalf("recommended Codex intent = %#v", selected.Clients[ClientCodex])
	}
	if _, exists := selected.Clients[ClientClaude]; exists {
		t.Fatalf("unrecommended Claude was selected: %#v", selected.Clients[ClientClaude])
	}
	if _, exists := selected.Clients[ClientHermes]; exists {
		t.Fatalf("new client admission changed setup intent: %#v", selected.Clients[ClientHermes])
	}
}

func TestSelectModelsForConnectedProvidersPreservesRecommendedProtocolForEquivalentModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["preferred"] = Provider{Label: "Preferred", Endpoints: Endpoints{
		Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1",
	}}
	cfg.Providers["connected"] = Provider{Label: "Connected", Endpoints: Endpoints{
		Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1",
	}}
	cfg.Models["preferred"] = testModel("Preferred", "preferred", "shared-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Models["connected"] = testModel("Connected", "connected", "shared-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{
		Primary:      ClientSelection{Model: "preferred", Protocol: ProtocolAnthropic},
		Alternatives: []ClientSelection{{Model: "connected", Protocol: ProtocolAnthropic}},
	}

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	binding := selected.Clients[ClientHermes]
	if binding.Model != "connected" || binding.Protocol != ProtocolAnthropic {
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

func TestExplicitModelSelectionUsesUnboundRecommendationOptions(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	cfg.Models["recommended"] = testModel("Recommended", "team", "recommended-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Models["selected"] = testModel("Selected", "team", "selected-model", ProtocolAnthropic, ProtocolOpenAIResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: "recommended", Protocol: ProtocolAnthropic}}

	runtime, err := cfg.ResolveRuntime(ClientHermes, "selected")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelID != "selected" || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("explicit Hermes runtime = %#v", runtime)
	}
	cfg.SetSelectedModel(ClientHermes, "selected", "")
	if binding := cfg.Clients[ClientHermes]; binding.Model != "selected" || binding.Protocol != ProtocolAnthropic {
		t.Fatalf("stored Hermes binding = %#v", binding)
	}
}

func TestSelectModelsForConnectedProvidersSkipsModelsWithoutTheClientEndpoint(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Models["generic"] = testModel("Generic", "team", "claude-test", ProtocolAnthropic)
	cfg.Models["invalid-codex"] = testModel("Invalid Codex", "team", "gpt-test", ProtocolOpenAIResponses)

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != "" {
		t.Fatalf("route without Responses endpoint selected for Codex: %#v", selected.Clients)
	}
}

func TestClientEndpointRejectsUnknownClientAndMissingProtocolEndpoint(t *testing.T) {
	account := Provider{ID: "dmx", Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://dmx.test"}}
	admission := testAdmission()
	if _, admitted := admission.Spec("gemini"); admitted {
		t.Fatal("an unknown client is admitted")
	}
	codex, _ := admission.Spec(ClientCodex)
	if _, _, err := codex.ResolveEndpoint(account, ""); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
		t.Fatalf("missing OpenAI endpoint error = %v", err)
	}
	claude, _ := admission.Spec(ClientClaude)
	if endpoint, _, err := claude.ResolveEndpoint(account, ""); err != nil || endpoint != "https://dmx.test" {
		t.Fatalf("ResolveEndpoint = %q, %v", endpoint, err)
	}
}

func TestResolveProviderRejectsModelWithUnknownProvider(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Models["orphan"] = testModel("Orphan", "missing", "", ProtocolAnthropic)
	_, _, err := cfg.ResolveProvider("orphan")
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("orphan route error = %v", err)
	}
}

func TestDomainErrorTextAndModelSelectionBranches(t *testing.T) {
	if got := ProtocolLabel(ProtocolAnthropic); got != "Anthropic Messages" {
		t.Fatalf("Anthropic protocol name = %q", got)
	}
	if got := ProtocolLabel(ProtocolOpenAIResponses); got != "OpenAI Responses" {
		t.Fatalf("Responses protocol name = %q", got)
	}
	if got := (&RuntimeModelClientMismatchError{ModelID: "one", ExpectedClient: "codex", ActualClient: "claude"}).Error(); !strings.Contains(got, "for codex") {
		t.Fatalf("mismatch error = %q", got)
	}
	if got := (&RuntimeModelUnknownProviderError{ModelID: "one", ProviderID: "missing"}).Error(); !strings.Contains(got, "unknown account") {
		t.Fatalf("account error = %q", got)
	}
	if got := (&RuntimeMissingEndpointError{ProviderID: "one", Protocol: EndpointProtocol("future")}).Error(); !strings.Contains(got, "future endpoint") {
		t.Fatalf("endpoint error = %q", got)
	}

	cfg := NewConfig(testAdmission())
	cfg.Providers["generic"] = Provider{Label: "Generic", Endpoints: Endpoints{Anthropic: "https://one.test"}}
	cfg.Models["a-skip"] = testModel("Skip", "generic", "", ProtocolAnthropic)
	cfg.Models["b-endpoint"] = testModel("Endpoint", "generic", "claude-endpoint", ProtocolAnthropic)
	for _, route := range []string{"a-skip", "b-endpoint"} {
		if _, err := cfg.ResolveRuntime(ClientCodex, route); err == nil {
			t.Fatalf("Codex resolved the Anthropic-only Route %q", route)
		}
	}
	if _, err := cfg.ResolveRuntime(ClientClaude, "a-skip"); err != nil {
		t.Fatalf("Claude Route without a Model: %v", err)
	}
}

func TestSelectedModelCopiesRecommendedOptionsOnlyForItsProvider(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["native"] = Provider{Label: "Native", Endpoints: Endpoints{OpenAIResponses: "https://native.test/v1"}}
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Models["native-gpt"] = testModel("Native", "native", "gpt-native", ProtocolOpenAIResponses)
	cfg.Models["team-gpt"] = testModel("Team", "team", "gpt-team", ProtocolOpenAIResponses)
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{
		Model: "native-gpt", ModelProvider: "native-provider", Authentication: AuthenticationClientNative,
	}}

	other := cfg.Clone()
	other.SetSelectedModel(ClientCodex, "team-gpt", "")
	runtime, err := other.ResolveRuntime(ClientCodex, "")
	if err != nil || !runtime.RequiresProviderToken() || other.Clients[ClientCodex].ModelProvider != "" {
		t.Fatalf("another Account's Route inherited the recommendation's native options: %#v, %v", other.Clients[ClientCodex], err)
	}
	recommended := cfg.Clone()
	recommended.SetSelectedModel(ClientCodex, "native-gpt", "")
	if binding := recommended.Clients[ClientCodex]; binding.Authentication != AuthenticationClientNative || binding.ModelProvider != "native-provider" {
		t.Fatalf("the recommended Route lost its native options: %#v", binding)
	}
}
