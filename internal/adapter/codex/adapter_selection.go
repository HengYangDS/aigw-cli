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
	chosen := ""
	for _, target := range binding.Targets {
		model, changed, err := ObserveSelection(target)
		if err != nil {
			return domain.ModelRef{}, err
		}
		if changed && chosen != "" && model != chosen {
			return domain.ModelRef{}, adapter.RefusedSelection(recorded, fmt.Sprintf("has files that select different models, %q and %q", chosen, model), "")
		}
		if changed {
			chosen = model
		}
	}
	if chosen == "" {
		return domain.ModelRef{}, nil
	}
	ref, added := modelFor(*cfg, selected, chosen)
	if !added {
		return domain.ModelRef{}, adapter.RefusedSelection(recorded,
			fmt.Sprintf("selects model %q, which provider %q has not added", chosen, selected.ProviderID),
			"aigw use codex "+ref.String())
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
