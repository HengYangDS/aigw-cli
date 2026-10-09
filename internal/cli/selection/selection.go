// Package selection owns explicit client Route selection.
package selection

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/prompt"

	"github.com/spf13/cobra"
)

// NewUseCommand constructs the daily client Route selection command.
func NewUseCommand(runtime invocation.Context) *cobra.Command {
	var options useOptions
	cmd := &cobra.Command{
		Use:   "use <route>",
		Short: "Select a Route for one client",
		Args: cobra.MatchAll(cobra.MaximumNArgs(1), func(cmd *cobra.Command, args []string) error {
			return options.validate(cmd, args, runtime.Interactive)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.run(runtime, cmd, args)
		},
	}
	cmd.Flags().StringVar(&options.client, "for", "", "Client: "+configuration.AdmittedClientUsage())
	cmd.Flags().StringVar(&options.forwardingEndpoint, "forwarding-endpoint", "", "Codex forwarding destination; retain the Account upstream")
	cmd.Flags().BoolVar(&options.direct, "direct", false, "Restore the selected Codex Account endpoint")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Preview selection without credentials, network requests or writes")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the credential-free selection preview as JSON")
	return cmd
}

type useOptions struct {
	client, forwardingEndpoint string
	direct, dryRun, jsonMode   bool
}

func (o useOptions) validate(cmd *cobra.Command, args []string, interactive bool) error {
	if o.jsonMode && !o.dryRun {
		return fmt.Errorf("--json requires --dry-run")
	}
	if o.direct && cmd.Flags().Changed("forwarding-endpoint") {
		return fmt.Errorf("--direct and --forwarding-endpoint are mutually exclusive")
	}
	if len(args) == 0 && !interactive {
		return fmt.Errorf("non-interactive use requires a Route; run `aigw use --for <client> <route>`")
	}
	if len(args) == 1 && o.client == "" && !interactive {
		return fmt.Errorf("non-interactive use requires --for; run `aigw use --for <client> <route>`")
	}
	if o.client != "" && !configuration.IsAdmittedClient(o.client) {
		return fmt.Errorf("--for must be %s; run `aigw use --help`", configuration.AdmittedClientUsage())
	}
	return nil
}

func (o useOptions) destination(cmd *cobra.Command) ([]string, error) {
	if o.direct {
		return []string{""}, nil
	}
	if !cmd.Flags().Changed("forwarding-endpoint") {
		return nil, nil
	}
	if strings.TrimSpace(o.forwardingEndpoint) == "" {
		return nil, fmt.Errorf("--forwarding-endpoint requires an endpoint; use --direct to remove forwarding")
	}
	return []string{o.forwardingEndpoint}, nil
}

func (o useOptions) run(runtime invocation.Context, cmd *cobra.Command, args []string) error {
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	clientID, name, err := resolveUseSelection(runtime, cfg, o.client, args)
	if err != nil {
		return err
	}
	if _, exists := cfg.Routes[name]; !exists {
		return fmt.Errorf("unknown route %q; run `aigw route list`", name)
	}
	destination, err := o.destination(cmd)
	if err != nil {
		return err
	}
	synchronizer := invocation.Synchronizer(runtime)
	after, selected, err := synchronizer.PrepareSelection(cfg, clientID, name, destination...)
	if err != nil {
		return err
	}
	if o.dryRun {
		synchronizer.AuthorizeCodexRouteSelection = clientID == configuration.ClientCodex
		plans, err := synchronizer.Plan(cfg, after, clientID)
		if err != nil {
			return err
		}
		preview := selectionPreview{
			DryRun: true, Client: clientID, Route: name, Endpoint: selected.Endpoint,
			UpstreamEndpoint:     selected.UpstreamEndpoint,
			ConfigurationChanged: !reflect.DeepEqual(cfg.Clients[clientID], after.Clients[clientID]),
			Projections:          plans,
		}
		return renderSelectionPreview(runtime, cfg, preview, o.jsonMode)
	}
	token, err := selectionToken(cmd.Context(), runtime, cfg, clientID, name)
	if err != nil {
		return err
	}
	changed, binding, err := synchronizer.SelectRoute(cmd.Context(), cfg, clientID, name, token, destination...)
	if err != nil {
		return err
	}
	return renderSelectionResult(runtime, cfg, clientID, name, binding, changed, token != "")
}

func renderSelectionResult(runtime invocation.Context, cfg configuration.Config, clientID, name string, binding configuration.ClientBinding, changed, tokenStored bool) error {
	title, detail := "Route already selected", "Selected client configuration synchronized; binding unchanged"
	switch {
	case changed && tokenStored:
		title, detail = "Route selected", "Account token stored; client configuration synchronized"
	case changed:
		title, detail = "Route selected", "Client configuration synchronized"
	case tokenStored:
		title, detail = "Token stored", "Account token stored; selected client configuration synchronized"
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle(title)
	r.Section("Current selection")
	r.Row("Route", cfg.RouteLabel(name))
	if purpose := strings.TrimSpace(cfg.Routes[name].Purpose); purpose != "" {
		r.Row("Purpose", purpose)
	}
	r.Row("Client", invocation.Title(clientID))
	spec, _ := configuration.ClientSpecFor(clientID)
	if spec.RestartAfterProjection && (binding.Executable == "" || len(binding.Targets) == 0) {
		r.Row("Projection", fmt.Sprintf("Deferred; %s is not installed", spec.Label))
		r.Next(fmt.Sprintf("Install %s, then run `aigw sync`", spec.Label))
		return r.Err()
	}
	r.Success(detail)
	if spec.RestartAfterProjection && changed {
		r.Row("Activation", "Restart required")
		r.Next(fmt.Sprintf("Restart %s, then run `aigw check`", spec.Label))
	} else {
		r.Next("aigw check")
	}
	return r.Err()
}

type selectionPreview struct {
	DryRun               bool                    `json:"dry_run"`
	Client               string                  `json:"client"`
	Route                string                  `json:"route"`
	Endpoint             string                  `json:"endpoint"`
	UpstreamEndpoint     string                  `json:"upstream_endpoint"`
	ConfigurationChanged bool                    `json:"configuration_changed"`
	Projections          []client.ProjectionPlan `json:"projections"`
}

func renderSelectionPreview(runtime invocation.Context, cfg configuration.Config, preview selectionPreview, jsonMode bool) error {
	if jsonMode {
		return presentation.WriteJSON(runtime.Out, preview)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Selection preview")
	r.Rows(
		presentation.Field{Label: "Client", Value: invocation.Title(preview.Client)},
		presentation.Field{Label: "Route", Value: cfg.RouteLabel(preview.Route)},
		presentation.Field{Label: "Endpoint", Value: preview.Endpoint},
		presentation.Field{Label: "Account upstream", Value: preview.UpstreamEndpoint},
	)
	if len(preview.Projections) > 0 {
		r.Section("Client projections")
		for _, plan := range preview.Projections {
			r.Row(plan.Target, plan.Action)
		}
	}
	r.Success("Preview did not read credentials, send requests or change files")
	return r.Err()
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
