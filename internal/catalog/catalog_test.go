package catalog_test

import (
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/catalog"
)

func TestBuiltinTeamCatalogIsValidAndSecretFree(t *testing.T) {
	team, err := catalog.Team()
	if err != nil {
		t.Fatal(err)
	}
	if team.RecommendedDefault != "gpt-5.6-sol-cdx" || team.Accounts["dmx"].Label != "DMXAPI" {
		t.Fatalf("catalog = %#v", team)
	}
}

func TestBuiltinCatalogIncludesDefaultModelProfiles(t *testing.T) {
	team, err := catalog.Team()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gpt-5.6-sol-cdx", "gpt-5.6-terra-cdx", "gpt-5.6-luna-cdx", "gpt-5.5", "gpt-5.5-ssvip", "claude-fable-5", "claude-fable-5-ssvip", "claude-sonnet-5", "claude-sonnet-5-ssvip", "claude-opus-4-8-thinking", "claude-opus-4-8-ssvip"} {
		if _, ok := team.Profiles[name]; !ok {
			t.Fatalf("catalog missing profile %s: %#v", name, team.Profiles)
		}
	}
	if _, ok := team.Accounts["dmx"]; !ok {
		t.Fatalf("catalog missing dmx account: %#v", team.Accounts)
	}
}

func TestBuiltinCatalogExcludesMisleadingOrUnwantedModelProfiles(t *testing.T) {
	team, err := catalog.Team()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gpt-5.6", "claude-haiku-4-5-ssvip", "claude-code-1"} {
		if _, ok := team.Profiles[name]; ok {
			t.Fatalf("catalog should not include %s", name)
		}
	}
}
