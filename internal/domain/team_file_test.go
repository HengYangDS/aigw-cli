package domain

import (
	"slices"
	"strings"
	"testing"
)

func teamFileConfig(t *testing.T) Config {
	t.Helper()
	cfg := NewConfig(testAdmission())
	if err := cfg.AppendProvider("team", Provider{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AppendFamily("opus", Family{Label: "Opus"}); err != nil {
		t.Fatal(err)
	}
	derived := anthropicModel("team", "opus-1")
	derived.Family, derived.Label = "opus", "Team · Opus"
	explicit := anthropicModel("team", "opus-cdx")
	explicit.Family, explicit.Label = "opus", "Team · Opus · CDX"
	for _, model := range []Model{derived, explicit} {
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	cfg.SetSelectedModel(ClientClaude, derived.Ref(), "")
	cfg.Recommendations[ClientClaude] = ClientRecommendation{Primary: ClientSelection{Model: explicit.Ref()}}
	return cfg
}

// TestExportTeamFileKeepsTheTeamsChoicesOnly exports providers, models,
// families and recommendations in display order, without clients and
// without a label that the provider and family derive, and leaves the
// configuration as it was.
func TestExportTeamFileKeepsTheTeamsChoicesOnly(t *testing.T) {
	cfg := teamFileConfig(t)
	exported, err := ExportTeamFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	derived, explicit := NewModelRef("team", "opus-1"), NewModelRef("team", "opus-cdx")
	if len(exported.Clients) != 0 || exported.Models[derived].Label != "" || exported.Models[explicit].Label != "Team · Opus · CDX" ||
		exported.RecommendedModel(ClientClaude) != explicit || !slices.Equal(exported.ModelRefs(), []ModelRef{derived, explicit}) {
		t.Fatalf("exported = %+v", exported)
	}
	if cfg.SelectedModel(ClientClaude) != derived || cfg.Models[derived].Label != "Team · Opus" {
		t.Fatal("the export changed the configuration")
	}
	broken := cfg.Clone()
	broken.Models[NewModelRef("ghost", "m")] = anthropicModel("ghost", "m")
	if _, err := ExportTeamFile(broken); err == nil || !strings.Contains(err.Error(), `names the unknown provider "ghost"`) {
		t.Fatalf("an invalid configuration was exported: %v", err)
	}
}

// TestAdmitTeamFileRefusesAnInvalidFile names the file and its fault.
func TestAdmitTeamFileRefusesAnInvalidFile(t *testing.T) {
	file := teamFileConfig(t)
	file.Clients = map[string]ClientBinding{}
	model := file.Models[NewModelRef("team", "opus-1")]
	model.Protocols = map[EndpointProtocol][]Capability{"carrier-pigeon": {}}
	file.Models[model.Ref()] = model
	if _, err := file.AdmitTeamFile(); err == nil || !strings.HasPrefix(err.Error(), "invalid team file: ") {
		t.Fatalf("an invalid team file = %v", err)
	}
	for client, recommendation := range map[string]ClientRecommendation{
		"unknown-client": {Primary: ClientSelection{Model: model.Ref()}},
		ClientClaude:     {},
	} {
		file := teamFileConfig(t)
		file.Recommendations = map[string]ClientRecommendation{client: recommendation}
		if _, err := file.AdmitTeamFile(); err == nil {
			t.Errorf("the recommendation of %s = %+v was admitted", client, recommendation)
		}
	}
}

// TestRemoveRecommendedModelPromotesTheFirstAlternative drops a model from
// every recommendation: a primary that goes makes the first alternative
// primary, and a recommendation with no choice left goes.
func TestRemoveRecommendedModelPromotesTheFirstAlternative(t *testing.T) {
	gone, kept, other := NewModelRef("p", "gone"), NewModelRef("p", "kept"), NewModelRef("p", "other")
	cfg := Config{Recommendations: map[string]ClientRecommendation{
		"promoted":   {Primary: ClientSelection{Model: gone}, Alternatives: []ClientSelection{{Model: kept}, {Model: other}}},
		"trimmed":    {Primary: ClientSelection{Model: kept}, Alternatives: []ClientSelection{{Model: gone}}},
		"dropped":    {Primary: ClientSelection{Model: gone}},
		"untouched":  {Primary: ClientSelection{Model: other}, Alternatives: []ClientSelection{{Model: kept}}},
		"untouched2": {Primary: ClientSelection{Model: kept}},
	}}
	if changed := cfg.RemoveRecommendedModel(gone); !slices.Equal(changed, []string{"dropped", "promoted", "trimmed"}) {
		t.Fatalf("changed = %v", changed)
	}
	if promoted := cfg.Recommendations["promoted"]; promoted.Primary.Model != kept || len(promoted.Alternatives) != 1 || promoted.Alternatives[0].Model != other {
		t.Fatalf("promoted = %+v", promoted)
	}
	if trimmed := cfg.Recommendations["trimmed"]; trimmed.Primary.Model != kept || len(trimmed.Alternatives) != 0 {
		t.Fatalf("trimmed = %+v", trimmed)
	}
	if _, kept := cfg.Recommendations["dropped"]; kept || len(cfg.Recommendations) != 4 {
		t.Fatalf("recommendations = %+v", cfg.Recommendations)
	}
}

// TestModelLabelsAndProtocolClients derives a label from the provider and
// the family, falls back to IDs, and names the clients that can use a model.
func TestModelLabelsAndProtocolClients(t *testing.T) {
	cfg := teamFileConfig(t)
	own := anthropicModel("team", "own-model")
	cfg.Models[own.Ref()] = own
	cfg.Providers["bare"] = Provider{Endpoints: Endpoints{Anthropic: "https://bare.test"}}
	bare := anthropicModel("bare", "m")
	cfg.Models[bare.Ref()] = bare
	for ref, want := range map[ModelRef]string{
		NewModelRef("team", "opus-1"):   "Team · Opus",
		NewModelRef("team", "opus-cdx"): "Team · Opus · CDX",
		own.Ref():                       "Team · own-model",
		bare.Ref():                      "bare · m",
		NewModelRef("team", "missing"):  "team/missing",
	} {
		if got := cfg.ModelLabel(ref); got != want {
			t.Errorf("ModelLabel(%s) = %q, want %q", ref, got, want)
		}
	}
	if clients, err := cfg.ProtocolClientIDs(own.Ref()); err != nil || !slices.Equal(clients, []string{ClientClaude, ClientClaudeDesktop, ClientHermes}) {
		t.Fatalf("clients of %s = %v, %v", own.Ref(), clients, err)
	}
	if _, err := cfg.ProtocolClientIDs(NewModelRef("team", "missing")); err == nil {
		t.Fatal("an unknown model has clients")
	}
	cfg.Models[NewModelRef("ghost", "m")] = anthropicModel("ghost", "m")
	if _, err := cfg.ProtocolClientIDs(NewModelRef("ghost", "m")); err == nil || !strings.Contains(err.Error(), `names the unknown provider "ghost"`) {
		t.Fatalf("a model of an unknown provider = %v", err)
	}
	withText := anthropicModel("team", "with-text")
	withText.Protocols[ProtocolAnthropic] = []Capability{CapabilityText}
	if !withText.Declares(ProtocolAnthropic, CapabilityText) || withText.Declares(ProtocolResponses, CapabilityText) || own.Declares(ProtocolAnthropic, CapabilityText) {
		t.Fatal("Declares does not follow the model's capabilities")
	}
}

// TestSelectionValidationNamesTheField refuses a recommendation's model
// provider and authentication that its client cannot use.
func TestSelectionValidationNamesTheField(t *testing.T) {
	ref := NewModelRef("gateway", "gpt-test")
	for _, test := range []struct {
		selection ClientSelection
		want      string
	}{
		{ClientSelection{Model: ref, ModelProvider: "Bad Provider"}, `invalid model provider "Bad Provider"`},
		{ClientSelection{Model: ref, Authentication: AuthenticationClientNative}, "client-native authentication requires model_provider"},
		{ClientSelection{Model: ref, Authentication: "password"}, `invalid authentication "password"`},
	} {
		cfg := NewConfig(testAdmission())
		cfg.Providers["gateway"] = Provider{Label: "Gateway", Endpoints: Endpoints{Responses: "https://gateway.test/v1"}}
		cfg.Models[ref] = Model{Provider: "gateway", ID: "gpt-test", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}}
		cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: test.selection}
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%+v = %v, want %q", test.selection, err, test.want)
		}
	}
	cfg := NewConfig(testAdmission())
	cfg.Clients[ClientCodex] = ClientBinding{CredentialCommand: "relative/aigw"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "credential_command must be one absolute executable path") {
		t.Fatalf("a relative credential command = %v", err)
	}
	cfg = NewConfig(testAdmission())
	cfg.Families["Bad Family"] = Family{Label: "Bad"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), `"Bad Family"`) {
		t.Fatalf("an invalid family ID = %v", err)
	}
}
