package store

import (
	"aigw-cli/internal/domain"
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
	got, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), team, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("import converted recommendations into client bindings: %#v", got.Clients)
	}

	got.Providers["personal"] = domain.Provider{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	got.Models["personal-claude"] = testModel("Personal Claude", "personal", "personal-model", domain.ProtocolAnthropic)
	got.SetSelectedModel(ClientClaude, "personal-claude", "")
	merged, _, err := domain.MergeTeamFile(got, team, false)
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
	cfg.Providers["personal"] = domain.Provider{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	cfg.Models["personal"] = testModel("Personal", "personal", "claude-personal", domain.ProtocolAnthropic)
	got, _, err := domain.MergeTeamFile(cfg, m, false)
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
	got, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), team, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Clients) != 0 {
		t.Fatalf("merge invented a client binding without a recommendation: %#v", got.Clients)
	}
}
