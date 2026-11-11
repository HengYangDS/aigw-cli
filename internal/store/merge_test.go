package store

import (
	"aigw-cli/internal/domain"
	"testing"
)

func TestRecommendationsAreValidatedExportedAndDoNotOverridePersonalChoices(t *testing.T) {
	raw := []byte(`version = 8

[[providers]]
id = "team"
label = "Team"
endpoints = {anthropic = "https://team.test", responses = "https://team.test/v1"}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "Team Claude"
protocols = {anthropic = []}

[[providers.models]]
id = "gpt-test"
family = "gpt-test"
label = "Team Codex"
protocols = {responses = []}

[recommendations.claude]
primary = {model = "team/claude-test"}

[recommendations.codex]
primary = {model = "team/gpt-test"}
`)
	team, err := ParseTeamFile(raw, testAdmission())
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
	got.Models[domain.NewModelRef("personal", "personal-model")] = testModel("Personal Claude", "personal", "personal-model", domain.ProtocolAnthropic)
	got.SetSelectedModel(ClientClaude, domain.NewModelRef("personal", "personal-model"), "")
	merged, _, err := domain.MergeTeamFile(got, team, false)
	if err != nil {
		t.Fatal(err)
	}
	if merged.SelectedModel(ClientClaude) != domain.NewModelRef("personal", "personal-model") || !merged.SelectedModel(ClientCodex).IsZero() {
		t.Fatalf("merge replaced a personal client selection: %#v", merged.Clients)
	}

	exported, err := ExportTeamFile(merged)
	if err != nil {
		t.Fatal(err)
	}
	exportedTeam, err := ParseTeamFile(exported, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if exportedTeam.Recommendations[ClientClaude].Primary.Model != domain.NewModelRef("team", "claude-test") || exportedTeam.Recommendations[ClientCodex].Primary.Model != domain.NewModelRef("team", "gpt-test") {
		t.Fatalf("exported recommendations = %#v", exportedTeam.Recommendations)
	}
}

func TestParseConfigurationManifestAndMergePreservesPersonalState(t *testing.T) {
	raw := []byte(`version = 8

[[providers]]
id = "team"
label = "Team Gateway"
endpoints = {anthropic = "https://gateway.test", responses = "https://gateway.test/v1"}

[[providers.models]]
id = "claude-team"
family = "claude-team"
label = "Team Gateway"
protocols = {anthropic = []}
`)
	m, err := ParseTeamFile(raw, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.SetClientActivation(ClientClaude, true, "/personal/claude", nil)
	cfg.SetSelectedModel(ClientClaude, domain.NewModelRef("personal", "claude-personal"), "")
	cfg.Providers["personal"] = domain.Provider{Label: "Personal", Endpoints: domain.Endpoints{Anthropic: "https://personal.test"}}
	cfg.Models[domain.NewModelRef("personal", "claude-personal")] = testModel("Personal", "personal", "claude-personal", domain.ProtocolAnthropic)
	got, _, err := domain.MergeTeamFile(cfg, m, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(ClientClaude) != domain.NewModelRef("personal", "claude-personal") {
		t.Fatalf("personal selection changed: %#v", got.Clients)
	}
	if got.Clients[ClientClaude].Executable != "/personal/claude" {
		t.Fatalf("personal adapter changed: %#v", got.Clients)
	}
	if got.Models[domain.NewModelRef("team", "claude-team")].Label != "Team Gateway" {
		t.Fatalf("imported route missing: %#v", got.Models)
	}
}

func TestMergeDefaultsToFirstImportedModelWhenNeitherSideChoosesADefault(t *testing.T) {
	team, err := ParseTeamFile([]byte(`version = 8

[[providers]]
id = "team"
label = "Team"
endpoints = {anthropic = "https://team.test"}

[[providers.models]]
id = "claude-solo"
family = "claude-solo"
label = "Solo"
protocols = {anthropic = []}
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
