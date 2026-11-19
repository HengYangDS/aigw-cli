package store

import (
	"slices"
	"testing"

	"aigw-cli/internal/domain"
)

// A team member may hold any one provider's Token, and setup fills a client
// from its recommendations alone: so every provider that has a model of the
// family that a client's first recommendation names, which the client can
// use, appears among that client's recommendations. The test reads the
// file's structure, never which models it names.
func TestTeamManifestRecommendsEveryProviderThatServesTheFamily(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	if len(manifest.Recommendations) == 0 {
		t.Fatal("the team file recommends nothing")
	}
	for client, recommendation := range manifest.Recommendations {
		family := manifest.Models[recommendation.Primary.Model].Family
		var recommended []string
		for _, choice := range recommendation.Selections() {
			recommended = append(recommended, choice.Model.Provider())
		}
		for ref, model := range manifest.Models {
			if model.Family != family || slices.Contains(recommended, ref.Provider()) {
				continue
			}
			if !usableBy(manifest, client, ref) {
				continue
			}
			t.Errorf("%s serves %s's family %s with %s, but %s recommends none of %s's models", ref.Provider(), client, family, ref, client, ref.Provider())
		}
	}
}

// usableBy reports whether client can use the model as the file declares it:
// by its own choice of protocol, or under any one of the model's protocols,
// which a recommendation may name.
func usableBy(manifest domain.Config, client string, ref domain.ModelRef) bool {
	if _, err := manifest.ResolveSelection(client, domain.ClientSelection{Model: ref}); err == nil {
		return true
	}
	for protocol := range manifest.Models[ref].Protocols {
		if _, err := manifest.ResolveSelection(client, domain.ClientSelection{Model: ref, Protocol: protocol}); err == nil {
			return true
		}
	}
	return false
}

// Each client's recommendations name at most one model of a provider, in the
// order in which the file declares its providers, which the rollout guide
// states: a person who holds several Tokens gets the first provider's model,
// whichever client it is. The test reads the order from the file.
func TestTeamManifestRecommendsProvidersInTheFilesOrder(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	order := manifest.ProviderIDs()
	for client, recommendation := range manifest.Recommendations {
		last := -1
		for _, choice := range recommendation.Selections() {
			position := slices.Index(order, choice.Model.Provider())
			if position <= last {
				t.Errorf("%s recommends %s after a model of a provider that the file declares at or after %s's: the file's order is %v",
					client, choice.Model, choice.Model.Provider(), order)
			}
			last = position
		}
	}
}
