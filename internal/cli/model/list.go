package model

import (
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// listFilters narrow `aigw model list`: every query word, ignoring case, must
// appear in a model's ID or label, and repeated --provider and --client values
// each admit a model that any of them admits.
type listFilters struct {
	providers []string
	clients   []string
	protocol  domain.EndpointProtocol
}

func newListCommand(runtime invocation.Context) *cobra.Command {
	var filters listFilters
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "list [query]",
		Short: "List the models you have added; reads no Token and sends no request",
		Example: "aigw model list\n" +
			"aigw model list sonnet --provider dmxapi\n" +
			"aigw model list gpt --protocol responses --json",
		Args: func(cmd *cobra.Command, _ []string) error {
			for _, client := range filters.clients {
				if _, err := invocation.ResolveClient(runtime.Clients.Admission(), client); err != nil {
					return err
				}
			}
			return nil
		},
		ValidArgsFunction: invocation.CompleteNothing,
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			for _, provider := range filters.providers {
				if _, err := invocation.ResolveProvider(cfg, provider); err != nil {
					return err
				}
			}
			document := listDocument{Models: []modelItem{}}
			for _, ref := range cfg.ModelRefs() {
				if item := describe(cfg, ref); filters.admit(item, args) {
					document.Models = append(document.Models, item)
				}
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, listSchemaVersion, true, document)
			}
			return renderList(runtime, cfg, document)
		},
	}
	cmd.Flags().StringSliceVar(&filters.providers, "provider", nil, "Only this provider's models; repeatable")
	cmd.Flags().StringSliceVar(&filters.clients, "client", nil, "Only the models this client can use; repeatable: "+runtime.Clients.Admission().Usage())
	invocation.BindProtocol(cmd, &filters.protocol, "Only the models that declare this protocol")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the models as JSON")
	invocation.CompleteFlag(cmd, "provider", invocation.CompleteProviders(runtime))
	invocation.CompleteFlag(cmd, "client", invocation.CompleteClients(runtime))
	return cmd
}

// admit reports whether a model passes every filter and every query word.
func (filters listFilters) admit(item modelItem, words []string) bool {
	if len(filters.providers) > 0 && !slices.Contains(filters.providers, item.Provider) {
		return false
	}
	if len(filters.clients) > 0 && !slices.ContainsFunc(filters.clients, func(client string) bool { return slices.Contains(item.ProtocolClients, client) }) {
		return false
	}
	if filters.protocol != "" && !slices.ContainsFunc(item.Protocols, func(protocol protocolItem) bool { return protocol.Protocol == filters.protocol.Name() }) {
		return false
	}
	_, id, _ := strings.Cut(item.ID, "/")
	return matches(words, id, item.Label)
}

func renderList(runtime invocation.Context, cfg domain.Config, document listDocument) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Models")
	if len(document.Models) == 0 {
		r.Text("No added model matches.")
		r.Next("aigw model add <provider/model>")
		return r.Err()
	}
	provider, fix := "", ""
	for _, item := range document.Models {
		if item.Provider != provider {
			// A valid configuration gives every provider a label.
			provider = item.Provider
			r.Section(cfg.Providers[provider].Label)
		}
		state, detail := terminal.Info, []string{item.Label}
		protocols := make([]string, 0, len(item.Protocols))
		for _, protocol := range item.Protocols {
			label := protocolLabel(protocol.Protocol)
			if protocol.NoEndpoint {
				label += " (no endpoint)"
			}
			protocols = append(protocols, label)
		}
		detail = append(detail, strings.Join(protocols, ", "))
		if len(item.SelectedBy) > 0 {
			state = terminal.OK
			detail = append(detail, "used by "+clientLabels(runtime.Clients.Admission(), item.SelectedBy))
		}
		if item.Deprecated {
			state = terminal.Warn
			detail = append(detail, "deprecated")
		}
		r.Rows(terminal.Field{State: state, Label: item.ID, Value: strings.Join(detail, " · ")})
		if fix == "" {
			fix = missingEndpointFix(item)
		}
	}
	if fix != "" {
		r.Next(fix)
		return r.Err()
	}
	r.Next("aigw use <client> <provider/model>")
	return r.Err()
}

func newShowCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:               "show <provider/model>",
		Short:             "Show one model: its protocols, capabilities and the clients that use it",
		Example:           "aigw model show dmxapi/gpt-6.1-sol\naigw model show ucloud/minimax-m3 --json",
		Args:              modelArgument,
		ValidArgsFunction: invocation.CompleteModels(runtime),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, model, err := loadModel(runtime, args[0], "")
			if err != nil {
				return err
			}
			item := describe(cfg, model.Ref())
			provider := cfg.Providers[model.Provider]
			for _, protocol := range displayProtocols(model) {
				endpoint := endpointDetail{Protocol: protocol.Name(), URL: provider.Endpoints.For(protocol)}
				if endpoint.URL == "" {
					endpoint.Fix = endpointFix(model.Provider, protocol.Name())
				}
				item.Endpoints = append(item.Endpoints, endpoint)
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, showSchemaVersion, true, item)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Model")
			fields := modelFields(runtime, item)
			fields = append(fields, terminal.Field{Label: "Provider", Value: provider.Label})
			if len(item.ProtocolClients) > 0 {
				fields = append(fields, terminal.Field{Label: "Protocol clients", Value: clientLabels(runtime.Clients.Admission(), item.ProtocolClients)})
			}
			for _, endpoint := range item.Endpoints {
				value := endpoint.URL
				if endpoint.Fix != "" {
					value = "No endpoint; run `" + endpoint.Fix + "`"
				}
				fields = append(fields, terminal.Field{Label: protocolLabel(endpoint.Protocol), Value: value})
			}
			r.Rows(fields...)
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the model as JSON")
	return cmd
}

// endpointFix is the command that gives provider an endpoint for protocol.
func endpointFix(provider, protocol string) string {
	return "aigw provider edit " + provider + " --endpoint " + protocol + "=<url>"
}

// missingEndpointFix is the fix of the first protocol of item that its
// provider has no endpoint for, or empty.
func missingEndpointFix(item modelItem) string {
	for _, protocol := range item.Protocols {
		if protocol.NoEndpoint {
			return endpointFix(item.Provider, protocol.Protocol)
		}
	}
	return ""
}
