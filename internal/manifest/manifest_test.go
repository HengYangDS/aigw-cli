package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
)

func TestRepositoryTeamManifestMatchesCurrentProfileMatrix(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "team", "team-profiles.toml"))
	if err != nil {
		t.Fatal(err)
	}
	team, err := manifest.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if team.Version != 3 {
		t.Fatalf("team manifest version = %d, want 3", team.Version)
	}
	if team.RecommendedDefault != "dmxapi-gpt-5.6-terra" {
		t.Fatalf("recommended default = %q", team.RecommendedDefault)
	}
	if len(team.RecommendedRoutes) != 1 || team.RecommendedRoutes[domain.ClientClaude] != "aihubmix-claude-sonnet-5" {
		t.Fatalf("recommended routes = %#v", team.RecommendedRoutes)
	}
	if len(team.Accounts) != 2 || len(team.Profiles) != 24 {
		t.Fatalf("example matrix has %d Accounts and %d profiles, want 2 and 24", len(team.Accounts), len(team.Profiles))
	}
	if got := team.Accounts["aihubmix"].Endpoints; got.OpenAIResponses != "https://aihubmix.com/v1" || got.Anthropic != "https://aihubmix.com" {
		t.Fatalf("AIHubMix endpoints = %#v", got)
	}
	dmxapi := team.Accounts["dmxapi"]
	if dmxapi.Endpoints.OpenAIResponses != "http://127.0.0.1:8791/v1" || dmxapi.Endpoints.Anthropic != "https://www.dmxapi.cn" {
		t.Fatalf("DMXAPI endpoints = %#v", dmxapi.Endpoints)
	}
	if dmxapi.AccountProbe == nil || dmxapi.AccountProbe.Kind != "dmxapi" || dmxapi.AccountProbe.BaseURL != "https://www.dmxapi.cn" {
		t.Fatalf("DMXAPI account probe = %#v", dmxapi.AccountProbe)
	}

	want := map[string]struct{ client, model string }{
		"aihubmix-claude-fable-5":      {domain.ClientClaude, "claude-fable-5"},
		"aihubmix-claude-opus-5":       {domain.ClientClaude, "claude-opus-5"},
		"aihubmix-claude-sonnet-5":     {domain.ClientClaude, "claude-sonnet-5"},
		"aihubmix-gpt-5.6-luna":        {domain.ClientCodex, "gpt-5.6-luna"},
		"aihubmix-gpt-5.6-sol":         {domain.ClientCodex, "gpt-5.6-sol"},
		"aihubmix-gpt-5.6-terra":       {domain.ClientCodex, "gpt-5.6-terra"},
		"dmxapi-claude-fable-5":        {domain.ClientClaude, "claude-fable-5"},
		"dmxapi-claude-fable-5-cc":     {domain.ClientClaude, "claude-fable-5-cc"},
		"dmxapi-claude-fable-5-ssvip":  {domain.ClientClaude, "claude-fable-5-ssvip"},
		"dmxapi-claude-opus-5-cc":      {domain.ClientClaude, "claude-opus-5-cc"},
		"dmxapi-claude-opus-5-ssvip":   {domain.ClientClaude, "claude-opus-5-ssvip"},
		"dmxapi-claude-sonnet-5":       {domain.ClientClaude, "claude-sonnet-5"},
		"dmxapi-claude-sonnet-5-cc":    {domain.ClientClaude, "claude-sonnet-5-cc"},
		"dmxapi-claude-sonnet-5-ssvip": {domain.ClientClaude, "claude-sonnet-5-ssvip"},
		"dmxapi-gpt-5.6-luna":          {domain.ClientCodex, "gpt-5.6-luna"},
		"dmxapi-gpt-5.6-luna-cdx":      {domain.ClientCodex, "gpt-5.6-luna-cdx"},
		"dmxapi-gpt-5.6-luna-ssvip":    {domain.ClientCodex, "gpt-5.6-luna-ssvip"},
		"dmxapi-gpt-5.6-sol":           {domain.ClientCodex, "gpt-5.6-sol"},
		"dmxapi-gpt-5.6-sol-cdx":       {domain.ClientCodex, "gpt-5.6-sol-cdx"},
		"dmxapi-gpt-5.6-sol-ssvip":     {domain.ClientCodex, "gpt-5.6-sol-ssvip"},
		"dmxapi-gpt-5.6-terra":         {domain.ClientCodex, "gpt-5.6-terra"},
		"dmxapi-gpt-5.6-terra-cdx":     {domain.ClientCodex, "gpt-5.6-terra-cdx"},
		"dmxapi-gpt-5.6-terra-ssvip":   {domain.ClientCodex, "gpt-5.6-terra-ssvip"},
		"dmxapi-qwen3.7-max":           {domain.ClientCodex, "qwen3.7-max"},
	}
	for name, expected := range want {
		profile, ok := team.Profiles[name]
		if !ok {
			t.Errorf("missing profile %q", name)
			continue
		}
		if profile.Client != expected.client || profile.Models[expected.client] != expected.model {
			t.Errorf("profile %q = client %q model %q, want %q and %q", name, profile.Client, profile.Models[expected.client], expected.client, expected.model)
		}
	}
	for name := range team.Profiles {
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected profile %q", name)
		}
	}
}

func TestExampleTeamManifestIsProviderNeutralVersionThree(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "team-profiles.toml"))
	if err != nil {
		t.Fatal(err)
	}
	team, err := manifest.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if team.Version != 3 || len(team.Accounts) != 1 || len(team.Profiles) != 4 {
		t.Fatalf("neutral example = version %d, %d Accounts, %d profiles", team.Version, len(team.Accounts), len(team.Profiles))
	}
	if _, ok := team.Profiles["claude-opus-5"]; !ok {
		t.Fatal("neutral example lacks Claude Opus 5")
	}
	for name := range team.Profiles {
		if strings.Contains(name, "4-8") {
			t.Fatalf("neutral example retains obsolete profile %q", name)
		}
	}
}

func TestRecommendedRoutesAreValidatedExportedAndDoNotOverridePersonalChoices(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "team-codex"
[recommended_routes]
claude = "team-claude"
codex = "team-codex"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
anthropic = "https://team.test"

[profiles.team-claude]
label = "Team Claude"
account = "team"
client = "claude"
[profiles.team-claude.models]
claude = "claude-test"

[profiles.team-codex]
label = "Team Codex"
account = "team"
client = "codex"
[profiles.team-codex.models]
codex = "gpt-test"
`)
	team, err := manifest.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := manifest.Merge(domain.NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Overrides[domain.ClientClaude] != "team-claude" || got.Routes.Overrides[domain.ClientCodex] != "team-codex" {
		t.Fatalf("recommended routes not applied: %#v", got.Routes)
	}

	got.Accounts["personal"] = domain.Account{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	got.Profiles["personal-claude"] = domain.Profile{Label: "Personal Claude", Account: "personal", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "personal-model"}}
	got.Routes.Overrides[domain.ClientClaude] = "personal-claude"
	merged, err := manifest.Merge(got, team)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Routes.Overrides[domain.ClientClaude] != "personal-claude" || merged.Routes.Overrides[domain.ClientCodex] != "team-codex" {
		t.Fatalf("merge replaced a personal route: %#v", merged.Routes)
	}

	exported, err := manifest.Export(merged)
	if err != nil {
		t.Fatal(err)
	}
	exportedTeam, err := manifest.Parse(exported)
	if err != nil {
		t.Fatal(err)
	}
	if exportedTeam.RecommendedRoutes[domain.ClientClaude] != "personal-claude" || exportedTeam.RecommendedRoutes[domain.ClientCodex] != "team-codex" {
		t.Fatalf("exported routes = %#v", exportedTeam.RecommendedRoutes)
	}
}

func TestParseRejectsIncompatibleRecommendedRoute(t *testing.T) {
	raw := []byte(`version = 3
recommended_default = "codex"
[recommended_routes]
claude = "codex"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
[profiles.codex]
label = "Codex"
account = "team"
client = "codex"
[profiles.codex.models]
codex = "gpt-test"
`)
	if _, err := manifest.Parse(raw); err == nil {
		t.Fatal("incompatible recommended route was accepted")
	}
}

func TestVersionTwoRejectsRecommendedRoutesButRemainsSupportedWithoutThem(t *testing.T) {
	legacy := `version = 2
recommended_default = "claude"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.claude]
label = "Claude"
account = "team"
client = "claude"
[profiles.claude.models]
claude = "claude-test"
`
	if _, err := manifest.Parse([]byte(legacy)); err != nil {
		t.Fatalf("legacy v2 manifest was rejected: %v", err)
	}
	withRoutes := strings.Replace(legacy, "[accounts.team]", "[recommended_routes]\nclaude = \"claude\"\n[accounts.team]", 1)
	if _, err := manifest.Parse([]byte(withRoutes)); err == nil || !strings.Contains(err.Error(), "version 3") {
		t.Fatalf("v2 recommended_routes error = %v", err)
	}
	withEmptyRoutes := strings.Replace(legacy, "[accounts.team]", "[recommended_routes]\n[accounts.team]", 1)
	if _, err := manifest.Parse([]byte(withEmptyRoutes)); err == nil || !strings.Contains(err.Error(), "version 3") {
		t.Fatalf("v2 empty recommended_routes error = %v", err)
	}
}

func TestExportRejectsRouteThatCannotBeParsedBack(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "team", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "codex"
	cfg.Routes.Overrides[domain.ClientClaude] = "codex"

	if _, err := manifest.Export(cfg); err == nil {
		t.Fatal("export accepted a client-incompatible route")
	}
}

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

func TestExportOmitsSecretsAndAdaptersAndPublishesRouteRecommendations(t *testing.T) {
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
	if !strings.Contains(text, "version = 3") || !strings.Contains(text, "recommended_routes") || !strings.Contains(text, "claude") {
		t.Fatalf("new config export must use manifest v3 with route recommendations:\n%s", text)
	}
}
