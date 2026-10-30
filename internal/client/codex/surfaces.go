package codex

import (
	"sort"
	"strings"

	"aigw-cli/internal/client/discovery"
	surfaceidentity "aigw-cli/internal/client/surface"
)

// HomeDefault identifies the canonical default Codex Home projection surface.
const HomeDefault surfaceidentity.ID = "codex-home-default"

const homeExplicitPrefix = "codex-home-explicit-"

// HomeExplicit returns the stable identity for an explicitly selected Codex
// home.
func HomeExplicit(identifier string) surfaceidentity.ID {
	return surfaceidentity.ID(homeExplicitPrefix + identifier)
}

// IsHome reports whether the identity is the default or an explicit Codex home
// shared by Codex CLI and Codex Desktop. AIGW owns every Codex home projection.
func IsHome(id surfaceidentity.ID) bool {
	return id == HomeDefault || strings.HasPrefix(string(id), homeExplicitPrefix)
}

// AutoManagedTargets returns Codex homes that AIGW may project without
// further operator admission. Presence remains an observed readiness fact; it
// does not determine whether the declared default surface may be created.
func AutoManagedTargets(result discovery.Result) []string {
	targets := make([]string, 0)
	for _, surface := range result.Surfaces {
		if surface.AutoManaged && IsHome(surfaceidentity.ID(surface.ID)) && surface.ConfigPath != "" {
			targets = append(targets, surface.ConfigPath)
		}
	}
	sort.Strings(targets)
	return targets
}
