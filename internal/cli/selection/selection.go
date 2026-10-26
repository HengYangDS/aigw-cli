// Package selection owns explicit client Route selection.
package selection

import (
	"context"
	"fmt"
	"slices"
	"strings"

	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

// NewUseCommand constructs the daily client Route selection command.
func NewUseCommand(runtime invocation.Context) *cobra.Command {
	var client string
	var protocol string
	var forwardingEndpoint string
	var retries retryOptions
	var direct, dryRun, jsonMode bool
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
			forwarding, err := forwardingChoice(forwardingEndpoint, cmd.Flags().Changed("forwarding-endpoint"), direct)
			if err != nil {
				return err
			}
			connection, err := retries.connection(cmd, forwarding)
			if err != nil {
				return err
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			operation := runtime
			operation.Secrets = secrets.ObserveAvailability(runtime.Secrets)
			client, name, err := resolveUseSelection(operation, cfg, client, args)
			if err != nil {
				return err
			}
			selected, err := resolveUseRuntime(operation, cfg, client, name, configuration.EndpointProtocol(protocol), connection)
			if err != nil {
				return err
			}
			if dryRun {
				return previewUse(operation, cfg, selected, connection, jsonMode)
			}
			token, err := selectionToken(cmd.Context(), operation, cfg, selected)
			if err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(operation)
			configurationChanged, projectionChanged, binding, err := synchronizer.SelectRoute(cmd.Context(), cfg, client, name, selected.Protocol, connection, token)
			if err != nil {
				return err
			}
			updated := cfg.Clone()
			updated.Clients[client] = binding
			return renderUse(runtime, updated, selected, configurationChanged, projectionChanged, token != "", jsonMode)
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Client: "+configuration.AdmittedClientUsage())
	cmd.Flags().StringVar(&protocol, "protocol", "", "Endpoint protocol for a multi-protocol Route")
	cmd.Flags().StringVar(&forwardingEndpoint, "forwarding-endpoint", "", "Explicit client destination; the Account upstream remains unchanged")
	cmd.Flags().BoolVar(&direct, "direct", false, "Connect this client directly to its Account upstream")
	retries.bind(cmd)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the selection without reading credentials or writing files")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the selection preview or result as JSON")
	return cmd
}

func forwardingChoice(endpoint string, specified, direct bool) (*string, error) {
	if specified {
		if endpoint == "" {
			return nil, fmt.Errorf("--forwarding-endpoint requires an endpoint; use --direct to remove forwarding")
		}
		if direct {
			return nil, fmt.Errorf("--direct and --forwarding-endpoint are mutually exclusive")
		}
		return &endpoint, nil
	}
	if direct {
		return new(string), nil
	}
	return nil, nil
}

func renderUse(runtime invocation.Context, cfg configuration.Config, selected configuration.Runtime, changed, projectionChanged, tokenStored, jsonMode bool) error {
	title, detail := "Route already selected", "Selected client configuration synchronized; binding unchanged"
	switch {
	case changed && tokenStored:
		title, detail = "Route selected", "Account token stored; client configuration synchronized"
	case changed:
		title, detail = "Route selected", "Client configuration synchronized"
	case tokenStored:
		title, detail = "Token stored", "Account token stored; selected client configuration synchronized"
	}
	spec, _ := configuration.ClientSpecFor(selected.Client)
	result := useResult{Changed: changed, ProjectionChanged: projectionChanged, Runtime: selected, NextAction: "aigw check"}
	if action := clientactivation.ProjectionPrerequisites(cfg)[selected.Client]; action != "" {
		result.ProjectionDeferred = true
		result.NextAction = action
	} else if spec.RestartAfterProjection && projectionChanged {
		result.RestartRequired = true
		result.NextAction = fmt.Sprintf("Restart %s, then run `aigw check`", spec.Label)
	}
	if jsonMode {
		return presentation.WriteJSON(runtime.Out, result)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle(title)
	r.Section("Current selection")
	rows := []presentation.Field{{Label: "Route", Value: selected.RouteLabel}}
	if purpose := strings.TrimSpace(cfg.Routes[selected.RouteID].Purpose); purpose != "" {
		rows = append(rows, presentation.Field{Label: "Purpose", Value: purpose})
	}
	rows = append(rows,
		presentation.Field{Label: "Client", Value: spec.Label},
		presentation.Field{Label: "Protocol", Value: configuration.ProtocolLabel(selected.Protocol)},
		presentation.Field{Label: "Endpoint", Value: selected.Endpoint},
	)
	if selected.Endpoint != selected.UpstreamEndpoint {
		rows = append(rows, presentation.Field{Label: "Account upstream", Value: selected.UpstreamEndpoint})
	}
	if selected.Client == configuration.ClientCodex {
		rows = append(rows, retryFields(selected.RetryPolicy)...)
	}
	if result.ProjectionDeferred {
		if tokenStored {
			rows = append(rows, presentation.Field{Label: "Account Token", Value: "Validated and stored"})
		}
		rows = append(rows, presentation.Field{Label: "Projection", Value: "Deferred; native client projection is unavailable"})
		r.Rows(rows...)
		r.Next(result.NextAction)
		return r.Err()
	}
	if result.RestartRequired {
		rows = append(rows, presentation.Field{Label: "Activation", Value: "Restart required"})
	}
	if spec.SessionsKeepRoute && projectionChanged {
		rows = append(rows, existingSessionsField)
	}
	r.Rows(rows...)
	r.Success(detail)
	r.Next(result.NextAction)
	return r.Err()
}

// existingSessionsField tells users of a client whose sessions keep the route
// they were created with that a projection change reaches new sessions only.
var existingSessionsField = presentation.Field{Label: "Existing sessions", Value: "Keep the route they started with; new sessions use this selection"}

type useResult struct {
	DryRun             bool   `json:"dry_run"`
	Changed            bool   `json:"changed"`
	ProjectionChanged  bool   `json:"projection_changed"`
	ProjectionDeferred bool   `json:"projection_deferred,omitempty"`
	RestartRequired    bool   `json:"restart_required,omitempty"`
	NextAction         string `json:"next_action,omitempty"`
	configuration.Runtime
	Targets []client.ProjectionPlan `json:"targets,omitempty"`
}

func previewUse(runtime invocation.Context, cfg configuration.Config, selected configuration.Runtime, connection configuration.ConnectionChange, jsonMode bool) error {
	synchronizer := invocation.Synchronizer(runtime)
	synchronizer.AuthorizedSelectionClient = selected.Client
	proposed, _, err := synchronizer.PrepareSelection(cfg, selected.Client, selected.RouteID, selected.Protocol, connection)
	if err != nil {
		return err
	}
	binding := proposed.Clients[selected.Client]
	binding.Enabled = true
	proposed.Clients[selected.Client] = binding
	proposed, _, err = synchronizer.DesiredClientConfiguration(proposed, selected.Client)
	if err != nil {
		return err
	}
	plans, err := synchronizer.Plan(cfg, proposed, selected.Client)
	if err != nil {
		return err
	}
	result := useResult{DryRun: true, Runtime: selected, Targets: plans}
	if jsonMode {
		return presentation.WriteJSON(runtime.Out, result)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Selection preview")
	r.Section("Selection")
	rows := []presentation.Field{
		{Label: "Client", Value: configuration.ClientLabel(selected.Client)},
		{Label: "Route", Value: selected.RouteLabel},
		{Label: "Endpoint", Value: selected.Endpoint},
		{Label: "Account upstream", Value: selected.UpstreamEndpoint},
	}
	if selected.Client == configuration.ClientCodex {
		rows = append(rows, retryFields(selected.RetryPolicy)...)
	}
	if spec, _ := configuration.ClientSpecFor(selected.Client); spec.SessionsKeepRoute &&
		slices.ContainsFunc(plans, func(plan client.ProjectionPlan) bool { return plan.ChangesState }) {
		rows = append(rows, existingSessionsField)
	}
	r.Rows(rows...)
	if len(plans) > 0 {
		r.Section("Projection targets")
		targets := make([]presentation.Field, 0, len(plans))
		for _, plan := range plans {
			targets = append(targets, presentation.Field{Label: plan.Target, Value: plan.Action})
		}
		r.Rows(targets...)
	}
	r.Success("Credentials, configuration and client files were unchanged")
	return r.Err()
}

func resolveUseRuntime(runtime invocation.Context, cfg configuration.Config, client, routeID string, requested configuration.EndpointProtocol, connection configuration.ConnectionChange) (configuration.Runtime, error) {
	_, selected, err := invocation.Synchronizer(runtime).PrepareSelection(cfg, client, routeID, requested, connection)
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
		choice := strings.Join(names, "|")
		refusal := fmt.Errorf("Route %q has multiple compatible protocols for %s; pass --protocol <%s>", routeID, client, choice)
		return configuration.Runtime{}, invocation.Problem(runtime, refusal.Error(), "", "No Client Binding changed.",
			fmt.Sprintf("aigw use --for %s --protocol <%s> %s", client, choice, routeID), refusal)
	}
	choice, err := runtime.Prompt.Select("Select the endpoint protocol: ", choices)
	if err != nil {
		return configuration.Runtime{}, err
	}
	_, selected, err = invocation.Synchronizer(runtime).PrepareSelection(cfg, client, routeID, configuration.EndpointProtocol(choice), connection)
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
	compatible, err := cfg.ProtocolClientIDs(route)
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
	for _, id := range cfg.CatalogRouteIDs() {
		compatible, err := cfg.ProtocolClientIDs(id)
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
