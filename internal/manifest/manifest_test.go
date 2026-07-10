package manifest_test

import (
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
)

func TestParseTeamManifestAndMergePreservesPersonalState(t *testing.T) {
	raw := []byte(`version = 1
recommended_default = "team"

[profiles.team]
label = "Team Gateway"

[profiles.team.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"
`)
	m, err := manifest.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/personal/claude"}
	cfg.Routes.Overrides[domain.ClientClaude] = "personal"
	cfg.Profiles["personal"] = domain.Profile{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	cfg.Routes.Default = "personal"
	got, err := manifest.Merge(cfg, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "personal" || got.Routes.Overrides[domain.ClientClaude] != "personal" {
		t.Fatalf("personal routes changed: %#v", got.Routes)
	}
	if got.Adapters[domain.ClientClaude].Executable != "/personal/claude" {
		t.Fatalf("personal adapter changed: %#v", got.Adapters)
	}
	if got.Profiles["team"].Label != "Team Gateway" {
		t.Fatalf("team profile missing: %#v", got.Profiles)
	}
}

func TestParseRejectsCredentialShapedFields(t *testing.T) {
	for _, key := range []string{"token", "api_key", "password", "auth_header", "client_secret"} {
		raw := []byte("version = 1\nrecommended_default = \"team\"\n" + key + " = \"must-not-exist\"\n")
		_, err := manifest.Parse(raw)
		if err == nil || !strings.Contains(err.Error(), "credential") {
			t.Errorf("key %s: error = %v", key, err)
		}
	}
}

func TestExportOmitsSecretsAdaptersAndOverrides(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes.Default = "team"
	cfg.Routes.Overrides[domain.ClientClaude] = "team"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/personal/claude"}
	data, err := manifest.Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"adapters", "overrides", "/personal/claude", "token", "secret"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("export contains %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, `recommended_default = 'team'`) && !strings.Contains(text, `recommended_default = "team"`) {
		t.Fatalf("export lacks recommended default:\n%s", text)
	}
}
