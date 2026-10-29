package codex

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/client"
	"aigw-cli/internal/domain"
)

// AdoptSelection records Codex's own model choice as its Client Binding when
// the root model is the only changed value AIGW owns in the Codex targets and
// it names the upstream model of one Route of the selected Account and
// protocol. The binding keeps its protocol, provider and forwarding.
func (codexAdapter) AdoptSelection(_ client.Dependencies, cfg *domain.Config) (string, error) {
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
			return "", fmt.Errorf("Codex targets select different models %q and %q; run `aigw use --for codex <route>`", chosen, model)
		}
		if changed {
			chosen = model
		}
	}
	if chosen == "" {
		return "", nil
	}
	routeID, err := routeFor(*cfg, selected, chosen)
	if err != nil {
		return "", err
	}
	cfg.SetSelectedRoute(ID, routeID, "")
	return routeID, nil
}

// routeFor maps Codex's model to the one Route with the selected Account,
// protocol and that upstream model, which the managed provider already reaches.
func routeFor(cfg domain.Config, selected domain.Runtime, model string) (string, error) {
	spec := codexAdapter{}.Spec()
	var matches []string
	for _, routeID := range cfg.CatalogRouteIDs() {
		route := cfg.Routes[routeID]
		if route.Account != selected.AccountID || route.UpstreamModelID() != model {
			continue
		}
		if slices.Contains(spec.CompatibleRouteProtocols(cfg.Accounts[route.Account], route), selected.Protocol) {
			matches = append(matches, routeID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("Codex selects model %q, which no Route of Account %q provides; run `aigw use --for codex <route>`", model, selected.AccountID)
	default:
		commands := make([]string, 0, len(matches))
		for _, routeID := range matches {
			commands = append(commands, fmt.Sprintf("`aigw use --for codex %s`", routeID))
		}
		return "", fmt.Errorf("Codex selects model %q, which several Routes provide; run one of %s", model, strings.Join(commands, ", "))
	}
}
