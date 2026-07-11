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

[accounts.team]
label = "Team Gateway"

[accounts.team.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"

[profiles.team]
label = "Team Gateway"
account = "team"
`)
	m, err := manifest.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/personal/claude"}
	cfg.Routes.Overrides[domain.ClientClaude] = "personal"
	cfg.Accounts["personal"] = domain.Account{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	cfg.Profiles["personal"] = domain.Profile{Label: "Personal", Account: "personal"}
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

func TestManifestPurposeRequiresVersionTwo(t *testing.T) {
	legacy := []byte(`version = 1
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "默认 Agent"
account = "team"
`)
	if _, err := manifest.Parse(legacy); err == nil || !strings.Contains(err.Error(), "version 2") {
		t.Fatalf("legacy purpose error = %v", err)
	}
	current := []byte(strings.Replace(string(legacy), "version = 1", "version = 2", 1))
	parsed, err := manifest.Parse(current)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version != domain.CurrentConfigVersion || parsed.Profiles["team"].Purpose != "默认 Agent" {
		t.Fatalf("parsed manifest = %#v", parsed)
	}
}

func TestMergeVersionTwoManifestRequiresLocalSchemaUpgrade(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "默认 Agent"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Version = domain.LegacyConfigVersion
	cfg.Accounts["local"] = domain.Account{Label: "Local", Endpoints: domain.Endpoints{Anthropic: "https://local.test"}}
	cfg.Profiles["local"] = domain.Profile{Label: "Local", Account: "local"}
	cfg.Routes.Default = "local"
	if _, err := manifest.Merge(cfg, team); err == nil || !strings.Contains(err.Error(), "config upgrade") {
		t.Fatalf("merge error = %v", err)
	}
}

func TestParseRejectsProfileOwnedEndpointResidue(t *testing.T) {
	raw := []byte(`version = 1
recommended_default = "team"

[profiles.team]
label = "Team Gateway"
account = "team"

[profiles.team.endpoints]
openai_responses = "https://gateway.test/v1"
`)
	if _, err := manifest.Parse(raw); err == nil {
		t.Fatalf("legacy Profile endpoint error = %v", err)
	}
}

func TestExportOmitsSecretsAdaptersAndOverrides(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Profiles["team"] = domain.Profile{Label: "Team", Account: "team"}
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
	if !strings.Contains(text, "version = 2") {
		t.Fatalf("new config export must use manifest v2:\n%s", text)
	}
}
