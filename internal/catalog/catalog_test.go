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
	if team.RecommendedDefault != "dmx" || team.Profiles["dmx"].Label != "DMXAPI" {
		t.Fatalf("catalog = %#v", team)
	}
}
