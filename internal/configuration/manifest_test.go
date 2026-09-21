package configuration

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestManifestAccountNamesReturnsEveryCredentialOwnerOnce(t *testing.T) {
	incoming := Manifest{
		Accounts: map[string]Account{"shared": {}, "direct": {}},
		Profiles: map[string]Profile{"alias": {Account: "shared"}, "implicit": {}},
	}
	want := []string{"direct", "shared"}
	if got := ManifestAccountNames(incoming); !reflect.DeepEqual(got, want) {
		t.Fatalf("account names = %#v, want %#v", got, want)
	}
}

func TestCredentialDetectionDescendsIntoArrays(t *testing.T) {
	value := []any{map[string]any{"metadata": map[string]any{"api_token": "secret"}}}
	if got := findCredentialKey(value, "profiles"); got != "profiles.metadata.api_token" {
		t.Fatalf("credential path = %q", got)
	}
}

func TestManifestAdmissionIsDerivedFromClientRegistry(t *testing.T) {
	previous := admittedClientSpecs
	admittedClientSpecs = append(AdmittedClientSpecs(), ClientSpec{
		ID:                "synthetic",
		Label:             "Synthetic",
		EndpointProtocols: []EndpointProtocol{ProtocolOpenAIResponses},
	})
	defer func() { admittedClientSpecs = previous }()

	manifest, err := Parse([]byte(`version = 5
[recommendations.synthetic]
profile = "synthetic-default"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"

[profiles.synthetic-default]
label = "Synthetic Default"
account = "team"
model = "synthetic-model"
`))
	if err != nil {
		t.Fatalf("registry-admitted client was rejected: %v", err)
	}
	if manifest.Recommendations["synthetic"].Profile != "synthetic-default" {
		t.Fatalf("recommended routes = %#v", manifest.Recommendations)
	}
}

func TestSyntheticProviderUsesOnlyManifestDataAcrossParseMergeAndRouteResolution(t *testing.T) {
	incoming, err := Parse([]byte(`version = 5
[recommendations.codex]
profile = "northstar-codex"
model_provider = "northstar"
authentication = "account-token"

[accounts.northstar]
label = "Northstar"
[accounts.northstar.endpoints]
openai_responses = "https://northstar.example.test/v1"

[profiles.northstar-codex]
label = "Northstar Codex"
account = "northstar"
model = "northstar-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := Merge(NewConfig(), incoming)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := merged.SelectProfilesForConnectedAccounts([]string{"northstar"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := selected.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.AccountID != "northstar" || runtime.Endpoint != "https://northstar.example.test/v1" || runtime.Model != "northstar-model" || runtime.ModelProvider != "northstar" || runtime.Authentication != AuthenticationAccountToken || !runtime.RequiresAccountToken() {
		t.Fatalf("runtime = %#v", runtime)
	}
}

func TestParseRejectsIncompatibleRecommendedRoute(t *testing.T) {
	raw := []byte(`version = 5
[recommendations.claude]
profile = "codex"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
[profiles.codex]
label = "Codex"
account = "team"
model = "gpt-test"
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("incompatible recommended route was accepted")
	}
}

func TestVersionTwoIsRejected(t *testing.T) {
	legacy := []byte(`version = 2
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.claude]
label = "Claude"
account = "team"
model = "claude-test"
`)
	if _, err := Parse(legacy); err == nil || !strings.Contains(err.Error(), "expected 5") {
		t.Fatalf("v2 manifest error = %v", err)
	}
}

func TestExportRejectsRouteThatCannotBeParsedBack(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "team", Model: "gpt-test"}
	cfg.SetSelectedProfile(ClientCodex, "codex")
	cfg.Clients[ClientClaude] = ClientBinding{Profile: "codex", Protocol: ProtocolOpenAIResponses}

	if _, err := Export(cfg); err == nil {
		t.Fatal("export accepted a client-incompatible route")
	}
}

func TestParseRejectsProfileWithoutItsClientProtocol(t *testing.T) {
	_, err := Parse([]byte(`version = 5

[recommendations.codex]
profile = "codex"

[accounts.gateway]
label = "Gateway"
[accounts.gateway.endpoints]
anthropic = "https://gateway.test"

[profiles.codex]
label = "Codex"
account = "gateway"
model = "model"
`))
	var missing *RuntimeMissingEndpointError
	if !errors.As(err, &missing) || missing.AccountID != "gateway" || missing.Protocol != ProtocolOpenAIResponses {
		t.Fatalf("manifest protocol validation = %v", err)
	}
}

func TestParseRejectsCredentialShapedFields(t *testing.T) {
	for _, key := range []string{"token", "api_key", "password", "auth_header", "client_secret"} {
		raw := []byte("version = 5\n" + key + " = \"must-not-exist\"\n")
		_, err := Parse(raw)
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
[profiles.team]
label = "Team"
purpose = "Default agent"
account = "team"
model = "claude-team"
`)
	if _, err := Parse(oldSchema); err == nil ||
		!strings.Contains(err.Error(), "unsupported configuration manifest version 1") ||
		!strings.Contains(err.Error(), "does not reinterpret schema versions") {
		t.Fatalf("version 1 parse error = %v", err)
	}
	current := []byte(strings.Replace(string(oldSchema), "version = 1", "version = 5", 1))
	parsed, err := Parse(current)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version != 5 || parsed.Profiles["team"].Purpose != "Default agent" {
		t.Fatalf("parsed manifest = %#v", parsed)
	}
}

func TestParseRejectsProfileOwnedEndpointResidue(t *testing.T) {
	raw := []byte(`version = 5

[profiles.team]
label = "Team Gateway"
account = "team"
model = "claude-team"

[profiles.team.endpoints]
openai_responses = "https://gateway.test/v1"
`)
	if _, err := Parse(raw); err == nil {
		t.Fatalf("legacyProfile endpoint error = %v", err)
	}
}

func TestParseRejectsMalformedTOML(t *testing.T) {
	if _, err := Parse([]byte("version = [1, 2\n")); err == nil || !strings.Contains(err.Error(), "parse configuration manifest") {
		t.Fatalf("malformed TOML error = %v", err)
	}
}

func TestParseRejectsManifestWithoutAnyProfile(t *testing.T) {
	raw := []byte(`version = 5
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "at least one profile") {
		t.Fatalf("no-profile error = %v", err)
	}
}

func TestParseRejectsRemovedRecommendedDefault(t *testing.T) {
	raw := []byte(`version = 5
recommended_default = "missing"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
model = "claude-team"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "fields in the document are missing") {
		t.Fatalf("removed recommended_default error = %v", err)
	}
}

func TestParseInitializesMissingAccountsAndDefaultsToFirstProfile(t *testing.T) {
	raw := []byte(`version = 5
[profiles.solo]
label = "Solo"
account = "missing"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "invalid configuration manifest") {
		t.Fatalf("account-less manifest error = %v", err)
	}
}

func TestParseRejectsRecommendedRouteWithUnsupportedClient(t *testing.T) {
	raw := []byte(`version = 5
[recommendations.gemini]
profile = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
model = "claude-team"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "unsupported client") {
		t.Fatalf("unsupported recommended route client error = %v", err)
	}
}

func TestParseRejectsRecommendedRouteReferencingUnknownProfile(t *testing.T) {
	raw := []byte(`version = 5
[recommendations.claude]
profile = "missing"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
model = "claude-team"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "references unknown profile") {
		t.Fatalf("unknown recommended route profile error = %v", err)
	}
}

func TestParseRejectsDeeplyNestedCredentialShapedFields(t *testing.T) {
	raw := []byte(`version = 5
[wrapper]
password = "leak"
[[entries]]
token = "leak"
`)
	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("nested credential field error = %v", err)
	}
}

func TestExportRejectsInvalidConfiguration(t *testing.T) {
	if _, err := Export(Config{}); err == nil {
		t.Fatal("export accepted an invalid configuration")
	}
}

func TestExportOmitsSecretsAndAdaptersAndPublishesRouteRecommendations(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Profiles["team"] = Profile{Label: "Team", Account: "team", Model: "claude-team"}
	cfg.SetSelectedProfile(ClientClaude, "team")
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
	if !strings.Contains(text, "version = 5") || !strings.Contains(text, "recommendations") || !strings.Contains(text, "claude") {
		t.Fatalf("new config export must use manifest v5 with client recommendations:\n%s", text)
	}
}

func TestExportIsCanonicalTypedManifestProjection(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label:     "Team",
		Endpoints: Endpoints{Anthropic: "https://team.example.test"},
	}
	cfg.Profiles["team"] = Profile{
		Label:   "Team Claude",
		Account: "team",
		Model:   "claude-team",
	}
	cfg.SetSelectedProfile(ClientClaude, "team")

	first, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Parse(first)
	if err != nil {
		t.Fatalf("parse exported manifest: %v", err)
	}
	projected := NewConfig()
	projected.Accounts = manifest.Accounts
	projected.Profiles = manifest.Profiles
	projected.Recommendations = manifest.Recommendations
	second, err := Export(projected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("manifest projection is not byte-stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
