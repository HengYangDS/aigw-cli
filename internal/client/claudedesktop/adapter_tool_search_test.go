package claudedesktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestClaudeDesktopToolSearchRequiresEveryExposedRoute(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	declareToolSearch(fixture.cfg, "selected", "alternate")
	fixture.apply(t)
	if _, enabled := readClaudeDesktopProfile(t, fixture.paths.Profile)["toolSearchEnabled"]; enabled {
		t.Fatal("tool search was enabled while the exposed manual Route lacks it")
	}

	qualified := fixture.cfg.Clone()
	declareToolSearch(qualified, "manual")
	if !fixture.adapter.ProjectionChanged(fixture.cfg, qualified) {
		t.Fatal("qualifying the last exposed Route did not change the projection")
	}
	if _, err := fixture.adapter.Apply(t.Context(), fixture.deps, fixture.cfg, qualified); err != nil {
		t.Fatal(err)
	}
	if readClaudeDesktopProfile(t, fixture.paths.Profile)["toolSearchEnabled"] != true {
		t.Fatal("tool search stayed disabled although every exposed Route declares it")
	}
	runtime, err := qualified.ResolveRuntime(configuration.ClientClaudeDesktop, "")
	if err != nil {
		t.Fatal(err)
	}
	if status := fixture.adapter.Inspect(t.Context(), fixture.deps, qualified, runtime); !status.Ready {
		t.Fatalf("inspection = %#v", status)
	}
	if !fixture.adapter.ProjectionChanged(qualified, fixture.cfg) {
		t.Fatal("withdrawing a Route's qualification did not change the projection")
	}
}

func TestShippedTeamManifestQualifiesDesktopToolSearchOnUCloudClaude(t *testing.T) {
	team, err := os.ReadFile(filepath.Join("..", "..", "..", "manifests", "team.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := configuration.Merge(configuration.NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	for routeID, want := range map[string]bool{
		"ucloud-claude-haiku-5-5":  true,
		"ucloud-claude-opus-5-5":   true,
		"ucloud-claude-sonnet-5-5": true,
		"aihubmix-claude-opus-5-5": false,
		"dmxapi-claude-opus-5-5":   false,
	} {
		cfg.SetSelectedRoute(configuration.ClientClaudeDesktop, routeID, "")
		runtime, err := cfg.ResolveRuntime(configuration.ClientClaudeDesktop, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := claudeDesktopToolSearch(cfg, runtime); got != want {
			t.Errorf("Desktop tool search with %s selected = %v, want %v", routeID, got, want)
		}
	}
}

// declareToolSearch qualifies tool search on the Anthropic interface of each Route.
func declareToolSearch(cfg configuration.Config, routeIDs ...string) {
	for _, routeID := range routeIDs {
		route := cfg.Routes[routeID]
		route.Interfaces = map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic: {configuration.CapabilityTools, configuration.Capability("tool_search")},
		}
		cfg.Routes[routeID] = route
	}
}

func readClaudeDesktopProfile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	return profile
}
