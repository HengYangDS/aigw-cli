package store

import (
	"aigw-cli/internal/domain"
	"bytes"
	"testing"
)

// An export carries the team's declared recommendations, never the exporting
// operator's bindings. An alternative or manual-only selection is a local
// choice; an import would otherwise treat it as reviewed preference.
func TestExportKeepsClientBindingsLocal(t *testing.T) {
	declared := func() domain.Config {
		cfg := domain.NewConfig(testAdmission())
		cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", Responses: "https://team.test/v1"}}
		cfg.Models[domain.NewModelRef("team", "claude-test")] = testModel("Team Claude", "team", "claude-test", domain.ProtocolAnthropic)
		cfg.Models[domain.NewModelRef("team", "claude-alternative")] = testModel("Team Claude Alternative", "team", "claude-alternative", domain.ProtocolAnthropic)
		cfg.Models[domain.NewModelRef("team", "gpt-test")] = testModel("Team Codex", "team", "gpt-test", domain.ProtocolResponses)
		cfg.Models[domain.NewModelRef("team", "gpt-manual")] = testModel("Manual Codex", "team", "gpt-manual", domain.ProtocolResponses)
		cfg.Recommendations[ClientClaude] = domain.ClientRecommendation{
			Primary:      domain.ClientSelection{Model: domain.NewModelRef("team", "claude-test")},
			Alternatives: []domain.ClientSelection{{Model: domain.NewModelRef("team", "claude-alternative")}},
		}
		cfg.Recommendations[ClientCodex] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("team", "gpt-test")}}
		return cfg
	}
	unselected, err := ExportTeamFile(declared())
	if err != nil {
		t.Fatal(err)
	}
	selected := declared()
	selected.SetSelectedModel(ClientClaude, domain.NewModelRef("team", "claude-alternative"), "")
	selected.SetSelectedModel(ClientCodex, domain.NewModelRef("team", "gpt-manual"), "")
	exported, err := ExportTeamFile(selected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(exported, unselected) {
		t.Fatalf("bindings changed the exported manifest:\nwithout bindings:\n%s\nwith bindings:\n%s", unselected, exported)
	}
	manifest, err := ParseTeamFile(exported, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	claude := manifest.Recommendations[ClientClaude]
	if claude.Primary.Model != domain.NewModelRef("team", "claude-test") || len(claude.Alternatives) != 1 || claude.Alternatives[0].Model != domain.NewModelRef("team", "claude-alternative") ||
		manifest.Recommendations[ClientCodex].Primary.Model != domain.NewModelRef("team", "gpt-test") || manifest.Models[domain.NewModelRef("team", "gpt-manual")].Provider != "team" {
		t.Fatalf("exported recommendations = %#v", manifest.Recommendations)
	}
}
