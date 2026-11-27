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
	resolved, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil || resolved.ProviderID != "dmx" || resolved.Endpoint != cfg.Providers["dmx"].Endpoints.Anthropic {
		t.Fatalf("selected Account capability = %#v, %v", resolved, err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatal("runtime resolution changed the supplied configuration")
	}
	empty := Config{admission: testAdmission()}
	if _, err := empty.ResolveRuntime(ClientClaude, ModelRef{}); err == nil || !reflect.DeepEqual(empty, Config{admission: testAdmission()}) {
		t.Fatalf("empty configuration resolution = %#v, %v", empty, err)
	}
}

func TestResolveRuntimeClassifiesAnUnselectedModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	_, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	var routeErr *RuntimeBindingUnselectedError
	if !errors.As(err, &routeErr) || routeErr.Client != ClientClaude {
		t.Fatalf("unselected route error = %#v, %v", routeErr, err)
	}
}

func TestSetSelectedModelUsesTheNewModelsOnlyAdmittedProtocol(t *testing.T) {
	cfg := validConfig()
	cfg.Clients[ClientHermes] = ClientBinding{
		Model:    NewModelRef("dmx", "claude-test"),
		Enabled:  true,
		Protocol: ProtocolAnthropic,
	}

	cfg.SetSelectedModel(ClientHermes, NewModelRef("backup", "gpt-test"), "")

	binding := cfg.Clients[ClientHermes]
	if binding.Model != NewModelRef("backup", "gpt-test") {
		t.Fatalf("selected Route = %q, want backup", binding.Model)
	}
	if binding.Protocol != ProtocolResponses {
		t.Fatalf("selected protocol = %q, want %q", binding.Protocol, ProtocolResponses)
	}
	runtime, err := cfg.ResolveRuntime(ClientHermes, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Protocol != ProtocolResponses {
		t.Fatalf("runtime protocol = %q, want %q", runtime.Protocol, ProtocolResponses)
	}
}

func BenchmarkResolveRuntime(b *testing.B) {
	cfg := validConfig()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := cfg.ResolveRuntime(ClientClaude, ModelRef{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestResolveRuntimeReturnsProviderEndpointAndModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["dmx"] = Provider{Label: "DMXAPI", Endpoints: Endpoints{Responses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Models[NewModelRef("dmx", "gpt-5.6")] = testModel("GPT-5.6", "dmx", "gpt-5.6", ProtocolResponses)
	cfg.Models[NewModelRef("dmx", "claude-opus")] = testModel("Claude Opus", "dmx", "claude-opus", ProtocolAnthropic)
	cfg.SetSelectedModel(ClientCodex, NewModelRef("dmx", "gpt-5.6"), "")
	cfg.SetSelectedModel(ClientClaude, NewModelRef("dmx", "claude-opus"), "")
	got, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != NewModelRef("dmx", "gpt-5.6") || got.ProviderID != "dmx" || got.UpstreamModel != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v", got)
	}
	got, err = cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != NewModelRef("dmx", "claude-opus") || got.UpstreamModel != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestResolveRuntimeRejectsUnknownModelProviderAndEndpoint(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["dmx"] = Provider{Label: "DMXAPI", Endpoints: Endpoints{Responses: "https://dmx.test/v1"}}
	codex := NewModelRef("dmx", "gpt-test")
	cfg.Models[codex] = testModel("Codex", "dmx", "gpt-test", ProtocolResponses)
	cfg.SetSelectedModel(ClientCodex, codex, "")

	if _, err := cfg.ResolveRuntime(ClientCodex, NewModelRef("dmx", "missing")); err == nil || !strings.Contains(err.Error(), "unknown model dmx/missing") {
		t.Fatalf("unknown model error = %v", err)
	}

	orphan := cfg.Clone()
	orphanRef := NewModelRef("missing-account", "gpt-test")
	orphan.Models = map[ModelRef]Model{orphanRef: testModel("Codex", "missing-account", "gpt-test", ProtocolResponses)}
	orphan.SetSelectedModel(ClientCodex, orphanRef, "")
	_, err := orphan.ResolveRuntime(ClientCodex, ModelRef{})
	var accountErr *RuntimeModelUnknownProviderError
	if !errors.As(err, &accountErr) || accountErr.Model != orphanRef || accountErr.ProviderID != "missing-account" {
		t.Fatalf("unknown provider error = %v", err)
	}

	claudeRoute := cfg.Models[codex]
	claudeRoute.Family = "claude-test"
	claudeRoute.Protocols = map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}
	cfg.Models[NewModelRef("claude-only", "claude-test")] = claudeRoute
	_, err = cfg.ResolveRuntime(ClientClaude, NewModelRef("claude-only", "claude-test"))
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.ProviderID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}
}

func TestSelectModelsForConnectedProvidersKeepsCapabilityAndChoosesUsableModels(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["alpha"] = Provider{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", Responses: "https://alpha.test/v1"}}
	cfg.Providers["beta"] = Provider{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test", Responses: "https://beta.test/v1"}}
	cfg.Models[NewModelRef("alpha", "claude-test")] = testModel("Alpha Claude", "alpha", "claude-test", ProtocolAnthropic)
	cfg.Models[NewModelRef("alpha", "gpt-test")] = testModel("Alpha Codex", "alpha", "gpt-test", ProtocolResponses)
	cfg.Models[NewModelRef("beta", "claude-test")] = testModel("Beta Claude", "beta", "claude-test", ProtocolAnthropic)
	cfg.Models[NewModelRef("beta", "gpt-test")] = testModel("Beta Codex", "beta", "gpt-test", ProtocolResponses)
	cfg.Recommendations[ClientClaude] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("alpha", "claude-test")}}
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("alpha", "gpt-test")}}
	claudeRecommendation := cfg.Recommendations[ClientClaude]
	claudeRecommendation.Alternatives = []ClientSelection{{Model: NewModelRef("beta", "claude-test")}}
	cfg.Recommendations[ClientClaude] = claudeRecommendation
	codexRecommendation := cfg.Recommendations[ClientCodex]
	codexRecommendation.Alternatives = []ClientSelection{{Model: NewModelRef("beta", "gpt-test")}}
	cfg.Recommendations[ClientCodex] = codexRecommendation

	one, err := cfg.SelectModelsForConnectedProviders([]string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if one.SelectedModel(ClientClaude) != NewModelRef("beta", "claude-test") || one.SelectedModel(ClientCodex) != NewModelRef("beta", "gpt-test") {
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

	if _, err := cfg.SelectModelsForConnectedProviders([]string{"missing"}); err == nil || !strings.Contains(err.Error(), `unknown provider "missing"`) {
		t.Fatalf("unknown connected Account error = %v", err)
	}
}

func TestSelectModelsForConnectedProvidersPreservesRecommendedModels(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["preferred"] = Provider{Label: "Preferred", Endpoints: Endpoints{Anthropic: "https://preferred.test", Responses: "https://preferred.test/v1"}}
	cfg.Providers["connected"] = Provider{Label: "Connected", Endpoints: Endpoints{Anthropic: "https://connected.test", Responses: "https://connected.test/v1"}}
	cfg.Models[NewModelRef("preferred", "claude-fable-5")] = testModel("Preferred Claude", "preferred", "claude-fable-5", ProtocolAnthropic)
	cfg.Models[NewModelRef("preferred", "gpt-5.6-sol")] = testModel("Preferred Codex", "preferred", "gpt-5.6-sol", ProtocolResponses)
	cfg.Models[NewModelRef("connected", "claude-fable-5")] = testModel("Connected Claude", "connected", "claude-fable-5", ProtocolAnthropic)
	cfg.Models[NewModelRef("connected", "gpt-5.6-luna")] = testModel("Connected Codex Luna", "connected", "gpt-5.6-luna", ProtocolResponses)
	cfg.Models[NewModelRef("connected", "gpt-5.6-sol")] = testModel("Connected Codex Sol", "connected", "gpt-5.6-sol", ProtocolResponses)
	cfg.Recommendations[ClientClaude] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("preferred", "claude-fable-5")}}
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("preferred", "gpt-5.6-sol")}}
	claudeRecommendation := cfg.Recommendations[ClientClaude]
	claudeRecommendation.Alternatives = []ClientSelection{{Model: NewModelRef("connected", "claude-fable-5")}}
	cfg.Recommendations[ClientClaude] = claudeRecommendation
	codexRecommendation := cfg.Recommendations[ClientCodex]
	codexRecommendation.Alternatives = []ClientSelection{{Model: NewModelRef("connected", "gpt-5.6-sol")}}
	cfg.Recommendations[ClientCodex] = codexRecommendation

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != NewModelRef("connected", "gpt-5.6-sol") {
		t.Fatalf("Codex binding lost the recommended model: %#v", selected.Clients)
	}
	if selected.SelectedModel(ClientClaude) != NewModelRef("connected", "claude-fable-5") {
		t.Fatalf("Claude route = %q", selected.SelectedModel(ClientClaude))
	}
}

func TestSelectModelsForConnectedProvidersDoesNotSubstituteAnotherRecommendedModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["preferred"] = Provider{Label: "Preferred", Endpoints: Endpoints{Responses: "https://preferred.test/v1"}}
	cfg.Providers["connected"] = Provider{Label: "Connected", Endpoints: Endpoints{Responses: "https://connected.test/v1"}}
	cfg.Models[NewModelRef("preferred", "gpt-preferred")] = testModel("Preferred", "preferred", "gpt-preferred", ProtocolResponses)
	cfg.Models[NewModelRef("connected", "gpt-different")] = testModel("Different", "connected", "gpt-different", ProtocolResponses)
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("preferred", "gpt-preferred")}}

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if !selected.SelectedModel(ClientCodex).IsZero() {
		t.Fatalf("unavailable recommendation was replaced by a different model: %#v", selected.Clients)
	}
}

func TestSelectModelsPreservesUnavailableExplicitChoice(t *testing.T) {
	cfg := validConfig()
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("backup", "gpt-test")}}
	chosen, err := cfg.SelectModelsForConnectedProviders(nil)
	if err != nil || !reflect.DeepEqual(chosen.Clients, cfg.Clients) {
		t.Fatalf("unavailable explicit choices changed: %#v, %v", chosen.Clients, err)
	}
}

func TestSelectModelsPrefersUsableRecommendationOverIdentifierOrder(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[NewModelRef("team", "first-model")] = testModel("Alpha", "team", "first-model", ProtocolAnthropic)
	cfg.Models[NewModelRef("team", "recommended-model")] = testModel("Zeta", "team", "recommended-model", ProtocolAnthropic)
	cfg.Recommendations[ClientClaude] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "recommended-model")}}
	deferred, err := cfg.SelectModelsForConnectedProviders(nil)
	if err != nil || len(deferred.Clients) != 0 || deferred.RecommendedModel(ClientClaude) != NewModelRef("team", "recommended-model") {
		t.Fatalf("deferred recommendation = %#v, %v", deferred, err)
	}
	selected, err := deferred.SelectModelsForConnectedProviders([]string{"team"})
	if err != nil || selected.SelectedModel(ClientClaude) != NewModelRef("team", "recommended-model") {
		t.Fatalf("recommendation lost to identifier order: %#v, %v", selected.Clients, err)
	}
}

func TestSelectedProviderIDsReturnsUniqueStableActiveProviders(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["alpha"] = Provider{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", Responses: "https://alpha.test/v1"}}
	cfg.Providers["optional"] = Provider{Label: "Optional", Endpoints: Endpoints{Responses: "https://optional.test/v1"}}
	cfg.Models[NewModelRef("alpha", "claude-test")] = testModel("Alpha Claude", "alpha", "claude-test", ProtocolAnthropic)
	cfg.Models[NewModelRef("alpha", "gpt-test")] = testModel("Alpha Codex", "alpha", "gpt-test", ProtocolResponses)
	cfg.Models[NewModelRef("optional", "gpt-optional")] = testModel("Optional", "optional", "gpt-optional", ProtocolResponses)
	cfg.SetSelectedModel(ClientClaude, NewModelRef("alpha", "claude-test"), "")
	cfg.SetSelectedModel(ClientCodex, NewModelRef("alpha", "gpt-test"), "")

	if got := cfg.SelectedProviderIDs(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("SelectedAccountIDs() = %#v", got)
	}
}

func TestSelectModelsForConnectedProvidersSelectsACompatibleModelForAnUnselectedClient(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["claude-only"] = Provider{Label: "Claude only", Endpoints: Endpoints{Anthropic: "https://claude.test"}}
	cfg.Providers["codex"] = Provider{Label: "Codex", Endpoints: Endpoints{Responses: "https://codex.test/v1"}}
	cfg.Models[NewModelRef("claude-only", "claude-test")] = testModel("Claude", "claude-only", "claude-test", ProtocolAnthropic)
	cfg.Models[NewModelRef("codex", "gpt-test")] = testModel("Codex", "codex", "gpt-test", ProtocolResponses)
	cfg.SetSelectedModel(ClientCodex, NewModelRef("codex", "gpt-test"), "")
	cfg.Recommendations[ClientClaude] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("claude-only", "claude-test")}}

	got, err := cfg.SelectModelsForConnectedProviders([]string{"claude-only"}, ClientClaude)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(ClientClaude) != NewModelRef("claude-only", "claude-test") {
		t.Fatalf("selected bindings = %#v", got.Clients)
	}
}

func TestSelectModelsForConnectedProvidersDefaultsToRecommendedClients(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{
		Anthropic: "https://team.test", Responses: "https://team.test/v1",
	}}
	cfg.Models[NewModelRef("team", "model")] = testModel("Shared", "team", "model", ProtocolResponses)
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "model")}}

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != NewModelRef("team", "model") || !selected.Clients[ClientCodex].Enabled {
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
		Anthropic: "https://preferred.test", Responses: "https://preferred.test/v1",
	}}
	cfg.Providers["connected"] = Provider{Label: "Connected", Endpoints: Endpoints{
		Anthropic: "https://connected.test", Responses: "https://connected.test/v1",
	}}
	cfg.Models[NewModelRef("preferred", "shared-model")] = testModel("Preferred", "preferred", "shared-model", ProtocolAnthropic, ProtocolResponses)
	cfg.Models[NewModelRef("connected", "shared-model")] = testModel("Connected", "connected", "shared-model", ProtocolAnthropic, ProtocolResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{
		Primary:      ClientSelection{Model: NewModelRef("preferred", "shared-model"), Protocol: ProtocolAnthropic},
		Alternatives: []ClientSelection{{Model: NewModelRef("connected", "shared-model"), Protocol: ProtocolAnthropic}},
	}

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	binding := selected.Clients[ClientHermes]
	if binding.Model != NewModelRef("connected", "shared-model") || binding.Protocol != ProtocolAnthropic {
		t.Fatalf("Hermes binding = %#v, want connected Route with Anthropic protocol", binding)
	}
	runtime, err := selected.ResolveRuntime(ClientHermes, ModelRef{})
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
		Anthropic: "https://team.test", Responses: "https://team.test/v1",
	}}
	cfg.Models[NewModelRef("team", "recommended-model")] = testModel("Recommended", "team", "recommended-model", ProtocolAnthropic, ProtocolResponses)
	cfg.Models[NewModelRef("team", "selected-model")] = testModel("Selected", "team", "selected-model", ProtocolAnthropic, ProtocolResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "recommended-model"), Protocol: ProtocolAnthropic}}

	runtime, err := cfg.ResolveRuntime(ClientHermes, NewModelRef("team", "selected-model"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Model != NewModelRef("team", "selected-model") || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("explicit Hermes runtime = %#v", runtime)
	}
	cfg.SetSelectedModel(ClientHermes, NewModelRef("team", "selected-model"), "")
	if binding := cfg.Clients[ClientHermes]; binding.Model != NewModelRef("team", "selected-model") || binding.Protocol != ProtocolAnthropic {
		t.Fatalf("stored Hermes binding = %#v", binding)
	}
}

func TestSelectModelsForConnectedProvidersSkipsModelsWithoutTheClientEndpoint(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[NewModelRef("team", "claude-test")] = testModel("Generic", "team", "claude-test", ProtocolAnthropic)
	cfg.Models[NewModelRef("team", "gpt-test")] = testModel("Invalid Codex", "team", "gpt-test", ProtocolResponses)

	selected, err := cfg.SelectModelsForConnectedProviders([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if !selected.SelectedModel(ClientCodex).IsZero() {
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

func TestDomainErrorTextAndModelSelectionBranches(t *testing.T) {
	if got := ProtocolLabel(ProtocolAnthropic); got != "Anthropic Messages" {
		t.Fatalf("Anthropic protocol name = %q", got)
	}
	if got := ProtocolLabel(ProtocolResponses); got != "OpenAI Responses" {
		t.Fatalf("Responses protocol name = %q", got)
	}
	if got := (&RuntimeModelUnknownProviderError{Model: NewModelRef("missing", "one"), ProviderID: "missing"}).Error(); got != `model missing/one names the unknown provider "missing"` {
		t.Fatalf("provider error = %q", got)
	}
	if got := (&RuntimeMissingEndpointError{ProviderID: "one", Protocol: EndpointProtocol("future")}).Error(); !strings.Contains(got, "future endpoint") {
		t.Fatalf("endpoint error = %q", got)
	}

	cfg := NewConfig(testAdmission())
	cfg.Providers["generic"] = Provider{Label: "Generic", Endpoints: Endpoints{Anthropic: "https://one.test"}}
	cfg.Models[NewModelRef("generic", "")] = testModel("Skip", "generic", "", ProtocolAnthropic)
	cfg.Models[NewModelRef("generic", "claude-endpoint")] = testModel("Endpoint", "generic", "claude-endpoint", ProtocolAnthropic)
	for _, ref := range []ModelRef{NewModelRef("generic", ""), NewModelRef("generic", "claude-endpoint")} {
		if _, err := cfg.ResolveRuntime(ClientCodex, ref); err == nil {
			t.Fatalf("Codex resolved the Anthropic-only model %s", ref)
		}
	}
	if _, err := cfg.ResolveRuntime(ClientClaude, NewModelRef("generic", "")); err != nil {
		t.Fatalf("Claude Route without a Model: %v", err)
	}
}

// A recommendation's protocol comes with its own model and its provider's
// models; another provider's model takes none of it.
func TestSelectedModelCopiesRecommendedOptionsOnlyForItsProvider(t *testing.T) {
	cfg := NewConfig(testAdmission())
	both := Endpoints{Anthropic: "https://native.test", Responses: "https://native.test/v1"}
	cfg.Providers["native"] = Provider{Label: "Native", Endpoints: both}
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: both}
	cfg.Models[NewModelRef("native", "gpt-native")] = testModel("Native", "native", "gpt-native", ProtocolAnthropic, ProtocolResponses)
	cfg.Models[NewModelRef("team", "gpt-team")] = testModel("Team", "team", "gpt-team", ProtocolAnthropic, ProtocolResponses)
	cfg.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("native", "gpt-native"), Protocol: ProtocolAnthropic}}

	other := cfg.Clone()
	other.SetSelectedModel(ClientHermes, NewModelRef("team", "gpt-team"), "")
	if binding := other.Clients[ClientHermes]; binding.Protocol != "" {
		t.Fatalf("another provider's model inherited the recommendation's protocol: %#v", binding)
	}
	recommended := cfg.Clone()
	recommended.SetSelectedModel(ClientHermes, NewModelRef("native", "gpt-native"), "")
	if binding := recommended.Clients[ClientHermes]; binding.Protocol != ProtocolAnthropic {
		t.Fatalf("the recommended model lost its protocol: %#v", binding)
	}
}

// Retry budgets belong to the client's provider: selecting another model or
// another protocol of the provider keeps them, and selecting another
// provider's model clears them. Forwarding stays and its destination follows
// the upstream; a selection on a protocol other than Responses, which the
// forwarding cannot carry, is refused, and a model chosen in the client on
// that protocol is followed without the forwarding.
func TestSelectionKeepsRetryBudgetsWithinTheProvider(t *testing.T) {
	spec := ClientSpec{ID: "native", Label: "Native", EndpointProtocols: []EndpointProtocol{ProtocolResponses, ProtocolChat}, RetryBudgets: true}
	base := NewConfig(NewAdmission(spec))
	base.Providers["one"] = Provider{Label: "One", Endpoints: Endpoints{Responses: "https://one.test/v1", Chat: "https://one.test/chat/v1"}}
	base.Providers["two"] = Provider{Label: "Two", Endpoints: Endpoints{Responses: "https://two.test/v1"}}
	base.Models[NewModelRef("one", "first")] = testModel("First", "one", "first", ProtocolResponses, ProtocolChat)
	base.Models[NewModelRef("one", "second")] = testModel("Second", "one", "second", ProtocolResponses)
	base.Models[NewModelRef("two", "other")] = testModel("Other", "two", "other", ProtocolResponses)
	base.SetSelectedModel("native", NewModelRef("one", "first"), ProtocolResponses)
	three := uint64(3)
	binding := base.Clients["native"]
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &three}
	base.Clients["native"] = binding
	if err := base.SetForwardingRoot("native", "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, provider, model string
		protocol              EndpointProtocol
		budgets               bool
		destination           string
	}{
		{"another model on the same protocol", "one", "second", ProtocolResponses, true, "http://127.0.0.1:8792/https/one.test/v1"},
		{"the same provider on another protocol", "one", "first", ProtocolChat, true, ""},
		{"another provider", "two", "other", ProtocolResponses, false, "http://127.0.0.1:8792/https/two.test/v1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := base.Clone()
			cfg.SetSelectedModel("native", NewModelRef(test.provider, test.model), test.protocol)
			selected := cfg.Clients["native"]
			if budgets := selected.RetryPolicy != nil; budgets != test.budgets {
				t.Fatalf("retry budgets = %#v, want them kept: %t", selected.RetryPolicy, test.budgets)
			}
			if selected.ForwardingRoot == "" {
				t.Fatal("the selection dropped the forwarding")
			}
			err := cfg.Validate()
			if test.destination == "" {
				if refused, ok := errors.AsType[*ForwardingUpstreamError](err); !ok || refused.Protocol != ProtocolChat || !strings.Contains(err.Error(), "`aigw client edit native --direct`") {
					t.Fatalf("a forwarded selection on another protocol = %v", err)
				}
				followed := base.Clone()
				followed.FollowSelectedModel("native", NewModelRef(test.provider, test.model), test.protocol)
				if binding := followed.Clients["native"]; binding.ForwardingRoot != "" || binding.RetryPolicy == nil || followed.Validate() != nil {
					t.Fatalf("a followed choice on another protocol = %#v, %v", binding, followed.Validate())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if runtime, err := cfg.ResolveRuntime("native", ModelRef{}); err != nil || runtime.Endpoint != test.destination {
				t.Fatalf("the destination = %#v, %v, want %s", runtime, err, test.destination)
			}
			followed := base.Clone()
			followed.FollowSelectedModel("native", NewModelRef(test.provider, test.model), test.protocol)
			if followed.Clients["native"].ForwardingRoot == "" {
				t.Fatal("a followed choice that the forwarding carries dropped it")
			}
		})
	}
}
