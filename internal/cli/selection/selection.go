// Package selection owns explicit client Route selection.
package selection

import (
	"context"
	"fmt"
	"slices"
	"strings"

	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/prompt"

	"github.com/spf13/cobra"
)

// NewUseCommand constructs the daily client Route selection command.
func NewUseCommand(runtime invocation.Context) *cobra.Command {
	var client string
	var protocol string
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
			selected, err := resolveUseRuntime(runtime, cfg, client, name, configuration.EndpointProtocol(protocol))
			if err != nil {
				return err
			}
			route, ok := cfg.Routes[name]
			if !ok {
				return fmt.Errorf("unknown route %q; run `aigw route list`", name)
			}
			token, err := selectionToken(cmd.Context(), runtime, cfg, selected)
			if err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(runtime)
			configurationChanged, binding, err := synchronizer.SelectRoute(cmd.Context(), cfg, client, name, selected.Protocol, token)
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
			spec, _ := configuration.ClientSpecFor(client)
			r.Row("Client", spec.Label)
			r.Row("Protocol", string(selected.Protocol))
			updated := cfg.Clone()
			updated.Clients[client] = binding
			activation := clientactivation.AssessActivation(updated, runtime.Secrets)
			if action := activation.ProjectionPrerequisites[client]; action != "" {
				if token != "" {
					r.Row("Account Token", "Validated and stored")
				}
				r.Row("Projection", "Deferred; native client projection is unavailable")
				r.Next(action)
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
	cmd.Flags().StringVar(&protocol, "protocol", "", "Endpoint protocol for a multi-protocol Route")
	return cmd
}

func resolveUseRuntime(runtime invocation.Context, cfg configuration.Config, client, routeID string, requested configuration.EndpointProtocol) (configuration.Runtime, error) {
	_, selected, err := invocation.Synchronizer(runtime).PrepareSelection(cfg, client, routeID, requested)
	if err == nil || requested != "" {
		return selected, err
	}
	route := cfg.Routes[routeID]
	spec, _ := configuration.ClientSpecFor(client)
	compatible := spec.CompatibleRouteProtocols(cfg.Accounts[route.Account], route)
	if len(compatible) < 2 {
		return configuration.Runtime{}, err
	}
	choices := make([]prompt.Choice, 0, len(compatible))
	names := make([]string, 0, len(compatible))
	for _, candidate := range compatible {
		name := string(candidate)
		choices = append(choices, prompt.Choice{Value: name, Label: name})
		names = append(names, name)
	}
	if !runtime.Interactive {
		return configuration.Runtime{}, fmt.Errorf("Route %q has multiple compatible protocols for %s; pass --protocol <%s>", routeID, client, strings.Join(names, "|"))
	}
	choice, err := runtime.Prompt.Select("Select the endpoint protocol: ", choices)
	if err != nil {
		return configuration.Runtime{}, err
	}
	_, selected, err = invocation.Synchronizer(runtime).PrepareSelection(cfg, client, routeID, configuration.EndpointProtocol(choice))
	return selected, err
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

func selectionToken(ctx context.Context, runtime invocation.Context, cfg configuration.Config, selected configuration.Runtime) (string, error) {
	if !selected.UsesAIGWCredentialStore() {
		return "", nil
	}
	available, err := runtime.Secrets.Exists(selected.AccountID)
	if err != nil {
		return "", fmt.Errorf("cannot inspect Account %q credential: %w", selected.AccountID, err)
	}
	if available {
		return "", nil
	}
	instruction, writable := credential.TokenRecovery(runtime.Secrets, selected.AccountID)
	if !writable {
		retry := fmt.Sprintf("aigw use --for %s %s", selected.Client, selected.RouteID)
		spec, _ := configuration.ClientSpecFor(selected.Client)
		if len(spec.CompatibleRouteProtocols(cfg.Accounts[selected.AccountID], cfg.Routes[selected.RouteID])) > 1 {
			retry = fmt.Sprintf("aigw use --for %s --protocol %s %s", selected.Client, selected.Protocol, selected.RouteID)
		}
		return "", fmt.Errorf("account %q is missing a token; %s; then run `%s`", selected.AccountID, instruction, retry)
	}
	if !runtime.Interactive {
		return "", fmt.Errorf("account %q is missing a token; %s", selected.AccountID, instruction)
	}
	account := cfg.Accounts[selected.AccountID]
	token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
	if err != nil {
		return "", err
	}
	if err := credential.ValidateRuntime(ctx, runtime.HTTP, selected, token); err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}
	return token, nil
}

func chooseRoute(runtime invocation.Context, cfg configuration.Config, client, label string) (string, error) {
	choices := make([]prompt.Choice, 0, len(cfg.Routes))
	for _, id := range cfg.RouteIDs() {
		compatible, err := cfg.CompatibleClientIDs(id)
		if err != nil || !slices.Contains(compatible, client) {
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
