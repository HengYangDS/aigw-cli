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
	references := make([]string, 0, len(next.Profiles))
	for profileID, profile := range next.Profiles {
		if profile.Account != oldID {
			continue
		}
		profile.Account = newID
		next.Profiles[profileID] = profile
		references = append(references, "profiles."+profileID+".account")
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return Plan{}, fmt.Errorf("Validate account rename: %w", err)
	}

	return Plan{
		Resource:           "account",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
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
	profile, ok := cfg.Profiles[oldID]
	if !ok {
		return Plan{}, fmt.Errorf("Unknown profile %q", oldID)
	}
	if _, exists := cfg.Profiles[newID]; exists {
		return Plan{}, fmt.Errorf("Profile %q already exists", newID)
	}

	next := cfg.Clone()
	delete(next.Profiles, oldID)
	next.Profiles[newID] = profile
	references := make([]string, 0, len(next.Routes))
	for client, profileID := range next.Routes {
		if profileID != oldID {
			continue
		}
		next.Routes[client] = newID
		references = append(references, "routes."+client)
	}
	sort.Strings(references)
	if err := next.Validate(); err != nil {
		return Plan{}, fmt.Errorf("Validate profile rename: %w", err)
	}

	return Plan{
		Resource:           "profile",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
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
