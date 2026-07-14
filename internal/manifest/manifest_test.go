package manifest_test

import (
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
)

func TestParseTeamManifestAndMergePreservesPersonalState(t *testing.T) {
	raw := []byte(`version = 2
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

func TestMergeRejectsConflictingExistingAccountWithoutMutatingLocalConfig(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "team-profile"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team-profile]
label = "Team Profile"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Personal Gateway", Endpoints: domain.Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Profiles["local"] = domain.Profile{Label: "Local", Account: "team"}
	cfg.Routes.Default = "local"

	_, err = manifest.Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `account "team" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if got := cfg.Accounts["team"].Endpoints.Anthropic; got != "https://personal.example.test" {
		t.Fatalf("conflicting merge mutated existing endpoint: %q", got)
	}
	if _, exists := cfg.Profiles["team-profile"]; exists {
		t.Fatalf("conflicting merge partially imported profile: %#v", cfg.Profiles)
	}
}

func TestMergeRejectsConflictingExistingProfileWithoutMutatingLocalConfig(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "shared"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[profiles.shared]
label = "Team Model"
account = "team"
client = "codex"
[profiles.shared.models]
codex = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = domain.Profile{Label: "Personal Model", Account: "team", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "personal-model"}}
	cfg.Routes.Default = "shared"

	_, err = manifest.Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `profile "shared" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if got := cfg.Profiles["shared"].Models[domain.ClientCodex]; got != "personal-model" {
		t.Fatalf("conflicting merge mutated active profile model: %q", got)
	}
}

func TestMergeAcceptsEquivalentExistingIdentityWithoutReplacingLocalState(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "shared"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.shared]
label = "Team Profile"
purpose = "Default agent"
account = "team"
client = "claude"
[profiles.shared.models]
claude = "claude-team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team Gateway", Endpoints: domain.Endpoints{Anthropic: "https://team.example.test/"}}
	cfg.Profiles["shared"] = domain.Profile{Label: "Team Profile", Purpose: "Default agent", Account: "team", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-team"}}
	cfg.Routes.Default = "shared"

	got, err := manifest.Merge(cfg, team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "shared" || got.Profiles["shared"].Models[domain.ClientClaude] != "claude-team" {
		t.Fatalf("equivalent merge = %#v", got)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test/" {
		t.Fatalf("idempotent import should preserve local canonical representation: %#v", got.Accounts["team"])
	}
}

func TestMergeWithOptionsReplacesOnlyExplicitConflictingIdentity(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "shared"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[profiles.shared]
label = "Team Model"
account = "team"
client = "codex"
[profiles.shared.models]
codex = "team-model"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Personal Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://personal.example.test/v1"}}
	cfg.Profiles["shared"] = domain.Profile{Label: "Personal Model", Account: "team", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "personal-model"}}
	cfg.Routes.Default = "shared"

	got, err := manifest.MergeWithOptions(cfg, team, manifest.MergeOptions{
		ReplaceAccounts: map[string]bool{"team": true},
		ReplaceProfiles: map[string]bool{"shared": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.OpenAIResponses != "https://team.example.test/v1" {
		t.Fatalf("account replacement = %#v", got.Accounts["team"])
	}
	if got.Profiles["shared"].Models[domain.ClientCodex] != "team-model" {
		t.Fatalf("profile replacement = %#v", got.Profiles["shared"])
	}
}

func TestMergeWithOptionsRejectsUnusedReplacementSelectors(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "team-profile"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team-profile]
label = "Team Profile"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["local"] = domain.Account{Label: "Local", Endpoints: domain.Endpoints{Anthropic: "https://local.example.test"}}
	cfg.Profiles["local"] = domain.Profile{Label: "Local", Account: "local"}
	cfg.Routes.Default = "local"

	_, err = manifest.MergeWithOptions(cfg, team, manifest.MergeOptions{ReplaceAccounts: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-account "missing"`) {
		t.Fatalf("unused account replacement error = %v", err)
	}
	_, err = manifest.MergeWithOptions(cfg, team, manifest.MergeOptions{ReplaceProfiles: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-profile "missing"`) {
		t.Fatalf("unused profile replacement error = %v", err)
	}
}

func TestMergeWithOptionsDoesNotNormalizeOrMutateRejectedInput(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "team"

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[profiles.team]
label = "Team"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.Config{
		Version: domain.ConfigVersion,
		Accounts: map[string]domain.Account{
			"team": {Label: "Personal Gateway", Endpoints: domain.Endpoints{Anthropic: "https://personal.example.test"}},
		},
		Profiles: map[string]domain.Profile{
			"local": {Label: "Local", Account: "team"},
		},
		Routes: domain.Routes{Default: "local"},
	}

	_, err = manifest.MergeWithOptions(cfg, team, manifest.MergeOptions{})
	if err == nil {
		t.Fatal("expected conflict")
	}
	if cfg.Routes.Overrides != nil || cfg.Adapters != nil {
		t.Fatalf("rejected merge normalized caller-owned config: %#v", cfg)
	}
}

func TestParseRejectsCredentialShapedFields(t *testing.T) {
	for _, key := range []string{"token", "api_key", "password", "auth_header", "client_secret"} {
		raw := []byte("version = 2\nrecommended_default = \"team\"\n" + key + " = \"must-not-exist\"\n")
		_, err := manifest.Parse(raw)
		if err == nil || !strings.Contains(err.Error(), "credential") {
			t.Errorf("key %s: error = %v", key, err)
		}
	}
}

func TestParseRejectsNonCanonicalSchemaVersion(t *testing.T) {
	oldSchema := []byte(`version = 1
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "Default agent"
account = "team"
`)
	if _, err := manifest.Parse(oldSchema); err == nil || !strings.Contains(err.Error(), "unsupported team manifest version 1") {
		t.Fatalf("version 1 parse error = %v", err)
	}
	current := []byte(strings.Replace(string(oldSchema), "version = 1", "version = 2", 1))
	parsed, err := manifest.Parse(current)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version != domain.ConfigVersion || parsed.Profiles["team"].Purpose != "Default agent" {
		t.Fatalf("parsed manifest = %#v", parsed)
	}
}

func TestMergeRejectsNonCanonicalLocalSchemaVersion(t *testing.T) {
	team, err := manifest.Parse([]byte(`version = 2
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://gateway.test"
[profiles.team]
label = "Team"
purpose = "Default agent"
account = "team"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Version = 1
	cfg.Accounts["local"] = domain.Account{Label: "Local", Endpoints: domain.Endpoints{Anthropic: "https://local.test"}}
	cfg.Profiles["local"] = domain.Profile{Label: "Local", Account: "local"}
	cfg.Routes.Default = "local"
	if _, err := manifest.Merge(cfg, team); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("merge error = %v", err)
	}
}

func TestParseRejectsProfileOwnedEndpointResidue(t *testing.T) {
	raw := []byte(`version = 2
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
