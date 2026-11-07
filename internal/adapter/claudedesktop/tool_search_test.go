package claudedesktop

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

func TestProjectionOwnsTheToolSearchSetting(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-opus-5-5"}},
		ToolSearch:           true,
	}
	applyProjection(t, paths, desired)
	requireJSONValues(t, paths.Profile, map[string]any{"toolSearchEnabled": true})

	desired.ToolSearch = false
	applyProjection(t, paths, desired)
	if _, present := readJSON(t, paths.Profile)["toolSearchEnabled"]; present {
		t.Fatal("tool search stayed in the profile after its qualification was withdrawn")
	}

	profile := readJSON(t, paths.Profile)
	profile["toolSearchEnabled"] = true
	writeJSON(t, paths.Profile, profile)
	if _, err := Prepare(paths, &desired); err == nil || !strings.Contains(err.Error(), "changed outside AIGW") {
		t.Fatalf("tool search enabled outside AIGW: error = %v, want managed drift", err)
	}
}

func applyProjection(t *testing.T, paths Paths, desired Desired) {
	t.Helper()
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
}

// Desktop's tool search is on while every model that its profile lists
// declares it, and a model of the selected provider and protocol that
// declares nothing, as aigw use adds one, turns it off, which the registry
// reports as lost with that model; a search that was already off, and a
// disabled Desktop, have none to lose.
func TestRegistryReportsALostToolSearch(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	declareToolSearch(fixture.cfg, "selected", "alternate", "manual")
	registry, err := adapter.NewRegistry(New())
	if err != nil {
		t.Fatal(err)
	}
	if enabled, lacking := (claudeDesktopAdapter{}).ToolSearch(fixture.cfg); !enabled || len(lacking) != 0 {
		t.Fatalf("tool search = %t, lacking %q, although every listed model declares it", enabled, lacking)
	}
	added := fixture.cfg.Clone()
	added.Families["fresh"] = domain.Family{Label: "fresh"}
	added.Models["fresh"] = domain.Model{Provider: added.Models["selected"].Provider, Family: "fresh", UpstreamModel: "fresh",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	want := []adapter.ToolSearchLoss{{Client: ID, Models: []string{"fresh"}}}
	if lost := registry.ToolSearchLost(fixture.cfg, added); !reflect.DeepEqual(lost, want) {
		t.Fatalf("lost tool search = %#v, want %#v", lost, want)
	}
	if lost := registry.ToolSearchLost(added, fixture.cfg); len(lost) != 0 {
		t.Fatalf("a regained tool search was reported lost: %#v", lost)
	}
	twice := added.Clone()
	twice.Families["second"] = domain.Family{Label: "second"}
	twice.Models["second"] = domain.Model{Provider: added.Models["selected"].Provider, Family: "second", UpstreamModel: "second",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	if lost := registry.ToolSearchLost(added, twice); len(lost) != 0 {
		t.Fatalf("a tool search that was already off was reported lost: %#v", lost)
	}
	if enabled, lacking := (claudeDesktopAdapter{}).ToolSearch(twice); enabled || !slices.Equal(lacking, []string{"fresh", "second"}) {
		t.Fatalf("tool search = %t, lacking %q, want off for fresh and second", enabled, lacking)
	}
	disabled := fixture.cfg.Clone()
	disabled.SetClientActivation(ID, false, "", nil)
	if enabled, lacking := (claudeDesktopAdapter{}).ToolSearch(disabled); enabled || len(lacking) != 0 {
		t.Fatalf("a disabled Desktop reported tool search %t, lacking %q", enabled, lacking)
	}
}
