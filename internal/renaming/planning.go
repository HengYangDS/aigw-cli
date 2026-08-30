package renaming

import (
	"fmt"
	"sort"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/synchronization"
)

func resolveIDs(deps Dependencies, resource string, args []string, choices []prompt.Choice) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	if !deps.Interactive {
		return "", "", fmt.Errorf("%s rename requires <old> <new> in non-interactive mode", resource)
	}
	if deps.Prompt == nil {
		return "", "", fmt.Errorf("%s rename requires an interactive prompt or explicit <old> <new> arguments", resource)
	}

	oldID := ""
	if len(args) == 1 {
		oldID = args[0]
	} else {
		if len(choices) == 0 {
			return "", "", fmt.Errorf("No %ss are configured", resource)
		}
		selected, err := deps.Prompt.Select("Select the "+resource+" to rename: ", choices)
		if err != nil {
			return "", "", fmt.Errorf("Select %s to rename: %w", resource, err)
		}
		oldID = selected
	}
	newID, err := deps.Prompt.Text("New " + resource + " ID: ")
	if err != nil {
		return "", "", fmt.Errorf("Read new %s ID: %w", resource, err)
	}
	return oldID, strings.TrimSpace(newID), nil
}

func profileChoices(cfg configuration.Config) []prompt.Choice {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]prompt.Choice, 0, len(names))
	for _, name := range names {
		profile := cfg.Profiles[name]
		label := profile.Label
		if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
			label += " · " + purpose
		}
		choices = append(choices, prompt.Choice{Value: name, Label: label})
	}
	return choices
}

func accountChoices(cfg configuration.Config) []prompt.Choice {
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]prompt.Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, prompt.Choice{Value: name, Label: cfg.Accounts[name].Label})
	}
	return choices
}

func planAccount(cfg configuration.Config, oldID, newID string) (Plan, error) {
	if !configuration.ValidProfileName(newID) {
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
	authenticationAction := "unchanged"
	if synchronization.AuthenticationChanged(cfg, next) {
		authenticationAction = "rebind-codex"
	}

	return Plan{
		Resource:           "account",
		OldID:              oldID,
		NewID:              newID,
		Status:             "planned",
		AffectedReferences: references,
		Actions: Actions{
			Configuration:  "rename-and-update-profile-references",
			APIToken:       "inspect",
			AccountProbe:   "inspect",
			Authentication: authenticationAction,
			Backup:         "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Account:       providerAccount,
	}, nil
}

func planProfile(cfg configuration.Config, oldID, newID string) (Plan, error) {
	if !configuration.ValidProfileName(newID) {
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
			Configuration:  "rename-and-update-references",
			APIToken:       "unchanged",
			AccountProbe:   "unchanged",
			Authentication: "unchanged",
			Backup:         "refresh-on-apply",
		},
		ExternalTODOs: []string{},
		Config:        next,
		Profile:       profile,
	}, nil
}
