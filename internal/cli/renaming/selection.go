package renaming

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/prompt"
	"fmt"
	"sort"
	"strings"
)

func resolveIDs(runtime invocation.Context, resource string, args []string) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return "", "", err
	}
	choices := profileChoices(cfg)
	if resource == "account" {
		choices = accountChoices(cfg)
	}
	var oldID string
	if len(args) == 1 {
		oldID = args[0]
	} else {
		if len(choices) == 0 {
			return "", "", fmt.Errorf("No %ss are configured", resource)
		}
		selected, err := runtime.Prompt.Select("Select the "+resource+" to rename: ", choices)
		if err != nil {
			return "", "", fmt.Errorf("Select %s to rename: %w", resource, err)
		}
		oldID = selected
	}
	newID, err := runtime.Prompt.Text("New " + resource + " ID: ")
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
