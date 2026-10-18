package client

import (
	"encoding/json"
	"os"
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
