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
// binding keeps its protocol, provider and forwarding.
func (codexAdapter) AdoptSelection(_ adapter.Dependencies, cfg *domain.Config) (domain.ModelRef, error) {
	binding := cfg.Clients[ID]
	if !binding.Enabled || len(binding.Targets) == 0 {
		return domain.ModelRef{}, nil
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return domain.ModelRef{}, nil
	}
	chosen := ""
	for _, target := range binding.Targets {
		model, changed, err := ObserveSelection(target)
		if err != nil {
			return domain.ModelRef{}, err
		}
		if changed && chosen != "" && model != chosen {
			return domain.ModelRef{}, fmt.Errorf("Codex targets select different models %q and %q; run `aigw use codex <provider/model>`", chosen, model)
		}
		if changed {
			chosen = model
		}
	}
	if chosen == "" {
		return domain.ModelRef{}, nil
	}
	ref, err := modelFor(*cfg, selected, chosen)
	if err != nil {
		return domain.ModelRef{}, err
	}
	cfg.SetSelectedModel(ID, ref, "")
	return ref, nil
}

// modelFor maps Codex's model to the model of the selected provider with
// that ID, which the managed provider already reaches on the selected
// protocol.
func modelFor(cfg domain.Config, selected domain.Runtime, model string) (domain.ModelRef, error) {
	spec := codexAdapter{}.Spec()
	ref := domain.NewModelRef(selected.ProviderID, model)
	added, exists := cfg.Models[ref]
	if !exists || !slices.Contains(spec.CompatibleModelProtocols(cfg.Providers[added.Provider], added), selected.Protocol) {
		return domain.ModelRef{}, fmt.Errorf("Codex selects model %q, which provider %q has not added; run `aigw use codex %s/%s`", model, selected.ProviderID, selected.ProviderID, model)
	}
	return ref, nil
}
