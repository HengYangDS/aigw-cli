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
	var client string
	var all bool
	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Switch the active AI service",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && client != "" {
				return fmt.Errorf("--all and --for cannot be used together; run `aigw use --help`")
			}
			if client != "" && !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be %s; run `aigw use --help`", configuration.AdmittedClientUsage())
			}
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
			accountID, providerAccount, err := cfg.ResolveAccount(name)
			if err != nil {
				return err
			}
			addedToken := false
			if !runtime.Secrets.Has(accountID) {
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
				if err := credential.Validate(context.Background(), runtime.HTTP, providerAccount, token); err != nil {
					return fmt.Errorf("Token validation failed: %w", err)
				}
				if err := runtime.Secrets.Set(accountID, token); err != nil {
					return err
				}
				addedToken = true
			}
			switch {
			case all:
				cfg.Routes.Default = name
				cfg.Routes.Overrides = map[string]string{}
			case client != "":
				cfg.Routes.Overrides[client] = name
			default:
				cfg.Routes.Default = name
			}
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "route"); err != nil {
				if addedToken {
					_ = runtime.Secrets.Delete(accountID)
				}
				return err
			}
			r := renderer(runtime)
			r.ProductTitle("Service switched")
			r.Section("Current selection")
			r.Row("Service", profile.Label)
			if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			scope := "Default route"
			if client != "" {
				scope = invocation.Title(client)
			} else if all {
				scope = "All clients"
			}
			r.Row("Scope", scope)
			r.Success("Client configuration synchronized")
			r.Next("aigw check")
			return r.Err()
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Set only "+configuration.AdmittedClientUsage())
	cmd.Flags().BoolVar(&all, "all", false, "Set the default route and clear client overrides")
	return cmd
}

// NewCommand constructs the route command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "route", Short: "Manage client routes"}
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "Show the default route and client overrides", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			return runList(runtime)
		}},
		newResetCommand(runtime),
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
		return invocation.Problem(runtime, "Not configured", "No service profiles have been created.", "No default route or client override is available to inspect.", "aigw setup", fmt.Errorf("not configured"))
	}
	r := renderer(runtime)
	r.ProductTitle("Current routes")
	r.Section("Default route")
	profile := cfg.Profiles[cfg.Routes.Default]
	r.Status(presentation.OK, "Default profile", cfg.Routes.Default)
	r.Detail(profileChoiceLabel(profile))
	r.Section("Client")
	nextCommand := ""
	for _, client := range configuration.AdmittedClientIDs() {
		clientRuntime, inherited, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			message := "No " + invocation.Title(client) + " profile selected"
			if suggested := cfg.FirstProfileForClient(client); suggested != "" {
				command := "aigw use " + suggested + " --for " + client
				message += " · " + command
				if nextCommand == "" {
					nextCommand = command
				}
			}
			r.Status(presentation.Warn, invocation.Title(client), message)
			continue
		}
		mode := "Inherits default"
		if !inherited {
			mode = "Explicit override"
		}
		profile := cfg.Profiles[clientRuntime.ProfileID]
		r.Status(presentation.OK, invocation.Title(client), clientRuntime.ProfileID+" · "+mode)
		r.Detail(profileChoiceLabel(profile))
	}
	if nextCommand == "" {
		nextCommand = "aigw use <profile> --for <" + strings.Join(configuration.AdmittedClientIDs(), "|") + ">"
	}
	r.Next(nextCommand)
	return nil
}

func newResetCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use: "reset <client>", Short: "Restore a client's inheritance of the default route", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := args[0]
			if !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("Client must be %s; run `aigw route reset --help`", configuration.AdmittedClientUsage())
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			before := cfg.Clone()
			delete(cfg.Routes.Overrides, client)
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "route reset"); err != nil {
				return err
			}
			r := renderer(runtime)
			r.ProductTitle("Route reset")
			r.Row("Client", invocation.Title(client))
			r.Success("Now inherits the default service")
			r.Next("aigw check")
			return r.Err()
		},
	}
}

func renderer(runtime invocation.Context) *presentation.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	return presentation.NewWithWidth(out, runtime.Color, runtime.Width)
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
