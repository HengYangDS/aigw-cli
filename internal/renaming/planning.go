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
	for profileID, profile := range next.Routes {
		if profile.Account != oldID {
			continue
		}
		profile.Account = newID
		next.Routes[profileID] = profile
		references = append(references, "profiles."+profileID+".account")
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
			Configuration: "rename-and-update-profile-references",
			APIToken:      "inspect",
			AccountProbe:  "inspect",
			Backup:        "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Account:       providerAccount,
	}, nil
}

func planProfile(cfg configuration.Config, oldID, newID string) (Plan, error) {
	if !configuration.ValidIdentifier(newID) {
		return Plan{}, fmt.Errorf("Invalid new profile ID %q", newID)
	}
	profile, ok := cfg.Routes[oldID]
	if !ok {
		return Plan{}, fmt.Errorf("Unknown profile %q", oldID)
	}
	if _, exists := cfg.Routes[newID]; exists {
		return Plan{}, fmt.Errorf("Profile %q already exists", newID)
	}

	next := cfg.Clone()
	delete(next.Routes, oldID)
	next.Routes[newID] = profile
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
		return Plan{}, fmt.Errorf("Validate profile rename: %w", err)
	}

	return Plan{
		Resource:           ResourceProfile,
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
		Profile:       profile,
	}, nil
}
