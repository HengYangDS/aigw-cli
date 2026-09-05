// Package route owns explicit client-route command behavior.
package route

import (
	"context"
	"fmt"
	"reflect"
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
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			before := cfg.Clone()
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				if !runtime.Interactive {
					return fmt.Errorf("Non-interactive use requires a profile; run `aigw use <profile>`")
				}
				name, err = chooseProfile(runtime, cfg, "Select the AI service to use: ")
				if err != nil {
					return err
				}
			}
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("Unknown profile %q; run `aigw profile list`", name)
			}
			client := profile.Client
			accountID := profile.Account
			providerAccount := cfg.Accounts[accountID]
			addedToken := false
			clientRuntime, err := cfg.ResolveRuntime(client, name)
			if err != nil {
				return err
			}
			available := !clientRuntime.RequiresAccountToken()
			if clientRuntime.RequiresAccountToken() {
				available, err = runtime.Secrets.Exists(accountID)
				if err != nil {
					return fmt.Errorf("Cannot inspect Account %q credential: %w", accountID, err)
				}
			}
			if clientRuntime.RequiresAccountToken() && !available {
				instruction, writable := credential.TokenRecovery(runtime.Secrets, accountID)
				if !writable {
					return fmt.Errorf("Account %q is missing a token; %s; then run `aigw use %s`", accountID, instruction, name)
				}
				if !runtime.Interactive {
					return fmt.Errorf("Account %q is missing a token; %s", accountID, instruction)
				}
				token, err := runtime.Prompt.Secret("Paste " + providerAccount.Label + " token: ")
				if err != nil {
					return err
				}
				providerAccount.ID = accountID
				if err := credential.Validate(context.Background(), runtime.HTTP, providerAccount, token, client); err != nil {
					return fmt.Errorf("Token validation failed: %w", err)
				}
				if err := runtime.Secrets.Set(accountID, token); err != nil {
					return err
				}
				addedToken = true
			}
			cfg.Routes[client] = name
			synchronizer := invocation.Synchronizer(runtime)
			cfg, _, err = synchronizer.DesiredClientConfiguration(cfg, client)
			if err != nil {
				if addedToken {
					_ = runtime.Secrets.Delete(accountID)
				}
				return err
			}
			if reflect.DeepEqual(before, cfg) {
				r := invocation.Renderer(runtime)
				r.ProductTitle("Service already selected")
				r.Row("Service", profile.Label)
				r.Row("Client", invocation.Title(client))
				r.Success("Configuration, credentials, and client files were unchanged")
				r.Next("aigw check")
				return r.Err()
			}
			if err := synchronizer.Commit(cmd.Context(), before, cfg, "route"); err != nil {
				if addedToken {
					_ = runtime.Secrets.Delete(accountID)
				}
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Service switched")
			r.Section("Current selection")
			r.Row("Service", profile.Label)
			if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Row("Client", invocation.Title(client))
			r.Success("Client configuration synchronized")
			r.Next("aigw check")
			return r.Err()
		},
	}
	return cmd
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
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "Show each client's selected profile", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			return runList(runtime)
		}},
	)
	return root
}

// runList answers the narrow question "which profile will each client use?".
// Operational readiness remains owned by the readiness command group.
func runList(runtime invocation.Context) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return invocation.Problem(runtime, "Not configured", "No service profiles have been created.", "No client route is available to inspect.", "aigw setup", fmt.Errorf("not configured"))
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Current routes")
	r.Section("Clients")
	nextCommand := ""
	for _, client := range configuration.AdmittedClientIDs() {
		clientRuntime, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			message := "No " + invocation.Title(client) + " profile selected"
			if suggested := cfg.FirstProfileForClient(client); suggested != "" {
				command := "aigw use " + suggested
				message += " · " + command
				if nextCommand == "" {
					nextCommand = command
				}
			}
			r.Status(presentation.Warn, invocation.Title(client), message)
			continue
		}
		profile := cfg.Profiles[clientRuntime.ProfileID]
		r.Status(presentation.OK, invocation.Title(client), clientRuntime.ProfileID)
		r.Detail(profileChoiceLabel(profile))
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
