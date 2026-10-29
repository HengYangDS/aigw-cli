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
		cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
		cfg.Routes["team-claude"] = testRoute("Team Claude", "team", "claude-test", domain.ProtocolAnthropic)
		cfg.Routes["team-claude-alternative"] = testRoute("Team Claude Alternative", "team", "claude-alternative", domain.ProtocolAnthropic)
		cfg.Routes["team-codex"] = testRoute("Team Codex", "team", "gpt-test", domain.ProtocolOpenAIResponses)
		cfg.Routes["manual-codex"] = testRoute("Manual Codex", "team", "gpt-manual", domain.ProtocolOpenAIResponses)
		cfg.Recommendations[ClientClaude] = domain.ClientRecommendation{
			Primary:      domain.ClientSelection{Route: "team-claude"},
			Alternatives: []domain.ClientSelection{{Route: "team-claude-alternative"}},
		}
		cfg.Recommendations[ClientCodex] = domain.ClientRecommendation{Primary: domain.ClientSelection{Route: "team-codex"}}
		return cfg
	}
	unselected, err := Export(declared())
	if err != nil {
		t.Fatal(err)
	}
	selected := declared()
	selected.SetSelectedRoute(ClientClaude, "team-claude-alternative", "")
	selected.SetSelectedRoute(ClientCodex, "manual-codex", "")
	exported, err := Export(selected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(exported, unselected) {
		t.Fatalf("bindings changed the exported manifest:\nwithout bindings:\n%s\nwith bindings:\n%s", unselected, exported)
	}
	manifest, err := Parse(exported, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	claude := manifest.Recommendations[ClientClaude]
	if claude.Primary.Route != "team-claude" || len(claude.Alternatives) != 1 || claude.Alternatives[0].Route != "team-claude-alternative" ||
		manifest.Recommendations[ClientCodex].Primary.Route != "team-codex" || manifest.Routes["manual-codex"].Account != "team" {
		t.Fatalf("exported recommendations = %#v", manifest.Recommendations)
	}
}
