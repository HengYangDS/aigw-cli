package store

import (
	"aigw-cli/internal/domain"
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
	team, err := Parse(raw, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	got, err := domain.Merge(domain.NewConfig(testAdmission()), team)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("import converted recommendations into client bindings: %#v", got.Clients)
	}

	got.Accounts["personal"] = domain.Account{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	got.Models["personal-claude"] = testModel("Personal Claude", "personal", "personal-model", domain.ProtocolAnthropic)
	got.SetSelectedModel(ClientClaude, "personal-claude", "")
	merged, err := domain.Merge(got, team)
	if err != nil {
		t.Fatal(err)
	}
	if merged.SelectedModel(ClientClaude) != "personal-claude" || merged.SelectedModel(ClientCodex) != "" {
		t.Fatalf("merge replaced a personal client selection: %#v", merged.Clients)
	}

	exported, err := Export(merged)
	if err != nil {
		t.Fatal(err)
	}
	exportedTeam, err := Parse(exported, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if exportedTeam.Recommendations[ClientClaude].Primary.Model != "team-claude" || exportedTeam.Recommendations[ClientCodex].Primary.Model != "team-codex" {
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
	m, err := Parse(raw, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.SetClientActivation(ClientClaude, true, "/personal/claude", nil)
	cfg.SetSelectedModel(ClientClaude, "personal", "")
	cfg.Accounts["personal"] = domain.Account{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	cfg.Models["personal"] = testModel("Personal", "personal", "claude-personal", domain.ProtocolAnthropic)
	got, err := domain.Merge(cfg, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(ClientClaude) != "personal" {
		t.Fatalf("personal selection changed: %#v", got.Clients)
	}
	if got.Clients[ClientClaude].Executable != "/personal/claude" {
		t.Fatalf("personal adapter changed: %#v", got.Clients)
	}
	if got.Models["team"].Label != "Team Gateway" {
		t.Fatalf("imported route missing: %#v", got.Models)
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Personal Gateway", Endpoints: domain.Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Models["local"] = testModel("Local", "team", "claude-local", domain.ProtocolAnthropic)

	_, err = domain.Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `account "team" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if !strings.Contains(err.Error(), "aigw config export") || strings.Contains(err.Error(), "aigw account list") {
		t.Fatalf("merge guidance must name an existing inspection command: %v", err)
	}
	if got := cfg.Accounts["team"].Endpoints.Anthropic; got != "https://personal.example.test" {
		t.Fatalf("conflicting merge mutated existing endpoint: %q", got)
	}
	if _, exists := cfg.Models["team-route"]; exists {
		t.Fatalf("conflicting merge partially imported route: %#v", cfg.Models)
	}
}

func TestMergeRejectsConflictingExistingModelWithoutMutatingLocalConfig(t *testing.T) {
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Team Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Models["shared"] = testModel("Personal Model", "team", "personal-model", domain.ProtocolOpenAIResponses)

	_, err = domain.Merge(cfg, team)
	if err == nil || !strings.Contains(err.Error(), `route "shared" conflicts`) {
		t.Fatalf("merge error = %v", err)
	}
	if got := cfg.Models["shared"].Family; got != "personal-model" {
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Team Gateway", Endpoints: domain.Endpoints{Anthropic: "https://team.example.test/"}}
	cfg.Models["shared"] = testModel("TeamRoute", "team", "claude-team", domain.ProtocolAnthropic)
	route := cfg.Models["shared"]
	route.Purpose = "Default agent"
	cfg.Models["shared"] = route

	got, err := domain.Merge(cfg, team)
	if err != nil {
		t.Fatal(err)
	}
	if got.Models["shared"].Family != "claude-team" {
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Personal Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://personal.example.test/v1"}}
	cfg.Models["shared"] = testModel("Personal Model", "team", "personal-model", domain.ProtocolOpenAIResponses)

	got, err := domain.MergeWithOptions(cfg, team, domain.MergeOptions{
		ReplaceAccounts: map[string]bool{"team": true},
		ReplaceModels:   map[string]bool{"shared": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.OpenAIResponses != "https://team.example.test/v1" {
		t.Fatalf("account replacement = %#v", got.Accounts["team"])
	}
	if got.Models["shared"].Family != "team-model" {
		t.Fatalf("route replacement = %#v", got.Models["shared"])
	}
	kept, err := domain.MergeWithOptions(cfg, team, domain.MergeOptions{
		KeepAccounts:  map[string]bool{"team": true},
		ReplaceModels: map[string]bool{"shared": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Accounts["team"].Endpoints.OpenAIResponses != "https://personal.example.test/v1" || kept.Models["shared"].Family != "team-model" {
		t.Fatalf("local Account metadata was not retained alongside incoming Route: %#v", kept)
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{
		Label: "Team Gateway",
		Endpoints: domain.Endpoints{
			OpenAIResponses:       "https://team.example.test/v1",
			OpenAIChatCompletions: "https://team.example.test/v1",
		},
	}
	cfg.Models["shared"] = testModel("Team Model", "team", "team-model", domain.ProtocolOpenAIResponses)

	if _, err := domain.MergeWithOptions(cfg, team, domain.MergeOptions{}); err == nil || !strings.Contains(err.Error(), `route "shared" conflicts`) {
		t.Fatalf("different protocol keys must conflict even when capabilities are empty: %v", err)
	}
	got, err := domain.MergeWithOptions(cfg, team, domain.MergeOptions{ReplaceModels: map[string]bool{"shared": true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, oldProtocol := got.Models["shared"].Protocols[domain.ProtocolOpenAIResponses]; oldProtocol {
		t.Fatalf("Responses protocol remained after explicit Route replacement: %#v", got.Models["shared"].Protocols)
	}
	if _, newProtocol := got.Models["shared"].Protocols[domain.ProtocolOpenAIChatCompletions]; !newProtocol {
		t.Fatalf("Chat Completions protocol was not applied: %#v", got.Models["shared"].Protocols)
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["local"] = domain.Account{Label: "Local", Endpoints: domain.Endpoints{Anthropic: "https://local.example.test"}}
	cfg.Models["local"] = testModel("Local", "local", "claude-local", domain.ProtocolAnthropic)

	_, err = domain.MergeWithOptions(cfg, team, domain.MergeOptions{ReplaceAccounts: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-account "missing"`) {
		t.Fatalf("unused account replacement error = %v", err)
	}
	_, err = domain.MergeWithOptions(cfg, team, domain.MergeOptions{ReplaceModels: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), `--replace-route "missing"`) {
		t.Fatalf("unused Route replacement error = %v", err)
	}
	for _, test := range []struct {
		name    string
		options domain.MergeOptions
		want    string
	}{
		{"missing local", domain.MergeOptions{KeepAccounts: map[string]bool{"team": true}}, "does not name an existing local Account"},
		{"missing incoming", domain.MergeOptions{KeepAccounts: map[string]bool{"local": true}}, "does not name an Account in the imported"},
		{"conflicting selectors", domain.MergeOptions{KeepAccounts: map[string]bool{"team": true}, ReplaceAccounts: map[string]bool{"team": true}}, "conflicts with --replace-account"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := domain.MergeWithOptions(cfg, team, test.options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Account retention error = %v, want %q", err, test.want)
			}
		})
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := admitted(domain.Config{
		Version: domain.ConfigVersion,
		Accounts: map[string]domain.Account{
			"team": {Label: "Personal Gateway", Endpoints: domain.Endpoints{Anthropic: "https://personal.example.test"}},
		},
		Models: map[string]domain.Model{
			"local": {Label: "Local", Account: "team"},
		},
		Clients: nil,
	})

	_, err = domain.MergeWithOptions(cfg, team, domain.MergeOptions{})
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Version = 1
	cfg.Accounts["local"] = domain.Account{Label: "Local", Endpoints: domain.Endpoints{Anthropic: "https://local.test"}}
	cfg.Models["local"] = testModel("Local", "local", "claude-local", domain.ProtocolAnthropic)
	if _, err := domain.Merge(cfg, team); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("merge error = %v", err)
	}
}

func TestMergeWithOptionsRejectsNonCanonicalConfigurationManifestVersion(t *testing.T) {
	team := domain.Manifest{Version: 99, Models: map[string]domain.Model{"team": {Label: "Team", Account: "team"}}}
	if _, err := domain.MergeWithOptions(domain.NewConfig(testAdmission()), team, domain.MergeOptions{}); err == nil || !strings.Contains(err.Error(), "unsupported configuration manifest version 99") {
		t.Fatalf("unsupported manifest version error = %v", err)
	}
}

func TestMergeDefaultsToFirstImportedModelWhenNeitherSideChoosesADefault(t *testing.T) {
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	got, err := domain.Merge(domain.NewConfig(testAdmission()), team)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("merge invented a client binding without a recommendation: %#v", got.Clients)
	}
}

func TestMergeRejectsConflictingFamilyOverrideWithOtherwiseIdenticalModel(t *testing.T) {
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Team Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Models["shared"] = testModel("TeamRoute", "team", "personal-model", domain.ProtocolOpenAIResponses)

	if _, err := domain.Merge(cfg, team); err == nil || !strings.Contains(err.Error(), `route "shared" conflicts`) {
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
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{
		Label:        "Team Gateway",
		Endpoints:    domain.Endpoints{Anthropic: "https://team.example.test"},
		AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://team.example.test/probe/"},
	}
	cfg.Models["local"] = testModel("Local", "team", "claude-local", domain.ProtocolAnthropic)

	got, err := domain.Merge(cfg, team)
	if err != nil {
		t.Fatalf("equivalent account probes must not conflict: %v", err)
	}
	if got.Accounts["team"].AccountProbe.Kind != "dmxapi" {
		t.Fatalf("merged account probe = %#v", got.Accounts["team"].AccountProbe)
	}
}
