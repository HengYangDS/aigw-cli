// Package route owns explicit client-route command behavior.
package route

import (
	"context"
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/prompt"

	"github.com/spf13/cobra"
)

// NewUseCommand constructs the daily route-selection command.
func NewUseCommand(runtime invocation.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Select a profile for its declared client",
		Args: cobra.MatchAll(cobra.MaximumNArgs(1), func(_ *cobra.Command, args []string) error {
			if len(args) == 0 && !runtime.Interactive {
				return fmt.Errorf("Non-interactive use requires a profile; run `aigw use <profile>`")
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			var name string
			if len(args) == 1 {
				name = args[0]
			} else {
				name, err = chooseProfile(runtime, cfg, "Select the Profile to use: ")
				if err != nil {
					return err
				}
			}
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("Unknown profile %q; run `aigw profile list`", name)
			}
			client := profile.Client
			token, err := selectionToken(cmd.Context(), runtime, cfg, name)
			if err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(runtime)
			configurationChanged, err := synchronizer.SelectProfile(cmd.Context(), cfg, name, token)
			if err != nil {
				return err
			}
			title, detail := "Profile already selected", "Selected client configuration synchronized; route unchanged"
			switch {
			case configurationChanged:
				title, detail = "Profile selected", "Client configuration synchronized"
			case token != "":
				title, detail = "Token stored", "Account token stored; selected client configuration synchronized"
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle(title)
			r.Section("Current selection")
			r.Row("Profile", profile.Label)
			if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Row("Client", invocation.Title(client))
			r.Success(detail)
			r.Next("aigw check")
			return r.Err()
		},
	}
	return cmd
}

func selectionToken(ctx context.Context, runtime invocation.Context, cfg configuration.Config, name string) (string, error) {
	profile := cfg.Profiles[name]
	selected, err := cfg.ResolveRuntime(profile.Client, name)
	if err != nil {
		return "", err
	}
	if !selected.UsesAIGWCredentialStore() {
		return "", nil
	}
	available, err := runtime.Secrets.Exists(profile.Account)
	if err != nil {
		return "", fmt.Errorf("Cannot inspect Account %q credential: %w", profile.Account, err)
	}
	if available {
		return "", nil
	}
	instruction, writable := credential.TokenRecovery(runtime.Secrets, profile.Account)
	if !writable {
		return "", fmt.Errorf("Account %q is missing a token; %s; then run `aigw use %s`", profile.Account, instruction, name)
	}
	if !runtime.Interactive {
		return "", fmt.Errorf("Account %q is missing a token; %s", profile.Account, instruction)
	}
	account := cfg.Accounts[profile.Account]
	token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
	if err != nil {
		return "", err
	}
	account.ID = profile.Account
	if err := credential.Validate(ctx, runtime.HTTP, account, token, profile.Client); err != nil {
		return "", fmt.Errorf("Token validation failed: %w", err)
	}
	return token, nil
}

// NewCommand constructs the route command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{
		Use:   "route",
		Short: "Manage client routes",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("Choose a route subcommand; run `aigw route --help`")
		},
	}
	root.AddCommand(newListCommand(runtime))
	return root
}

func newListCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{Use: "list", Short: "Show each client's selected profile", Args: cobra.NoArgs}
	command.RunE = func(_ *cobra.Command, _ []string) error { return runListWithFormat(runtime, jsonMode) }
	command.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return command
}

type routeListOutput struct {
	Routes []routeListItem `json:"routes"`
}

type routeSelectionState string

const (
	routeSelected   routeSelectionState = "selected"
	routeUnselected routeSelectionState = "unselected"
)

type routeListItem struct {
	Client     string              `json:"client"`
	State      routeSelectionState `json:"state"`
	Profile    string              `json:"profile,omitempty"`
	Label      string              `json:"label,omitempty"`
	Purpose    string              `json:"purpose,omitempty"`
	NextAction string              `json:"next_action,omitempty"`
}

func runListWithFormat(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return invocation.Problem(runtime, "Not configured", "No Profiles have been created.", "No client route is available to inspect.", "aigw setup", fmt.Errorf("not configured"))
	}
	result := routeListOutput{Routes: make([]routeListItem, 0, len(configuration.AdmittedClientIDs()))}
	nextCommand := ""
	for _, client := range configuration.AdmittedClientIDs() {
		clientRuntime, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			item := routeListItem{Client: client, State: routeUnselected}
			if suggested := cfg.FirstProfileForClient(client); suggested != "" {
				item.NextAction = "aigw use " + suggested
				if nextCommand == "" {
					nextCommand = item.NextAction
				}
			}
			result.Routes = append(result.Routes, item)
			continue
		}
		profile := cfg.Profiles[clientRuntime.ProfileID]
		result.Routes = append(result.Routes, routeListItem{
			Client: client, State: routeSelected, Profile: clientRuntime.ProfileID,
			Label: profile.Label, Purpose: strings.TrimSpace(profile.Purpose),
		})
	}
	if jsonMode {
		return presentation.WriteJSON(runtime.Out, result)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Current routes")
	r.Section("Clients")
	for _, item := range result.Routes {
		if item.State == routeUnselected {
			message := "No " + invocation.Title(item.Client) + " profile selected"
			if item.NextAction != "" {
				message += " · " + item.NextAction
			}
			r.Status(presentation.Warn, invocation.Title(item.Client), message)
			continue
		}
		r.Status(presentation.OK, invocation.Title(item.Client), item.Profile)
		r.Detail(profileChoiceLabel(configuration.Profile{Label: item.Label, Purpose: item.Purpose}))
	}
	if nextCommand == "" {
		nextCommand = "aigw use <profile>"
	}
	r.Next(nextCommand)
	return nil
}

func chooseProfile(runtime invocation.Context, cfg configuration.Config, label string) (string, error) {
	choices := make([]prompt.Choice, 0, len(cfg.Profiles))
	for _, id := range cfg.ProfileIDs() {
		choices = append(choices, prompt.Choice{Value: id, Label: profileChoiceLabel(cfg.Profiles[id])})
	}
	return runtime.Prompt.Select(label, choices)
}

func profileChoiceLabel(profile configuration.Profile) string {
	if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
		return profile.Label + " · " + purpose
	}
	return profile.Label
}
