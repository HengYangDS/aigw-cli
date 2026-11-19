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
			if _, err := manifest.ResolveSelection(client, domain.ClientSelection{Model: ref}); err != nil {
				continue
			}
			t.Errorf("%s serves %s's family %s with %s, but %s recommends none of %s's models", ref.Provider(), client, family, ref, client, ref.Provider())
		}
	}
}
