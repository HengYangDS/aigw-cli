package codex

import (
	"fmt"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// AdoptSelection records Codex's own model choice as its Client Binding when
// the root model is the only changed value AIGW owns in the Codex targets and
// it names the upstream model of one Route of the selected Account and
// protocol. The binding keeps its protocol, provider and forwarding.
func (codexAdapter) AdoptSelection(_ adapter.Dependencies, cfg *domain.Config) (string, error) {
	binding := cfg.Clients[ID]
	if !binding.Enabled || len(binding.Targets) == 0 {
		return "", nil
	}
	selected, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		return "", nil
	}
	chosen := ""
	for _, target := range binding.Targets {
		model, changed, err := ObserveSelection(target)
		if err != nil {
			return "", err
		}
		if changed && chosen != "" && model != chosen {
			return "", fmt.Errorf("Codex targets select different models %q and %q; run `aigw use codex <provider/model>`", chosen, model)
		}
		if changed {
			chosen = model
		}
	}
	if chosen == "" {
		return "", nil
	}
	routeID, err := modelFor(*cfg, selected, chosen)
	if err != nil {
		return "", err
	}
	cfg.SetSelectedModel(ID, routeID, "")
	return routeID, nil
}

// modelFor maps Codex's model to the one Route with the selected Account,
// protocol and that upstream model, which the managed provider already reaches.
func modelFor(cfg domain.Config, selected domain.Runtime, model string) (string, error) {
	spec := codexAdapter{}.Spec()
	var matches []string
	for _, routeID := range cfg.CatalogModelIDs() {
		route := cfg.Models[routeID]
		if route.Provider != selected.ProviderID || route.UpstreamModelID() != model {
			continue
		}
		if slices.Contains(spec.CompatibleModelProtocols(cfg.Providers[route.Provider], route), selected.Protocol) {
			matches = append(matches, routeID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("Codex selects model %q, which provider %q has not added; run `aigw use codex <provider/model>`", model, selected.ProviderID)
	default:
		// Only a configuration from before models were unique per provider
		// can hold several, which no selection tells apart.
		return "", fmt.Errorf("Codex selects model %q, which %d models of provider %q carry; correct the configuration, whose file `aigw config path` names", model, len(matches), selected.ProviderID)
	}
}
