package store

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// teamFileText is a team file with a channel variant, a model that is its own
// family, a deprecated model and recommendations.
const teamFileText = `version = 8

[[providers]]
id = "dmxapi"
label = "DMXAPI"
endpoints = {responses = "https://dmx.test/v1"}

[[providers.models]]
id = "GPT-6-Astra"
family = "gpt-6-astra"
protocols = {responses = ["text"]}

[[providers.models]]
id = "GPT-6-Astra-CDX"
family = "gpt-6-astra"
label = "DMXAPI · GPT-6 Astra · CDX"
protocols = {responses = ["text"]}

[[providers.models]]
id = "LongCat-2.5"
protocols = {responses = []}

[[providers.models]]
id = "legacy"
protocols = {responses = []}
deprecated = true

[[families]]
id = "gpt-6-astra"
label = "GPT-6 Astra"

[recommendations.codex]
primary = {model = "dmxapi/GPT-6-Astra-CDX", protocol = "responses"}
alternatives = [{model = "dmxapi/GPT-6-Astra"}]
`

func TestTeamFileParsesProvidersModelsFamiliesAndRecommendations(t *testing.T) {
	file, err := ParseTeamFile([]byte(teamFileText), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	variant := file.Models[dmxapiModel("GPT-6-Astra-CDX")]
	if variant.Provider != "dmxapi" || variant.ID != "GPT-6-Astra-CDX" || variant.Family != "gpt-6-astra" ||
		!variant.Declares(domain.ProtocolResponses, domain.CapabilityText) {
		t.Fatalf("channel variant = %#v", variant)
	}
	own := dmxapiModel("LongCat-2.5")
	if model := file.Models[own]; model.Family != "" || model.FamilyID() != "LongCat-2.5" {
		t.Fatalf("a model without a family = %#v", model)
	}
	if got := file.ModelLabel(own); got != "DMXAPI · LongCat-2.5" {
		t.Fatalf("own family label = %q", got)
	}
	if !file.Models[dmxapiModel("legacy")].Deprecated {
		t.Fatal("deprecated was not read")
	}
	codex := file.Recommendations[ClientCodex]
	if codex.Primary.Model != dmxapiModel("GPT-6-Astra-CDX") || codex.Primary.Protocol != domain.ProtocolResponses ||
		len(codex.Alternatives) != 1 || codex.Alternatives[0].Model != dmxapiModel("GPT-6-Astra") {
		t.Fatalf("recommendation = %#v", codex)
	}
	if len(file.Clients) != 0 {
		t.Fatalf("a team file set clients: %#v", file.Clients)
	}
}

func TestTeamFileExportOmitsDerivedLabelsAndRoundTrips(t *testing.T) {
	file, err := ParseTeamFile([]byte(teamFileText), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), file, false)
	if err != nil {
		t.Fatal(err)
	}
	model := cfg.Models[dmxapiModel("GPT-6-Astra")]
	model.Label = "DMXAPI · GPT-6 Astra"
	cfg.Models[dmxapiModel("GPT-6-Astra")] = model
	cfg.SetSelectedModel(ClientCodex, dmxapiModel("GPT-6-Astra"), "")
	exported, err := ExportTeamFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(exported)
	if strings.Contains(text, "label = 'DMXAPI · GPT-6 Astra'\n") || !strings.Contains(text, "label = 'DMXAPI · GPT-6 Astra · CDX'") {
		t.Fatalf("export kept a derived label or lost a channel label:\n%s", text)
	}
	if strings.Contains(text, "[clients") || !strings.HasPrefix(text, "version = 8\n") {
		t.Fatalf("export is not a version 8 team file without clients:\n%s", text)
	}
	again, err := ParseTeamFile(exported, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	merged, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), again, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExportTeamFile(merged)
	if err != nil || !bytes.Equal(exported, second) {
		t.Fatalf("export is not stable through setup --from:\nfirst:\n%s\nsecond:\n%s\n%v", exported, second, err)
	}
}

func TestTeamFileRefusesADeprecatedRecommendation(t *testing.T) {
	text := strings.Replace(teamFileText, `alternatives = [{model = "dmxapi/GPT-6-Astra"}]`, `alternatives = [{model = "dmxapi/legacy"}]`, 1)
	if _, err := ParseTeamFile([]byte(text), testAdmission()); err == nil || !strings.Contains(err.Error(), "deprecated model dmxapi/legacy") {
		t.Fatalf("a deprecated recommendation = %v", err)
	}
}

func TestCredentialDetectionDescendsIntoArrays(t *testing.T) {
	value := []any{map[string]any{"metadata": map[string]any{"api_token": "secret"}}}
	if got := findCredentialKey(value, "providers"); got != "providers.metadata.api_token" {
		t.Fatalf("credential path = %q", got)
	}
}

func TestTeamFileRecommendationsFollowTheAdmission(t *testing.T) {
	admission := domain.NewAdmission(append(testClientSpecs(), domain.ClientSpec{
		ID:                "synthetic",
		Label:             "Synthetic",
		EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolResponses},
	})...)
	text := strings.Replace(teamFileText, "[recommendations.codex]", "[recommendations.synthetic]", 1)
	file, err := ParseTeamFile([]byte(text), admission)
	if err != nil {
		t.Fatalf("an admitted client was rejected: %v", err)
	}
	if file.Recommendations["synthetic"].Primary.Model != dmxapiModel("GPT-6-Astra-CDX") {
		t.Fatalf("recommendations = %#v", file.Recommendations)
	}
	if _, err := ParseTeamFile([]byte(text), testAdmission()); err == nil || !strings.Contains(err.Error(), `unsupported client "synthetic"`) {
		t.Fatalf("a recommendation for a client the admission lacks = %v", err)
	}
}

func TestTeamFileSyntheticProviderUsesOnlyFileData(t *testing.T) {
	incoming, err := ParseTeamFile([]byte(`version = 8

[[providers]]
id = "northstar"
label = "Northstar"
endpoints = {responses = "https://northstar.example.test/v1"}

[[providers.models]]
id = "northstar-model"
label = "Northstar Codex"
protocols = {responses = []}

[recommendations.codex]
primary = {model = "northstar/northstar-model", model_provider = "northstar", authentication = "provider-token"}
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	merged, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), incoming, false)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := merged.SelectModelsForConnectedProviders([]string{"northstar"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := selected.ResolveRuntime(ClientCodex, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ProviderID != "northstar" || runtime.Endpoint != "https://northstar.example.test/v1" || runtime.UpstreamModel != "northstar-model" ||
		runtime.ModelProvider != "northstar" || runtime.Authentication != domain.AuthenticationProviderToken || !runtime.RequiresProviderToken() {
		t.Fatalf("runtime = %#v", runtime)
	}
}

// TestTeamFileRefusals refuses, before anything is applied, a file that is not
// TOML, carries a credential field, sets clients, has another version, uses a
// field that version 8 does not define, repeats a provider, model or family,
// names a model without its provider, or recommends a model that its client
// cannot use.
func TestTeamFileRefusals(t *testing.T) {
	for _, test := range []struct {
		name, text, want string
	}{
		{"not TOML", "version = [1, 2\n", "parse the team file"},
		{"a version that is not a number", strings.Replace(teamFileText, "version = 8", "version = \"eight\"", 1), "parse the team file"},
		{"credential", "version = 8\napi_key = \"must-not-exist\"\n", `credential field "api_key"`},
		{"nested credential", strings.Replace(teamFileText, `id = "legacy"`, "id = \"legacy\"\nauth_header = \"x\"", 1), `credential field "providers.models.auth_header"`},
		{"clients", teamFileText + "\n[clients.codex]\nenabled = true\n", "[clients] section"},
		{"version 7", strings.Replace(teamFileText, "version = 8", "version = 7", 1), "the team file has version 7, and this AIGW reads version 8"},
		{"version 9", strings.Replace(teamFileText, "version = 8", "version = 9", 1), "the team file has version 9"},
		{"retired order", strings.Replace(teamFileText, `label = "DMXAPI"`, "label = \"DMXAPI\"\norder = 10", 1), "does not define the field providers.order"},
		{"retired purpose", strings.Replace(teamFileText, `id = "legacy"`, "id = \"legacy\"\npurpose = \"x\"", 1), "does not define the field providers.models.purpose"},
		{"retired lifecycle", strings.Replace(teamFileText, "deprecated = true", "lifecycle = \"deprecated\"", 1), "does not define the field providers.models.lifecycle"},
		{"old endpoint name", strings.Replace(teamFileText, `endpoints = {responses = "https://dmx.test/v1"}`, `endpoints = {openai_responses = "https://dmx.test/v1"}`, 1), "does not define the field"},
		{"repeated provider", teamFileText + "\n[[providers]]\nid = \"dmxapi\"\nlabel = \"Again\"\nendpoints = {responses = \"https://dmx.test/v1\"}\n", `provider "dmxapi" is defined twice`},
		{"repeated model", strings.Replace(teamFileText, `id = "legacy"`, `id = "LongCat-2.5"`, 1), `provider "dmxapi" defines model "LongCat-2.5" twice`},
		{"repeated family", teamFileText + "\n[[families]]\nid = \"gpt-6-astra\"\nlabel = \"Again\"\n", `family "gpt-6-astra" is defined twice`},
		{"model without provider", strings.Replace(teamFileText, `{model = "dmxapi/GPT-6-Astra"}`, `{model = "GPT-6-Astra"}`, 1), "has no provider"},
		{"incompatible recommendation", strings.Replace(teamFileText, "[recommendations.codex]", "[recommendations.claude]", 1), "client recommendation"},
		{"unknown protocol", strings.Replace(teamFileText, `protocols = {responses = []}`, `protocols = {messages = []}`, 1), `unknown protocol "messages"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTeamFile([]byte(test.text), testAdmission())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseTeamFile = %v, want %q", err, test.want)
			}
		})
	}
	_, err := ParseTeamFile([]byte(strings.Replace(teamFileText, "version = 8", "version = 7", 1)), testAdmission())
	if version, ok := errors.AsType[*domain.UnsupportedTeamFileVersionError](err); !ok || version.Version != 7 {
		t.Fatalf("version refusal = %v", err)
	}
}

// A team file may declare providers without models, as a configuration may
// hold providers without models.
func TestTeamFileWithProvidersOnly(t *testing.T) {
	file, err := ParseTeamFile([]byte("version = 8\n\n[[providers]]\nid = \"team\"\nlabel = \"Team\"\nendpoints = {anthropic = \"https://team.test\"}\n"), testAdmission())
	if err != nil || len(file.Providers) != 1 || len(file.Models) != 0 {
		t.Fatalf("a team file with providers only = %+v, %v", file, err)
	}
}

func TestExportRefusesAConfigurationThatDoesNotValidate(t *testing.T) {
	if _, err := ExportTeamFile(domain.Config{}); err == nil {
		t.Fatal("export accepted an invalid configuration")
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Responses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "gpt-test")] = testModel("Codex", "team", "gpt-test", domain.ProtocolResponses)
	cfg.Clients[ClientClaude] = domain.ClientBinding{Model: domain.NewModelRef("team", "gpt-test"), Protocol: domain.ProtocolResponses}
	if _, err := ExportTeamFile(cfg); err == nil {
		t.Fatal("export accepted a client that cannot use its model")
	}
}

func TestExportOmitsClientsAndCredentials(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models[domain.NewModelRef("team", "claude-team")] = testModel("Team", "team", "claude-team", domain.ProtocolAnthropic)
	cfg.Recommendations[ClientClaude] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("team", "claude-team")}}
	cfg.SetSelectedModel(ClientClaude, domain.NewModelRef("team", "claude-team"), "")
	binding := cfg.Clients[ClientClaude]
	binding.Enabled = true
	binding.Executable = "/personal/claude"
	cfg.Clients[ClientClaude] = binding
	data, err := ExportTeamFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"clients", "/personal/claude", "token", "secret"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("export contains %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, "[recommendations.claude]\nprimary = { model = 'team/claude-team' }") {
		t.Fatalf("export lost the declared recommendation:\n%s", text)
	}
}
