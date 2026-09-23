package renaming

import (
	"fmt"
	"sort"

	configuration "aigw-cli/internal/configuration"
)

func planAccount(cfg configuration.Config, oldID, newID string) (Plan, error) {
	if !configuration.ValidIdentifier(newID) {
		return Plan{}, fmt.Errorf("Invalid new account ID %q", newID)
	}
	providerAccount, ok := cfg.Accounts[oldID]
	if !ok {
		return Plan{}, fmt.Errorf("Unknown account %q", oldID)
	}
	if _, exists := cfg.Accounts[newID]; exists {
		return Plan{}, fmt.Errorf("Account %q already exists", newID)
	}

	next := cfg.Clone()
	delete(next.Accounts, oldID)
	providerAccount.ID = newID
	next.Accounts[newID] = providerAccount
	references := make([]string, 0, len(next.Routes))
	for routeID, route := range next.Routes {
		if route.Account != oldID {
			continue
		}
		route.Account = newID
		next.Routes[routeID] = route
		references = append(references, "routes."+routeID+".account")
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return Plan{}, fmt.Errorf("Validate account rename: %w", err)
	}

	return Plan{
		Resource:           ResourceAccount,
		OldID:              oldID,
		NewID:              newID,
		Status:             StatusPlanned,
		AffectedReferences: references,
		Actions: Actions{
			Configuration: "rename-and-update-route-references",
			APIToken:      "inspect",
			AccountProbe:  "inspect",
			Backup:        "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Account:       providerAccount,
	}, nil
}

func planRoute(cfg configuration.Config, oldID, newID string) (Plan, error) {
	if !configuration.ValidIdentifier(newID) {
		return Plan{}, fmt.Errorf("Invalid new route ID %q", newID)
	}
	route, ok := cfg.Routes[oldID]
	if !ok {
		return Plan{}, fmt.Errorf("Unknown route %q", oldID)
	}
	if _, exists := cfg.Routes[newID]; exists {
		return Plan{}, fmt.Errorf("Route %q already exists", newID)
	}

	next := cfg.Clone()
	delete(next.Routes, oldID)
	next.Routes[newID] = route
	references := make([]string, 0, len(next.Clients)+len(next.Recommendations))
	for client, binding := range next.Clients {
		if binding.Route == oldID {
			binding.Route = newID
			next.Clients[client] = binding
			references = append(references, "clients."+client+".route")
		}
	}
	for client, recommendation := range next.Recommendations {
		changed := false
		if recommendation.Primary.Route == oldID {
			recommendation.Primary.Route = newID
			references = append(references, "recommendations."+client+".primary.route")
			changed = true
		}
		for index := range recommendation.Alternatives {
			if recommendation.Alternatives[index].Route != oldID {
				continue
			}
			recommendation.Alternatives[index].Route = newID
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
		Resource:           ResourceRoute,
		OldID:              oldID,
		NewID:              newID,
		Status:             StatusPlanned,
		AffectedReferences: references,
		Actions: Actions{
			Configuration: "rename-and-update-references",
			APIToken:      "unchanged",
			AccountProbe:  "unchanged",
			Backup:        "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Route:         route,
	}, nil
}
