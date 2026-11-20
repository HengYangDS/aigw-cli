package codex

import (
	"fmt"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// AdoptSelection records Codex's own model choice as its Client Binding when
// the root model is the only changed value AIGW owns in the Codex targets and
// it names a model of the selected provider on the selected protocol. The
// binding keeps its protocol, provider and forwarding. A model that the
// provider has not added, or targets that name different models, are refused
// with the command that restores the recorded selection.
func (codexAdapter) AdoptSelection(_ adapter.Dependencies, cfg *domain.Config) (domain.ModelRef, error) {
	binding := cfg.Clients[ID]
	if !binding.Enabled || len(binding.Targets) == 0 {
		return domain.ModelRef{}, nil
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return domain.ModelRef{}, nil
	}
	recorded := adapter.Recorded(codexAdapter{}.Spec(), selected)
	var chosen []string
	for _, target := range binding.Targets {
		observed, err := observeSelection(target)
		if err != nil {
			return domain.ModelRef{}, err
		}
		switch observed.state {
		case selectionDrifted:
			// An outside edit in any target is refused by the plan under
			// its own name; nothing is followed or refused past it here.
			return domain.ModelRef{}, nil
		case selectionChosen:
			if !slices.Contains(chosen, observed.model) {
				chosen = append(chosen, observed.model)
			}
		case selectionRecorded:
		}
	}
	switch len(chosen) {
	case 0:
		return domain.ModelRef{}, nil
	case 1:
	default:
		return domain.ModelRef{}, adapter.RefusedSelection(recorded, fmt.Sprintf("has files that select different models, %q and %q", chosen[0], chosen[1]), "")
	}
	return followChosen(cfg, selected, recorded, chosen[0])
}

// followChosen records chosen, the model that Codex's files select, as its
// selection, or refuses a model that the selected provider has not added on
// the selected protocol.
func followChosen(cfg *domain.Config, selected domain.Runtime, recorded adapter.RecordedSelection, chosen string) (domain.ModelRef, error) {
	ref, added := modelFor(*cfg, selected, chosen)
	if !added {
		return domain.ModelRef{}, adapter.RefusedSelection(recorded,
			fmt.Sprintf("selects model %q, which provider %q has not added on %s", chosen, selected.ProviderID, domain.ProtocolLabel(selected.Protocol)),
			adapter.SelectCommand(ID, ref, ""))
	}
	if ref == selected.Model {
		// The binding already names Codex's choice; only AIGW's record of
		// the file is behind, which the next write of the file brings up.
		return domain.ModelRef{}, nil
	}
	cfg.SetSelectedModel(ID, ref, "")
	return ref, nil
}

// modelFor maps Codex's model to the model of the selected provider with
// that ID, and reports whether that model is added on the selected protocol,
// which the managed provider already reaches.
func modelFor(cfg domain.Config, selected domain.Runtime, model string) (domain.ModelRef, bool) {
	spec := codexAdapter{}.Spec()
	ref := domain.NewModelRef(selected.ProviderID, model)
	added, exists := cfg.Models[ref]
	return ref, exists && slices.Contains(spec.CompatibleModelProtocols(cfg.Providers[added.Provider], added), selected.Protocol)
}
