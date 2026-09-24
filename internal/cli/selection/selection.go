// Package selection owns explicit client Route selection.
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

// NewUseCommand constructs the daily client Route selection command.
func NewUseCommand(runtime invocation.Context) *cobra.Command {
	var client string
	cmd := &cobra.Command{
		Use:   "use <route>",
		Short: "Select a Route for one client",
		Args: cobra.MatchAll(cobra.MaximumNArgs(1), func(_ *cobra.Command, args []string) error {
			if len(args) == 0 && !runtime.Interactive {
				return fmt.Errorf("non-interactive use requires a Route; run `aigw use --for <client> <route>`")
			}
			if len(args) == 1 && client == "" && !runtime.Interactive {
				return fmt.Errorf("non-interactive use requires --for; run `aigw use --for <client> <route>`")
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
			route, ok := cfg.Routes[name]
			if !ok {
				return fmt.Errorf("unknown route %q; run `aigw route list`", name)
			}
			token, err := selectionToken(cmd.Context(), runtime, cfg, client, name)
			if err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(runtime)
			configurationChanged, binding, err := synchronizer.SelectRoute(cmd.Context(), cfg, client, name, token)
			if err != nil {
				return err
			}
			title, detail := "Route already selected", "Selected client configuration synchronized; binding unchanged"
			switch {
			case configurationChanged && token != "":
				title, detail = "Route selected", "Account token stored; client configuration synchronized"
			case configurationChanged:
				title, detail = "Route selected", "Client configuration synchronized"
			case token != "":
				title, detail = "Token stored", "Account token stored; selected client configuration synchronized"
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle(title)
			r.Section("Current selection")
			r.Row("Route", cfg.RouteLabel(name))
			if purpose := strings.TrimSpace(route.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Row("Client", invocation.Title(client))
			spec, _ := configuration.ClientSpecFor(client)
			if spec.RestartAfterProjection && (binding.Executable == "" || len(binding.Targets) == 0) {
				r.Row("Projection", fmt.Sprintf("Deferred; %s is not installed", spec.Label))
				r.Next(fmt.Sprintf("Install %s, then run `aigw sync`", spec.Label))
				return r.Err()
			}
			r.Success(detail)
			if spec.RestartAfterProjection && configurationChanged {
				r.Row("Activation", "Restart required")
				r.Next(fmt.Sprintf("Restart %s, then run `aigw check`", spec.Label))
			} else {
				r.Next("aigw check")
			}
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
		route, err := chooseRoute(runtime, cfg, client, "Select the Route to use: ")
		return client, route, err
	}

	route := args[0]
	if _, ok := cfg.Routes[route]; !ok {
		return "", "", fmt.Errorf("unknown route %q; run `aigw route list`", route)
	}
	if client != "" {
		return client, route, nil
	}
	compatible, err := cfg.CompatibleClientIDs(route)
	if err != nil {
		return "", "", err
	}
	client, err = chooseClientIDs(runtime, compatible)
	return client, route, err
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
	route := cfg.Routes[name]
	selected, err := cfg.ResolveRuntime(client, name)
	if err != nil {
		return "", err
	}
	if !selected.UsesAIGWCredentialStore() {
		return "", nil
	}
	available, err := runtime.Secrets.Exists(route.Account)
	if err != nil {
		return "", fmt.Errorf("cannot inspect Account %q credential: %w", route.Account, err)
	}
	if available {
		return "", nil
	}
	instruction, writable := credential.TokenRecovery(runtime.Secrets, route.Account)
	if !writable {
		return "", fmt.Errorf("account %q is missing a token; %s; then run `aigw use --for %s %s`", route.Account, instruction, client, name)
	}
	if !runtime.Interactive {
		return "", fmt.Errorf("account %q is missing a token; %s", route.Account, instruction)
	}
	account := cfg.Accounts[route.Account]
	token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
	if err != nil {
		return "", err
	}
	account.ID = route.Account
	if err := credential.Validate(ctx, runtime.HTTP, account, token, client); err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}
	return token, nil
}

func chooseRoute(runtime invocation.Context, cfg configuration.Config, client, label string) (string, error) {
	choices := make([]prompt.Choice, 0, len(cfg.Routes))
	for _, id := range cfg.RouteIDs() {
		if _, err := cfg.ResolveRuntime(client, id); err != nil {
			continue
		}
		choices = append(choices, prompt.Choice{Value: id, Label: routeChoiceLabel(cfg, id)})
	}
	return runtime.Prompt.Select(label, choices)
}

func routeChoiceLabel(cfg configuration.Config, routeID string) string {
	label := cfg.RouteLabel(routeID)
	route := cfg.Routes[routeID]
	if purpose := strings.TrimSpace(route.Purpose); purpose != "" {
		return label + " · " + purpose
	}
	return label
}
