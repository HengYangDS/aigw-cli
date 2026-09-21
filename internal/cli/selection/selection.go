// Package selection owns explicit client Profile selection.
package selection

import (
	"context"
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/prompt"

	"github.com/spf13/cobra"
)

// NewUseCommand constructs the daily client Profile selection command.
func NewUseCommand(runtime invocation.Context) *cobra.Command {
	var client string
	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Select a Profile for one client",
		Args: cobra.MatchAll(cobra.MaximumNArgs(1), func(_ *cobra.Command, args []string) error {
			if len(args) == 0 && !runtime.Interactive {
				return fmt.Errorf("non-interactive use requires a Profile; run `aigw use --for <client> <profile>`")
			}
			if len(args) == 1 && client == "" && !runtime.Interactive {
				return fmt.Errorf("non-interactive use requires --for; run `aigw use --for <client> <profile>`")
			}
			if client != "" && !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be %s; run `aigw use --help`", configuration.AdmittedClientUsage())
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
			client, name, err := resolveUseSelection(runtime, cfg, client, args)
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("unknown profile %q; run `aigw profile list`", name)
			}
			token, err := selectionToken(cmd.Context(), runtime, cfg, client, name)
			if err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(runtime)
			configurationChanged, err := synchronizer.SelectProfile(cmd.Context(), cfg, client, name, token)
			if err != nil {
				return err
			}
			title, detail := "Profile already selected", "Selected client configuration synchronized; binding unchanged"
			switch {
			case configurationChanged && token != "":
				title, detail = "Profile selected", "Account token stored; client configuration synchronized"
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
	cmd.Flags().StringVar(&client, "for", "", "Client: "+configuration.AdmittedClientUsage())
	return cmd
}

func resolveUseSelection(runtime invocation.Context, cfg configuration.Config, client string, args []string) (string, string, error) {
	if len(args) == 0 {
		if client == "" {
			var err error
			client, err = chooseClient(runtime, configuration.AdmittedClientSpecs())
			if err != nil {
				return "", "", err
			}
		}
		profile, err := chooseProfile(runtime, cfg, client, "Select the Profile to use: ")
		return client, profile, err
	}

	profile := args[0]
	if _, ok := cfg.Profiles[profile]; !ok {
		return "", "", fmt.Errorf("unknown profile %q; run `aigw profile list`", profile)
	}
	if client != "" {
		return client, profile, nil
	}
	compatible, err := cfg.CompatibleClientIDs(profile)
	if err != nil {
		return "", "", err
	}
	client, err = chooseClientIDs(runtime, compatible)
	return client, profile, err
}

func chooseClient(runtime invocation.Context, specs []configuration.ClientSpec) (string, error) {
	choices := make([]prompt.Choice, 0, len(specs))
	for _, spec := range specs {
		choices = append(choices, prompt.Choice{Value: spec.ID, Label: spec.Label})
	}
	return runtime.Prompt.Select("Select the client to configure: ", choices)
}

func chooseClientIDs(runtime invocation.Context, clients []string) (string, error) {
	specs := make([]configuration.ClientSpec, 0, len(clients))
	for _, client := range clients {
		spec, _ := configuration.ClientSpecFor(client)
		specs = append(specs, spec)
	}
	return chooseClient(runtime, specs)
}

func selectionToken(ctx context.Context, runtime invocation.Context, cfg configuration.Config, client, name string) (string, error) {
	profile := cfg.Profiles[name]
	selected, err := cfg.ResolveRuntime(client, name)
	if err != nil {
		return "", err
	}
	if !selected.UsesAIGWCredentialStore() {
		return "", nil
	}
	available, err := runtime.Secrets.Exists(profile.Account)
	if err != nil {
		return "", fmt.Errorf("cannot inspect Account %q credential: %w", profile.Account, err)
	}
	if available {
		return "", nil
	}
	instruction, writable := credential.TokenRecovery(runtime.Secrets, profile.Account)
	if !writable {
		return "", fmt.Errorf("account %q is missing a token; %s; then run `aigw use --for %s %s`", profile.Account, instruction, client, name)
	}
	if !runtime.Interactive {
		return "", fmt.Errorf("account %q is missing a token; %s", profile.Account, instruction)
	}
	account := cfg.Accounts[profile.Account]
	token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
	if err != nil {
		return "", err
	}
	account.ID = profile.Account
	if err := credential.Validate(ctx, runtime.HTTP, account, token, client); err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}
	return token, nil
}

func chooseProfile(runtime invocation.Context, cfg configuration.Config, client, label string) (string, error) {
	choices := make([]prompt.Choice, 0, len(cfg.Profiles))
	for _, id := range cfg.ProfileIDs() {
		if _, err := cfg.ResolveRuntime(client, id); err != nil {
			continue
		}
		choices = append(choices, prompt.Choice{Value: id, Label: profileChoiceLabel(cfg.Profiles[id])})
	}
	return runtime.Prompt.Select(label, choices)
}

func profileChoiceLabel(profile configuration.Profile) string {
	label := profile.Label
	if profile.Tier != "" {
		label += " · " + strings.ToUpper(string(profile.Tier[:1])) + string(profile.Tier[1:])
	}
	if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
		return label + " · " + purpose
	}
	return label
}
