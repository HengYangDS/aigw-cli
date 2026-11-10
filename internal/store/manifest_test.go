package store

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

func TestManifestExportOmitsDerivedModelLabelsAndPreservesWireCase(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["dmxapi"] = domain.Provider{
		Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
	}
	cfg.Families["gpt-6-astra"] = domain.Family{Label: "GPT-6 Astra"}
	cfg.Models["dmxapi-gpt-6-astra"] = domain.Model{
		Label: "DMXAPI · GPT-6 Astra", Provider: "dmxapi", Family: "gpt-6-astra",
		UpstreamModel: "GPT-6-Astra", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}},
	}
	cfg.Models["dmxapi-gpt-6-astra-cdx"] = domain.Model{
		Label: "DMXAPI · GPT-6 Astra · CDX", Provider: "dmxapi", Family: "gpt-6-astra",
		UpstreamModel: "GPT-6-Astra-CDX", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}},
	}
	data, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Routes map[string]map[string]any `toml:"routes"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, stored := raw.Routes["dmxapi-gpt-6-astra"]["label"]; stored {
		t.Fatalf("derived label was duplicated in export: %s", data)
	}
	if got := raw.Routes["dmxapi-gpt-6-astra-cdx"]["label"]; got != "DMXAPI · GPT-6 Astra · CDX" {
		t.Fatalf("channel override = %#v", got)
	}
	if got := raw.Routes["dmxapi-gpt-6-astra"]["upstream_model"]; got != "GPT-6-Astra" {
		t.Fatalf("provider wire spelling = %#v", got)
	}
	parsed, err := Parse(data, testAdmission())
	if err != nil || parsed.Models["dmxapi-gpt-6-astra"].Label != "" {
		t.Fatalf("parsed derived Route label was materialized: %#v, %v", parsed.Models["dmxapi-gpt-6-astra"], err)
	}
}

func TestManifestMergeTreatsOldPlainLabelAsDerivedButPreservesCustomOverride(t *testing.T) {
	current := domain.NewConfig(testAdmission())
	current.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{OpenAIResponses: "https://ucloud.test/v1"}}
	current.Families["gpt-6-sol"] = domain.Family{Label: "GPT-6 Sol"}
	route := domain.Model{Label: "UCloud · GPT-6 Sol", Provider: "ucloud", Family: "gpt-6-sol", UpstreamModel: "gpt-6-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}}}
	current.Models["ucloud-gpt-6-sol"] = route
	current.SetSelectedModel(ClientCodex, "ucloud-gpt-6-sol", "")
	incoming := domain.Manifest{
		Version:   domain.ManifestVersion,
		Providers: map[string]domain.Provider{"ucloud": current.Providers["ucloud"]},
		Families:  map[string]domain.Family{"gpt-6-sol": current.Families["gpt-6-sol"]},
		Models: map[string]domain.Model{"ucloud-gpt-6-sol": {
			Provider: route.Provider, Family: route.Family, UpstreamModel: route.UpstreamModel, Protocols: route.Protocols,
		}},
	}
	merged, err := domain.Merge(current, incoming)
	if err != nil || merged.SelectedModel(ClientCodex) != "ucloud-gpt-6-sol" || merged.Models["ucloud-gpt-6-sol"].Label != route.Label {
		t.Fatalf("derived-label merge changed the explicit Client Binding or old label: %+v, %v", merged.Clients, err)
	}
	custom := current.Clone()
	route.Label = "Personal Sol"
	custom.Models["ucloud-gpt-6-sol"] = route
	if _, err := domain.Merge(custom, incoming); err == nil || !strings.Contains(err.Error(), "conflicts with local configuration") {
		t.Fatalf("custom Route label was silently overwritten: %v", err)
	}
}

func TestManifestSeparatesFamiliesFromQualifiedProviderModels(t *testing.T) {
	manifest, err := Parse([]byte(`version = 7
[models.gpt-6-astra]
label = "GPT-6 Astra"

[accounts.ucloud]
label = "UCloud"
[accounts.ucloud.endpoints]
openai_responses = "https://example.test/v1"

[routes.ucloud-gpt-6-astra]
account = "ucloud"
model = "gpt-6-astra"
upstream_model = "gpt-6-astra"
[routes.ucloud-gpt-6-astra.interfaces]
openai_responses = ["text", "reasoning", "streaming", "tools", "continuation"]

[recommendations.codex.primary]
route = "ucloud-gpt-6-astra"
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Families["gpt-6-astra"].Label; got != "GPT-6 Astra" {
		t.Fatalf("canonical model label = %q", got)
	}
	route := manifest.Models["ucloud-gpt-6-astra"]
	if route.Provider != "ucloud" || route.Family != "gpt-6-astra" || route.UpstreamModel != "gpt-6-astra" {
		t.Fatalf("route identity = %#v", route)
	}
	wantCapabilities := []domain.Capability{domain.CapabilityText, domain.CapabilityReasoning, domain.CapabilityStreaming, domain.CapabilityTools, domain.CapabilityContinuation}
	if got := route.Protocols[domain.ProtocolOpenAIResponses]; !reflect.DeepEqual(got, wantCapabilities) {
		t.Fatalf("Responses capabilities = %#v, want %#v", got, wantCapabilities)
	}
	if got := manifest.Recommendations[ClientCodex].Primary.Model; got != "ucloud-gpt-6-astra" {
		t.Fatalf("Codex primary Route = %q", got)
	}
}

func TestManifestRetainsDeprecatedModelsButRejectsThemAsRecommendations(t *testing.T) {
	raw := []byte(`version = 7
[models.legacy]
label = "Legacy"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"

[routes.legacy]
account = "team"
model = "legacy"
lifecycle = "deprecated"
interfaces = { openai_responses = ["text"] }
`)
	manifest, err := Parse(raw, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if lifecycle := manifest.Models["legacy"].LifecycleState(); lifecycle != domain.ModelDeprecated {
		t.Fatalf("lifecycle = %q", lifecycle)
	}

	withRecommendation := bytes.Replace(raw, []byte("[accounts.team]"), []byte("[recommendations.codex.primary]\nroute = \"legacy\"\n\n[accounts.team]"), 1)
	if _, err := Parse(withRecommendation, testAdmission()); err == nil || !strings.Contains(err.Error(), "deprecated route") {
		t.Fatalf("deprecated recommendation error = %v", err)
	}
}

func TestManifestProviderIDsReturnsEveryCredentialOwnerOnce(t *testing.T) {
	incoming := domain.Manifest{
		Providers: map[string]domain.Provider{"shared": {}, "direct": {}},
		Models:    map[string]domain.Model{"alias": {Provider: "shared"}, "implicit": {}},
	}
	want := []string{"direct", "shared"}
	if got := domain.ManifestProviderIDs(incoming); !reflect.DeepEqual(got, want) {
		t.Fatalf("account names = %#v, want %#v", got, want)
	}
}

func TestCredentialDetectionDescendsIntoArrays(t *testing.T) {
	value := []any{map[string]any{"metadata": map[string]any{"api_token": "secret"}}}
	if got := findCredentialKey(value, "routes"); got != "routes.metadata.api_token" {
		t.Fatalf("credential path = %q", got)
	}
}

func TestManifestRecommendationsFollowTheAdmission(t *testing.T) {
	admission := domain.NewAdmission(append(testClientSpecs(), domain.ClientSpec{
		ID:                "synthetic",
		Label:             "Synthetic",
		EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses},
	})...)

	manifest, err := Parse([]byte(`version = 7
[recommendations.synthetic.primary]
route = "synthetic-default"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"

[routes.synthetic-default]
label = "Synthetic Default"
account = "team"
model = "synthetic-model"
interfaces = { openai_responses = [] }
`), admission)
	if err != nil {
		t.Fatalf("an admitted client was rejected: %v", err)
	}
	if manifest.Recommendations["synthetic"].Primary.Model != "synthetic-default" {
		t.Fatalf("recommended routes = %#v", manifest.Recommendations)
	}
}

func TestSyntheticProviderUsesOnlyManifestDataAcrossParseMergeAndModelResolution(t *testing.T) {
	incoming, err := Parse([]byte(`version = 7
[recommendations.codex.primary]
route = "northstar-codex"
model_provider = "northstar"
authentication = "account-token"

[accounts.northstar]
label = "Northstar"
[accounts.northstar.endpoints]
openai_responses = "https://northstar.example.test/v1"

[routes.northstar-codex]
label = "Northstar Codex"
account = "northstar"
model = "northstar-model"
interfaces = { openai_responses = [] }
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	merged, err := domain.Merge(domain.NewConfig(testAdmission()), incoming)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := merged.SelectModelsForConnectedProviders([]string{"northstar"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := selected.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ProviderID != "northstar" || runtime.Endpoint != "https://northstar.example.test/v1" || runtime.UpstreamModel != "northstar-model" || runtime.ModelProvider != "northstar" || runtime.Authentication != domain.AuthenticationProviderToken || !runtime.RequiresProviderToken() {
		t.Fatalf("runtime = %#v", runtime)
	}
}

func TestParseRejectsIncompatibleRecommendedModel(t *testing.T) {
	raw := []byte(`version = 7
[recommendations.claude.primary]
route = "codex"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
[routes.codex]
label = "Codex"
account = "team"
model = "gpt-test"
interfaces = { openai_responses = [] }
`)
	if _, err := Parse(raw, testAdmission()); err == nil {
		t.Fatal("incompatible recommended route was accepted")
	}
}

func TestVersionTwoIsRejected(t *testing.T) {
	legacy := []byte(`version = 2
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[routes.claude]
label = "Claude"
account = "team"
model = "claude-test"
`)
	if _, err := Parse(legacy, testAdmission()); err == nil || !strings.Contains(err.Error(), "expected 7") {
		t.Fatalf("v2 manifest error = %v", err)
	}
}

func TestExportRejectsModelThatCannotBeParsedBack(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Models["codex"] = testModel("Codex", "team", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedModel(ClientCodex, "codex", "")
	cfg.Clients[ClientClaude] = domain.ClientBinding{Model: "codex", Protocol: domain.ProtocolOpenAIResponses}

	if _, err := Export(cfg); err == nil {
		t.Fatal("export accepted a client-incompatible route")
	}
}

func TestParseRejectsCredentialShapedFields(t *testing.T) {
	for _, key := range []string{"token", "api_key", "password", "auth_header", "client_secret"} {
		raw := []byte("version = 7\n" + key + " = \"must-not-exist\"\n")
		_, err := Parse(raw, testAdmission())
		if err == nil || !strings.Contains(err.Error(), "credential") {
			t.Errorf("key %s: error = %v", key, err)
		}
	}
}

func TestParseRejectsNonCanonicalSchemaVersion(t *testing.T) {
	oldSchema := []byte(`version = 1
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[routes.team]
label = "Team"
purpose = "Default agent"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`)
	if _, err := Parse(oldSchema, testAdmission()); err == nil ||
		!strings.Contains(err.Error(), "unsupported configuration manifest version 1") ||
		!strings.Contains(err.Error(), "does not reinterpret schema versions") {
		t.Fatalf("version 1 parse error = %v", err)
	}
	current := []byte(strings.Replace(string(oldSchema), "version = 1", "version = 7", 1))
	parsed, err := Parse(current, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version != 7 || parsed.Models["team"].Purpose != "Default agent" {
		t.Fatalf("parsed manifest = %#v", parsed)
	}
}

func TestParseRejectsModelOwnedEndpointResidue(t *testing.T) {
	raw := []byte(`version = 7

[routes.team]
label = "Team Gateway"
account = "team"
model = "claude-team"

[routes.team.endpoints]
openai_responses = "https://gateway.test/v1"
`)
	if _, err := Parse(raw, testAdmission()); err == nil {
		t.Fatalf("legacyRoute endpoint error = %v", err)
	}
}

func TestParseRejectsMalformedTOML(t *testing.T) {
	if _, err := Parse([]byte("version = [1, 2\n"), testAdmission()); err == nil || !strings.Contains(err.Error(), "parse configuration manifest") {
		t.Fatalf("malformed TOML error = %v", err)
	}
}

// A team manifest may declare providers without Routes, as a configuration
// may hold providers without models.
func TestParseAcceptsAManifestWithProvidersOnly(t *testing.T) {
	raw := []byte(`version = 7
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
`)
	manifest, err := Parse(raw, testAdmission())
	if err != nil || len(manifest.Providers) != 1 || len(manifest.Models) != 0 {
		t.Fatalf("a manifest with providers only = %+v, %v", manifest, err)
	}
}

func TestParseRejectsRemovedRecommendedDefault(t *testing.T) {
	raw := []byte(`version = 7
recommended_default = "missing"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[routes.team]
label = "Team"
account = "team"
model = "claude-team"
`)
	if _, err := Parse(raw, testAdmission()); err == nil || !strings.Contains(err.Error(), "fields in the document are missing") {
		t.Fatalf("removed recommended_default error = %v", err)
	}
}

func TestParseInitializesMissingProvidersAndDefaultsToFirstModel(t *testing.T) {
	raw := []byte(`version = 7
[routes.solo]
label = "Solo"
account = "missing"
`)
	if _, err := Parse(raw, testAdmission()); err == nil || !strings.Contains(err.Error(), "invalid configuration manifest") {
		t.Fatalf("account-less manifest error = %v", err)
	}
}

func TestParseRejectsRecommendedModelWithUnsupportedClient(t *testing.T) {
	raw := []byte(`version = 7
[recommendations.gemini.primary]
route = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[routes.team]
label = "Team"
account = "team"
model = "claude-team"
`)
	if _, err := Parse(raw, testAdmission()); err == nil || !strings.Contains(err.Error(), "unsupported client") {
		t.Fatalf("unsupported recommended route client error = %v", err)
	}
}

func TestParseRejectsDeeplyNestedCredentialShapedFields(t *testing.T) {
	raw := []byte(`version = 7
[wrapper]
password = "leak"
[[entries]]
token = "leak"
`)
	if _, err := Parse(raw, testAdmission()); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("nested credential field error = %v", err)
	}
}

func TestExportRejectsInvalidConfiguration(t *testing.T) {
	if _, err := Export(domain.Config{}); err == nil {
		t.Fatal("export accepted an invalid configuration")
	}
}

func TestExportOmitsSecretsAndAdaptersAndPublishesDeclaredRecommendations(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["team"] = testModel("Team", "team", "claude-team", domain.ProtocolAnthropic)
	cfg.Recommendations[ClientClaude] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: "team"}}
	cfg.SetSelectedModel(ClientClaude, "team", "")
	binding := cfg.Clients[ClientClaude]
	binding.Enabled = true
	binding.Executable = "/personal/claude"
	cfg.Clients[ClientClaude] = binding
	data, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"adapters", "overrides", "/personal/claude", "token", "secret"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("export contains %q:\n%s", forbidden, text)
		}
	}
	if strings.Contains(text, "recommended_default") {
		t.Fatalf("export retained removed recommended_default:\n%s", text)
	}
	if !strings.Contains(text, "version = 7") || !strings.Contains(text, "recommendations") || !strings.Contains(text, "claude") {
		t.Fatalf("new config export must use manifest v7 with client recommendations:\n%s", text)
	}
}

func TestExportIsCanonicalTypedManifestProjection(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["team"] = domain.Provider{
		Label:     "Team",
		Endpoints: domain.Endpoints{Anthropic: "https://team.example.test"},
	}
	cfg.Models["team"] = domain.Model{
		Label:    "Team Claude",
		Provider: "team",
		Family:   "claude-team",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {},
		},
	}
	cfg.SetSelectedModel(ClientClaude, "team", "")

	first, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Parse(first, testAdmission())
	if err != nil {
		t.Fatalf("parse exported manifest: %v", err)
	}
	projected := domain.NewConfig(testAdmission())
	projected.Providers = manifest.Providers
	projected.Models = manifest.Models
	projected.Recommendations = manifest.Recommendations
	second, err := Export(projected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("manifest projection is not byte-stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
