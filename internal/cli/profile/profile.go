// Package profile owns model-profile command behavior.
package profile

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

// NewCommand constructs the profile command tree. renameCommand is supplied
// by the rename domain because rename is a credential-aware transaction, not
// ordinary profile metadata editing.
func NewCommand(runtime invocation.Context, renameCommand *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "profile", Short: "Manage Profiles"}
	root.AddCommand(newAddCommand(runtime), newListCommand(runtime), newShowCommand(runtime), newEditCommand(runtime), renameCommand, newRemoveCommand(runtime))
	return root
}

func newAddCommand(runtime invocation.Context) *cobra.Command {
	var accountName, model, label, purpose, protocolName string
	cmd := &cobra.Command{
		Use: "add <profile>", Short: "Add a model profile to an existing account",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			if !configuration.ValidIdentifier(args[0]) {
				return fmt.Errorf("Invalid profile ID %q; use letters, numbers, dots, hyphens, or underscores; run `%s --help`", args[0], cmd.CommandPath())
			}
			if accountName != "" && !configuration.ValidIdentifier(accountName) {
				return fmt.Errorf("Invalid account ID %q; run `%s --help`", accountName, cmd.CommandPath())
			}
			if strings.TrimSpace(accountName) == "" || strings.TrimSpace(model) == "" || strings.TrimSpace(protocolName) == "" {
				return fmt.Errorf("--account, --model, and --protocol are required; run `%s --help`", cmd.CommandPath())
			}
			protocol := configuration.EndpointProtocol(protocolName)
			switch protocol {
			case configuration.ProtocolAnthropic, configuration.ProtocolOpenAIResponses, configuration.ProtocolOpenAIChatCompletions:
				return nil
			default:
				return fmt.Errorf("Unsupported protocol %q; run `%s --help`", protocolName, cmd.CommandPath())
			}
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Routes[profileName]; exists {
				return fmt.Errorf("Profile %q already exists", profileName)
			}
			_, exists := cfg.Accounts[accountName]
			if !exists {
				return fmt.Errorf("Unknown account %q; first run `aigw add %s ...`", accountName, accountName)
			}
			if label == "" {
				label = profileName
			}
			before := cfg.Clone()
			protocol := configuration.EndpointProtocol(protocolName)
			cfg.Routes[profileName] = configuration.Route{
				Label: label, Purpose: strings.TrimSpace(purpose), Account: accountName, Model: model,
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{protocol: {}},
			}
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "profile add"); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Model profile added")
			r.Row("Configuration", profileName)
			r.Row("Account", accountName)
			r.Row("Model", model)
			if purpose := strings.TrimSpace(purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Success("Reused the existing Account; no client selection was changed")
			r.Next("aigw use --for <client> " + profileName)
			return nil
		},
	}
	cmd.Flags().StringVar(&accountName, "account", "", "Existing account ID")
	cmd.Flags().StringVar(&model, "model", "", "Upstream model ID")
	cmd.Flags().StringVar(&protocolName, "protocol", "", "Wire protocol: anthropic, openai_responses, or openai_chat_completions")
	cmd.Flags().StringVar(&label, "label", "", "Display name")
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose note (display only)")
	return cmd
}

func newListCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "list", Short: "List Profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			result := profileListOutput{Profiles: make([]profileListItem, 0, len(cfg.Routes))}
			for _, name := range cfg.RouteIDs() {
				item, collectErr := collectProfileListItem(runtime, cfg, name)
				if collectErr != nil {
					return collectErr
				}
				result.Profiles = append(result.Profiles, item)
			}
			if jsonMode {
				return presentation.WriteJSON(runtime.Out, result)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Profiles")
			r.Section("Available profiles")
			for _, item := range result.Profiles {
				state, stateText := presentation.Info, "Available"
				if len(item.SelectedClients) > 0 {
					state, stateText = presentation.OK, "Selected for "+strings.Join(item.SelectedClients, ", ")
				}
				detail := []string{choiceLabel(configuration.Route{Label: item.Label, Purpose: item.Purpose}), stateText, "Account " + item.Account}
				if len(item.CompatibleClients) > 0 {
					detail = append(detail, "Clients "+strings.Join(item.CompatibleClients, ", "))
				}
				r.StatusLine(state, "Configuration", item.ID)
				r.Detail(strings.Join(detail, " · "))
			}
			r.Next("aigw use")
			return r.Err()
		},
	}
	command.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return command
}

type profileListOutput struct {
	Profiles []profileListItem `json:"profiles"`
}

type profileListItem struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Purpose           string   `json:"purpose,omitempty"`
	Account           string   `json:"account"`
	CompatibleClients []string `json:"compatible_clients"`
	SelectedClients   []string `json:"selected_clients"`
	Model             string   `json:"model"`
}

func collectProfileListItem(runtime invocation.Context, cfg configuration.Config, name string) (profileListItem, error) {
	profile := cfg.Routes[name]
	compatible, err := cfg.CompatibleClientIDs(name)
	if err != nil {
		return profileListItem{}, err
	}
	return profileListItem{
		ID: name, Label: profile.Label, Purpose: profile.Purpose, Account: profile.Account,
		CompatibleClients: compatible, SelectedClients: cfg.SelectedClientsForRoute(name), Model: profile.Model,
	}, nil
}

func newShowCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use: "show <profile>", Short: "Show secret-free profile metadata", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Routes[args[0]]
			if !ok {
				return fmt.Errorf("Unknown profile %q", args[0])
			}
			accountName := profile.Account
			account := cfg.Accounts[accountName]
			compatible, err := cfg.CompatibleClientIDs(args[0])
			if err != nil {
				return err
			}
			selected := cfg.SelectedClientsForRoute(args[0])
			if jsonMode {
				result := map[string]any{
					"id": args[0], "label": profile.Label, "purpose": profile.Purpose,
					"account": accountName, "model": profile.Model,
					"compatible_clients": compatible, "selected_clients": selected,
					"endpoints": account.Endpoints,
				}
				return presentation.WriteJSON(runtime.Out, result)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Profile details")
			r.Section("Profile")
			r.Row("Profile ID", args[0])
			r.Row("Name", profile.Label)
			if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Row("Account", accountName)
			r.Row("Model", profile.Model)
			r.Row("Compatible clients", strings.Join(compatible, ", "))
			if len(selected) > 0 {
				r.Row("Selected for", strings.Join(selected, ", "))
			}
			if account.Endpoints.OpenAIResponses != "" {
				r.Row("OpenAI", account.Endpoints.OpenAIResponses)
			}
			if account.Endpoints.Anthropic != "" {
				r.Row("Anthropic", account.Endpoints.Anthropic)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func newEditCommand(runtime invocation.Context) *cobra.Command {
	var label, purpose string
	cmd := &cobra.Command{
		Use: "edit <profile>", Short: "Update profile display metadata",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			if !configuration.ValidIdentifier(args[0]) {
				return fmt.Errorf("Invalid profile ID %q; run `%s --help`", args[0], cmd.CommandPath())
			}
			if cmd.Flags().Changed("label") && strings.TrimSpace(label) == "" {
				return fmt.Errorf("--label requires a non-empty value; run `%s --help`", cmd.CommandPath())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			before := cfg.Clone()
			profile, ok := cfg.Routes[args[0]]
			if !ok {
				return fmt.Errorf("Unknown profile %q", args[0])
			}
			if label != "" {
				profile.Label = label
			}
			if cmd.Flags().Changed("purpose") {
				profile.Purpose = strings.TrimSpace(purpose)
			}
			cfg.Routes[args[0]] = profile
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "profile"); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Profile updated")
			r.Row("Configuration", args[0])
			r.Success("Profile metadata saved")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "New display name")
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose note (pass an empty value to clear)")
	cmd.MarkFlagsOneRequired("label", "purpose")
	return cmd
}

func newRemoveCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use: "remove <profile>", Short: "Remove an unused profile",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			if !configuration.ValidIdentifier(args[0]) {
				return fmt.Errorf("Invalid profile ID %q; run `%s --help`", args[0], cmd.CommandPath())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			before := cfg.Clone()
			name := args[0]
			profile, ok := cfg.Routes[name]
			if !ok {
				return fmt.Errorf("Unknown profile %q", name)
			}
			for client, binding := range cfg.Clients {
				if binding.Route == name {
					return fmt.Errorf("Profile %q is selected for %s; first run `aigw use --for %s <other-profile>`", name, client, client)
				}
			}
			delete(cfg.Routes, name)
			for client, recommendation := range cfg.Recommendations {
				recommendation.Alternatives = slices.DeleteFunc(recommendation.Alternatives, func(selection configuration.ClientSelection) bool {
					return selection.Route == name
				})
				if recommendation.Primary.Route != name {
					cfg.Recommendations[client] = recommendation
					continue
				}
				if len(recommendation.Alternatives) == 0 {
					delete(cfg.Recommendations, client)
					continue
				}
				recommendation.Primary = recommendation.Alternatives[0]
				recommendation.Alternatives = recommendation.Alternatives[1:]
				cfg.Recommendations[client] = recommendation
			}
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "profile remove"); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Profile removed")
			r.Row("Configuration", name)
			r.Row("Account", profile.Account)
			r.Success("The account and token remain in place")
			return nil
		},
	}
}

func choiceLabel(profile configuration.Route) string {
	label := profile.Label
	if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
		return label + " · " + purpose
	}
	return label
}
