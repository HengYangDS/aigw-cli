package codex

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/discovery"
	surfaceidentity "aigw-cli/internal/adapter/surface"
)

func TestHomeIdentitiesAreStable(t *testing.T) {
	if got := string(HomeDefault); got != "codex-home-default" {
		t.Fatalf("default Codex Home = %q", got)
	}
	if got := string(HomeExplicit("0123456789ab")); got != "codex-home-explicit-0123456789ab" {
		t.Fatalf("explicit Codex Home = %q", got)
	}
}

func TestIsHomeClassifiesCodexHomes(t *testing.T) {
	tests := []struct {
		name string
		id   surfaceidentity.ID
		want bool
	}{
		{name: "default", id: HomeDefault, want: true},
		{name: "explicit", id: HomeExplicit("0123456789ab"), want: true},
		{name: "legacy empty explicit identifier", id: HomeExplicit(""), want: true},
		{name: "embedded prefix", id: surfaceidentity.ID("other-codex-home-explicit-0123456789ab"), want: false},
		{name: "empty", id: surfaceidentity.ID(""), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHome(tt.id); got != tt.want {
				t.Fatalf("IsHome() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Codex homes are AIGW-owned: a desired target accepts the AIGW authority and
// refuses any other.
func TestCodexHomesHaveTheAIGWAuthority(t *testing.T) {
	for _, id := range []surfaceidentity.ID{HomeDefault, HomeExplicit("0123456789ab")} {
		target := TargetRef{SurfaceID: string(id), Authority: string(surfaceidentity.AuthorityAIGW), ProjectionMode: ProjectionFullSelection}
		if err := validateDesiredCodexTarget(target); err != nil {
			t.Fatalf("%s with the AIGW authority = %v", id, err)
		}
		target.Authority = "foreign"
		if err := validateDesiredCodexTarget(target); err == nil {
			t.Fatalf("%s accepted a foreign authority", id)
		}
	}
	unknown := TargetRef{SurfaceID: "unknown", Authority: string(surfaceidentity.AuthorityAIGW), ProjectionMode: ProjectionFullSelection}
	if err := validateDesiredCodexTarget(unknown); err == nil {
		t.Fatal("an unknown surface was accepted as a Codex target")
	}
}

func TestCodexTargetValidationAcceptsStandaloneAndUnknownPaths(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), "config.toml")
	discovered := discovery.Result{Surfaces: []discovery.Surface{{
		ID: string(HomeDefault), ConfigPath: codexHome,
	}}}
	if err := (codexAdapter{}).ValidateTarget(discovered, codexHome); err != nil {
		t.Fatalf("Codex Home target rejected: %v", err)
	}
	if err := (codexAdapter{}).ValidateTarget(discovered, filepath.Join(t.TempDir(), "explicit.toml")); err != nil {
		t.Fatalf("explicit target rejected: %v", err)
	}
}
