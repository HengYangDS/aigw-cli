// Package profile owns model-profile command behavior.
package profile

import (
	"encoding/json"
	"fmt"
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
	root := &cobra.Command{Use: "profile", Short: "Manage service profiles"}
	root.AddCommand(newAddCommand(runtime), newListCommand(runtime), newShowCommand(runtime), newEditCommand(runtime), renameCommand, newRemoveCommand(runtime))
	return root
}

func newAddCommand(runtime invocation.Context) *cobra.Command {
	var accountName, client, model, label, purpose string
	cmd := &cobra.Command{
		Use: "add <profile>", Short: "Add a model profile to an existing account", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			if !configuration.ValidProfileName(profileName) {
				return fmt.Errorf("Invalid profile ID %q; use letters, numbers, dots, hyphens, or underscores", profileName)
			}
			if accountName == "" || client == "" || model == "" {
				return fmt.Errorf("--account, --for, and --model are required; run `aigw profile add --help`")
			}
			if !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be %s; run `aigw profile add --help`", configuration.AdmittedClientUsage())
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Profiles[profileName]; exists {
				return fmt.Errorf("Profile %q already exists", profileName)
			}
			account, exists := cfg.Accounts[accountName]
			if !exists {
				return fmt.Errorf("Unknown account %q; first run `aigw add %s ...`", accountName, accountName)
			}
			account.ID = accountName
			if _, err := account.EndpointFor(client); err != nil {
				return err
			}
			if label == "" {
				label = profileName
			}
			before := cfg.Clone()
			cfg.Profiles[profileName] = configuration.Profile{Label: label, Purpose: strings.TrimSpace(purpose), Account: accountName, Client: client, Model: model}
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
			r.Success("Reused the existing account token; current route was not changed")
			r.Next("aigw use " + profileName)
			return nil
		},
	}
	cmd.Flags().StringVar(&accountName, "account", "", "Existing account ID")
	cmd.Flags().StringVar(&client, "for", "", "Client: "+configuration.AdmittedClientUsage())
	cmd.Flags().StringVar(&model, "model", "", "Upstream model ID")
	cmd.Flags().StringVar(&label, "label", "", "Display name")
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose note (display only)")
	return cmd
}

func newListCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List service profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Service profiles")
			r.Section("Available profiles")
			for _, name := range cfg.ProfileIDs() {
				state, stateText := presentation.Info, "Available"
				profile := cfg.Profiles[name]
				if cfg.Routes[profile.Client] == name {
					state, stateText = presentation.OK, "Selected for "+invocation.Title(profile.Client)
				}
				accountName := profile.Account
				available, err := runtime.Secrets.Exists(accountName)
				if err != nil {
					return fmt.Errorf("observe credential for Account %q: %w", accountName, err)
				}
				secret := "Token missing"
				if available {
					secret = "Token available"
				}
				detail := []string{}
				if profile.Client != "" {
					detail = append(detail, invocation.Title(profile.Client))
				}
				detail = append(detail, choiceLabel(profile), stateText, "Account "+accountName, secret)
				r.StatusLine(state, "Configuration", name)
				r.Detail(strings.Join(detail, " · "))
			}
			r.Next("aigw use")
			return nil
		},
	}
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
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("Unknown profile %q", args[0])
			}
			accountName := profile.Account
			account := cfg.Accounts[accountName]
			available, err := runtime.Secrets.Exists(accountName)
			if err != nil {
				return fmt.Errorf("observe credential for Account %q: %w", accountName, err)
			}
			if jsonMode {
				return json.NewEncoder(runtime.Out).Encode(map[string]any{"id": args[0], "label": profile.Label, "purpose": profile.Purpose, "account": accountName, "client": profile.Client, "model": profile.Model, "endpoints": account.Endpoints, "secret_available": available})
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Service details")
			r.Section("Service profiles")
			r.Row("Profile ID", args[0])
			r.Row("Name", profile.Label)
			if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Row("Account", accountName)
			r.Row("Client", invocation.Title(profile.Client))
			r.Row("Model", profile.Model)
			if account.Endpoints.OpenAIResponses != "" {
				r.Row("OpenAI", account.Endpoints.OpenAIResponses)
			}
			if account.Endpoints.Anthropic != "" {
				r.Row("Anthropic", account.Endpoints.Anthropic)
			}
			state, text := presentation.Warn, "Missing"
			if available {
				state, text = presentation.OK, "Available"
			}
			r.Status(state, "System secret", text)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func newEditCommand(runtime invocation.Context) *cobra.Command {
	var label, purpose string
	cmd := &cobra.Command{
		Use: "edit <profile>", Short: "Update profile display metadata", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" && !cmd.Flags().Changed("purpose") {
				return fmt.Errorf("Nothing to update; provide --label or --purpose; use `aigw account edit <account>` for endpoints")
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			before := cfg.Clone()
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("Unknown profile %q", args[0])
			}
			if label != "" {
				profile.Label = label
			}
			if cmd.Flags().Changed("purpose") {
				profile.Purpose = strings.TrimSpace(purpose)
			}
			cfg.Profiles[args[0]] = profile
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "profile"); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Profile updated")
			r.Row("Configuration", args[0])
			if invocation.Synchronizer(runtime).ProjectionChanged(before, cfg) {
				r.Success("Client configuration synchronized")
			} else {
				r.Success("Display metadata saved; client configuration was not changed")
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "New display name")
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose note (pass an empty value to clear)")
	return cmd
}

func newRemoveCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use: "remove <profile>", Short: "Remove an unused profile", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			before := cfg.Clone()
			name := args[0]
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("Unknown profile %q", name)
			}
			for client, route := range cfg.Routes {
				if route == name {
					return fmt.Errorf("Profile %q is selected for %s; first run `aigw use <other-%s-profile>`", name, client, client)
				}
			}
			delete(cfg.Profiles, name)
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

func choiceLabel(profile configuration.Profile) string {
	label := profile.Label
	if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
		return label + " · " + purpose
	}
	return label
}
