package model

import (
	"fmt"
	"maps"
	"slices"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// pinRecommendations keeps every recommendation choice of the model with key
// on the protocol that it reaches before an edit of the model's protocols:
// a choice without a protocol records the one that it resolves to, so that new
// protocols cannot make it ambiguous. A choice whose protocol the edit
// removes refuses the edit, because a recommendation belongs to the team
// file, which `aigw setup --from` applies and `aigw config export` writes;
// the model never moves a choice by itself. It returns the clients whose
// recommendation records a protocol that it did not.
func pinRecommendations(cfg *domain.Config, key string, ref domain.ModelRef, protocols []domain.EndpointProtocol) ([]string, error) {
	pinned := []string{}
	for _, client := range slices.Sorted(maps.Keys(cfg.Recommendations)) {
		recommendation := cfg.Recommendations[client]
		choices := recommendation.Selections()
		changed := false
		for index, choice := range choices {
			if choice.Model != key {
				continue
			}
			// A loaded configuration is valid, so every choice resolves.
			protocol, _ := cfg.ResolveSelection(client, choice)
			if !slices.Contains(protocols, protocol) {
				label := cfg.Admission().Label(client)
				return nil, terminal.ProblemError(
					fmt.Sprintf("%s's recommendation names %s on %s, which this edit would remove", label, ref, domain.ProtocolLabel(protocol)),
					"Recommendations come from the team file: change this one there, then apply the file with `aigw setup --from <file>`.", "",
					"aigw config export", nil)
			}
			if choice.Protocol == "" {
				choices[index].Protocol = protocol
				changed = true
			}
		}
		if !changed {
			continue
		}
		cfg.Recommendations[client] = domain.ClientRecommendation{Primary: choices[0], Alternatives: choices[1:]}
		pinned = append(pinned, client)
	}
	return pinned, nil
}
