package configuration

import (
	"strings"
	"testing"
)

func TestRecommendationsAreValidatedExportedAndDoNotOverridePersonalChoices(t *testing.T) {
	raw := []byte(`version = 7
[recommendations.claude.primary]
route = "team-claude"

[recommendations.codex.primary]
route = "team-codex"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
anthropic = "https://team.test"

[routes.team-claude]
label = "Team Claude"
account = "team"
model = "claude-test"
interfaces = { anthropic = [] }

[routes.team-codex]
label = "Team Codex"
account = "team"
model = "gpt-test"
interfaces = { openai_responses = [] }
`)
	team, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Merge(NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("import converted recommendations into client bindings: %#v", got.Clients)
	}

	got.Accounts["personal"] = Account{Label: "Personal", Endpoints: Endpoints{Anthropic: "https://personal.test"}}
	got.Routes["personal-claude"] = testRoute("Personal Claude", "personal", "personal-model", ProtocolAnthropic)
	got.SetSelectedRoute(ClientClaude, "personal-claude")
	merged, err := Merge(got, team)
	if err != nil {
		t.Fatal(err)
	}
	if merged.SelectedRoute(ClientClaude) != "personal-claude" || merged.SelectedRoute(ClientCodex) != "" {
		t.Fatalf("merge replaced a personal client selection: %#v", merged.Clients)
	}

	exported, err := Export(merged)
	if err != nil {
		t.Fatal(err)
	}
	exportedTeam, err := Parse(exported)
	if err != nil {
		t.Fatal(err)
	}
	if exportedTeam.Recommendations[ClientClaude].Primary.Route != "personal-claude" || exportedTeam.Recommendations[ClientCodex].Primary.Route != "team-codex" {
		t.Fatalf("exported recommendations = %#v", exportedTeam.Recommendations)
	}
}

func TestParseConfigurationManifestAndMergePreservesPersonalState(t *testing.T) {
	raw := []byte(`version = 7

[accounts.team]
label = "Team Gateway"

[accounts.team.endpoints]
openai_responses = "https://gateway.test/v1"
anthropic = "https://gateway.test"

[routes.team]
label = "Team Gateway"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`)
	m, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.SetClientActivation(ClientClaude, true, "/personal/claude", nil)
	cfg.SetSelectedRoute(ClientClaude, "personal")
	cfg.Accounts["personal"] = Account{Label: "Personal", Endpoints: Endpoints{Anthropic: "https://personal.test"}}
	cfg.Routes["personal"] = testRoute("Personal", "personal", "claude-personal", ProtocolAnthropic)
	got, err := Merge(cfg, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedRoute(ClientClaude) != "personal" {
		t.Fatalf("personal selection changed: %#v", got.Clients)
	}
	if got.Clients[ClientClaude].Executable != "/personal/claude" {
		t.Fatalf("personal adapter changed: %#v", got.Clients)
	}
	if got.Routes["team"].Label != "Team Gateway" {
		t.Fatalf("imported route missing: %#v", got.Routes)
	}
}

func TestMergeRejectsConflictingExistingAccountWithoutMutatingLocalConfig(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[routes.team-route]
label = "TeamRoute"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Personal Gateway", Endpoints: Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Routes["local"] = testRoute("Local", "team", "claude-local", ProtocolAnthropic)

	_, err = Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `account "team" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if !strings.Contains(err.Error(), "aigw config export") || strings.Contains(err.Error(), "aigw account list") {
		t.Fatalf("merge guidance must name an existing inspection command: %v", err)
	}
	if got := cfg.Accounts["team"].Endpoints.Anthropic; got != "https://personal.example.test" {
		t.Fatalf("conflicting merge mutated existing endpoint: %q", got)
	}
	if _, exists := cfg.Routes["team-route"]; exists {
		t.Fatalf("conflicting merge partially imported route: %#v", cfg.Routes)
	}
}

func TestMergeRejectsConflictingExistingRouteWithoutMutatingLocalConfig(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[routes.shared]
label = "Team Model"
account = "team"
model = "team-model"
interfaces = { openai_responses = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Routes["shared"] = testRoute("Personal Model", "team", "personal-model", ProtocolOpenAIResponses)

	_, err = Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `route "shared" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if got := cfg.Routes["shared"].Model; got != "personal-model" {
		t.Fatalf("conflicting merge mutated active route model: %q", got)
	}
}

func TestMergeAcceptsEquivalentExistingIdentityWithoutReplacingLocalState(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[routes.shared]
label = "TeamRoute"
purpose = "Default agent"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{Anthropic: "https://team.example.test/"}}
	cfg.Routes["shared"] = testRoute("TeamRoute", "team", "claude-team", ProtocolAnthropic)
	route := cfg.Routes["shared"]
	route.Purpose = "Default agent"
	cfg.Routes["shared"] = route

	got, err := Merge(cfg, team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["shared"].Model != "claude-team" {
		t.Fatalf("equivalent merge = %#v", got)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test/" {
		t.Fatalf("idempotent import should preserve local canonical representation: %#v", got.Accounts["team"])
	}
}

func TestMergeWithOptionsReplacesOnlyExplicitConflictingIdentity(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"

[routes.shared]
label = "Team Model"
account = "team"
model = "team-model"
interfaces = { openai_responses = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Personal Gateway", Endpoints: Endpoints{OpenAIResponses: "https://personal.example.test/v1"}}
	cfg.Routes["shared"] = testRoute("Personal Model", "team", "personal-model", ProtocolOpenAIResponses)

	got, err := MergeWithOptions(cfg, team, MergeOptions{
		ReplaceAccounts: map[string]bool{"team": true},
		ReplaceRoutes:   map[string]bool{"shared": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.OpenAIResponses != "https://team.example.test/v1" {
		t.Fatalf("account replacement = %#v", got.Accounts["team"])
	}
	if got.Routes["shared"].Model != "team-model" {
		t.Fatalf("route replacement = %#v", got.Routes["shared"])
	}
}

func TestMergeWithOptionsDistinguishesEmptyCapabilityListsByProtocol(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
openai_chat_completions = "https://team.example.test/v1"

[routes.shared]
label = "Team Model"
account = "team"
model = "team-model"
interfaces = { openai_chat_completions = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label: "Team Gateway",
		Endpoints: Endpoints{
			OpenAIResponses:       "https://team.example.test/v1",
			OpenAIChatCompletions: "https://team.example.test/v1",
		},
	}
	cfg.Routes["shared"] = testRoute("Team Model", "team", "team-model", ProtocolOpenAIResponses)

	if _, err := MergeWithOptions(cfg, team, MergeOptions{}); err == nil || !strings.Contains(err.Error(), `route "shared" conflicts`) {
		t.Fatalf("different protocol keys must conflict even when capabilities are empty: %v", err)
	}
	got, err := MergeWithOptions(cfg, team, MergeOptions{ReplaceRoutes: map[string]bool{"shared": true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, oldProtocol := got.Routes["shared"].Interfaces[ProtocolOpenAIResponses]; oldProtocol {
		t.Fatalf("Responses protocol remained after explicit Route replacement: %#v", got.Routes["shared"].Interfaces)
	}
	if _, newProtocol := got.Routes["shared"].Interfaces[ProtocolOpenAIChatCompletions]; !newProtocol {
		t.Fatalf("Chat Completions protocol was not applied: %#v", got.Routes["shared"].Interfaces)
	}
}

func TestMergeWithOptionsRejectsUnusedReplacementSelectors(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[routes.team-route]
label = "TeamRoute"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["local"] = Account{Label: "Local", Endpoints: Endpoints{Anthropic: "https://local.example.test"}}
	cfg.Routes["local"] = testRoute("Local", "local", "claude-local", ProtocolAnthropic)

	_, err = MergeWithOptions(cfg, team, MergeOptions{ReplaceAccounts: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-account "missing"`) {
		t.Fatalf("unused account replacement error = %v", err)
	}
	_, err = MergeWithOptions(cfg, team, MergeOptions{ReplaceRoutes: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-route "missing"`) {
		t.Fatalf("unused Route replacement error = %v", err)
	}
}

func TestMergeWithOptionsDoesNotNormalizeOrMutateRejectedInput(t *testing.T) {
	team, err := Parse([]byte(`version = 7

[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"

[routes.team]
label = "Team"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Version: ConfigVersion,
		Accounts: map[string]Account{
			"team": {Label: "Personal Gateway", Endpoints: Endpoints{Anthropic: "https://personal.example.test"}},
		},
		Routes: map[string]Route{
			"local": {Label: "Local", Account: "team"},
		},
		Clients: nil,
	}

	_, err = MergeWithOptions(cfg, team, MergeOptions{})
	if err == nil {
		t.Fatal("expected conflict")
	}
	if cfg.Recommendations != nil || cfg.Clients != nil {
		t.Fatalf("rejected merge normalized caller-owned config: %#v", cfg)
	}
}

func TestMergeRejectsNonCanonicalLocalSchemaVersion(t *testing.T) {
	team, err := Parse([]byte(`version = 7
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
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Version = 1
	cfg.Accounts["local"] = Account{Label: "Local", Endpoints: Endpoints{Anthropic: "https://local.test"}}
	cfg.Routes["local"] = testRoute("Local", "local", "claude-local", ProtocolAnthropic)
	if _, err := Merge(cfg, team); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("merge error = %v", err)
	}
}

func TestMergeWithOptionsRejectsNonCanonicalConfigurationManifestVersion(t *testing.T) {
	team := Manifest{Version: 99, Routes: map[string]Route{"team": {Label: "Team", Account: "team"}}}
	if _, err := MergeWithOptions(NewConfig(), team, MergeOptions{}); err == nil || !strings.Contains(err.Error(), "unsupported configuration manifest version 99") {
		t.Fatalf("unsupported manifest version error = %v", err)
	}
}

func TestMergeDefaultsToFirstImportedRouteWhenNeitherSideChoosesADefault(t *testing.T) {
	team, err := Parse([]byte(`version = 7
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[routes.solo]
label = "Solo"
account = "team"
model = "claude-solo"
interfaces = { anthropic = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Merge(NewConfig(), team)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("merge invented a client binding without a recommendation: %#v", got.Clients)
	}
}

func TestMergeRejectsConflictingModelOverrideWithOtherwiseIdenticalRoute(t *testing.T) {
	team, err := Parse([]byte(`version = 7
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
[routes.shared]
label = "TeamRoute"
account = "team"
model = "team-model"
interfaces = { openai_responses = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team Gateway", Endpoints: Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Routes["shared"] = testRoute("TeamRoute", "team", "personal-model", ProtocolOpenAIResponses)

	if _, err := Merge(cfg, team); err == nil || !strings.Contains(err.Error(), `route "shared" conflicts`) {
		t.Fatalf("model-only conflict error = %v", err)
	}
}

func TestMergeTreatsIdenticalAccountProbesAsEquivalent(t *testing.T) {
	team, err := Parse([]byte(`version = 7
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"
[accounts.team.account_probe]
kind = "dmxapi"
base_url = "https://team.example.test/probe"
[routes.team]
label = "Team"
account = "team"
model = "claude-team"
interfaces = { anthropic = [] }
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label:        "Team Gateway",
		Endpoints:    Endpoints{Anthropic: "https://team.example.test"},
		AccountProbe: &AccountProbe{Kind: "dmxapi", BaseURL: "https://team.example.test/probe/"},
	}
	cfg.Routes["local"] = testRoute("Local", "team", "claude-local", ProtocolAnthropic)

	got, err := Merge(cfg, team)
	if err != nil {
		t.Fatalf("equivalent account probes must not conflict: %v", err)
	}
	if got.Accounts["team"].AccountProbe.Kind != "dmxapi" {
		t.Fatalf("merged account probe = %#v", got.Accounts["team"].AccountProbe)
	}
}
