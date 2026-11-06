package renaming

import (
	"fmt"
	"sort"

	"aigw-cli/internal/domain"
)

func planProvider(cfg domain.Config, oldID, newID string) (Plan, error) {
	if !domain.ValidIdentifier(newID) {
		return Plan{}, fmt.Errorf("Invalid new account ID %q", newID)
	}
	providerAccount, ok := cfg.Providers[oldID]
	if !ok {
		return Plan{}, fmt.Errorf("Unknown account %q", oldID)
	}
	if _, exists := cfg.Providers[newID]; exists {
		return Plan{}, fmt.Errorf("Account %q already exists", newID)
	}

	next := cfg.Clone()
	delete(next.Providers, oldID)
	providerAccount.ID = newID
	next.Providers[newID] = providerAccount
	references := make([]string, 0, len(next.Models))
	for routeID, route := range next.Models {
		if route.Provider != oldID {
			continue
		}
		route.Provider = newID
		next.Models[routeID] = route
		references = append(references, "routes."+routeID+".account")
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return Plan{}, fmt.Errorf("Validate account rename: %w", err)
	}

	return Plan{
		Resource:           ResourceProvider,
		OldID:              oldID,
		NewID:              newID,
		Status:             StatusPlanned,
		AffectedReferences: references,
		Actions: Actions{
			Configuration: "rename_and_update_route_references",
			APIToken:      "inspect",
			BalanceProbe:  "inspect",
			Backup:        "refresh_on_apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Provider:      providerAccount,
	}, nil
}

func planModel(cfg domain.Config, oldID, newID string) (Plan, error) {
	if !domain.ValidIdentifier(newID) {
		return Plan{}, fmt.Errorf("Invalid new route ID %q", newID)
	}
	route, ok := cfg.Models[oldID]
	if !ok {
		return Plan{}, fmt.Errorf("Unknown route %q", oldID)
	}
	if _, exists := cfg.Models[newID]; exists {
		return Plan{}, fmt.Errorf("Route %q already exists", newID)
	}

	next := cfg.Clone()
	delete(next.Models, oldID)
	next.Models[newID] = route
	references := make([]string, 0, len(next.Clients)+len(next.Recommendations))
	for client, binding := range next.Clients {
		if binding.Model == oldID {
			binding.Model = newID
			next.Clients[client] = binding
			references = append(references, "clients."+client+".route")
		}
	}
	for client, recommendation := range next.Recommendations {
		changed := false
		if recommendation.Primary.Model == oldID {
			recommendation.Primary.Model = newID
			references = append(references, "recommendations."+client+".primary.route")
			changed = true
		}
		for index := range recommendation.Alternatives {
			if recommendation.Alternatives[index].Model != oldID {
				continue
			}
			recommendation.Alternatives[index].Model = newID
			references = append(references, fmt.Sprintf("recommendations.%s.alternatives.%d.route", client, index))
			changed = true
		}
		if changed {
			next.Recommendations[client] = recommendation
		}
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return Plan{}, fmt.Errorf("Validate route rename: %w", err)
	}

	return Plan{
		Resource:           ResourceModel,
		OldID:              oldID,
		NewID:              newID,
		Status:             StatusPlanned,
		AffectedReferences: references,
		Actions: Actions{
			Configuration: "rename_and_update_references",
			APIToken:      "unchanged",
			BalanceProbe:  "unchanged",
			Backup:        "refresh_on_apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Model:         route,
	}, nil
}
