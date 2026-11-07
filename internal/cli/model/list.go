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
			for _, key := range cfg.CatalogModelIDs() {
				if item := describe(cfg, key); filters.admit(cfg.Models[key], item, args) {
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
func (filters listFilters) admit(model domain.Model, item modelItem, words []string) bool {
	if len(filters.providers) > 0 && !slices.Contains(filters.providers, model.Provider) {
		return false
	}
	if len(filters.clients) > 0 && !slices.ContainsFunc(filters.clients, func(client string) bool { return slices.Contains(item.ProtocolClients, client) }) {
		return false
	}
	if _, declared := model.Protocols[filters.protocol]; filters.protocol != "" && !declared {
		return false
	}
	text := strings.ToLower(model.UpstreamModelID() + "\x00" + item.Label)
	for _, word := range words {
		for part := range strings.FieldsSeq(strings.ToLower(word)) {
			if !strings.Contains(text, part) {
				return false
			}
		}
	}
	return true
}

func renderList(runtime invocation.Context, cfg domain.Config, document listDocument) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Models")
	if len(document.Models) == 0 {
		r.Text("No added model matches.")
		r.Next("aigw model add <provider/model>")
		return r.Err()
	}
	provider := ""
	for _, item := range document.Models {
		if item.Provider != provider {
			// A valid configuration gives every provider a label.
			provider = item.Provider
			r.Section(cfg.Providers[provider].Label)
		}
		state, detail := terminal.Info, []string{item.Label}
		protocols := make([]string, 0, len(item.Protocols))
		for _, protocol := range item.Protocols {
			protocols = append(protocols, protocolLabel(protocol.Protocol))
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
		r.Rows(terminal.Field{State: state, Label: item.Model, Value: strings.Join(detail, " · ")})
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
		ValidArgsFunction: invocation.CompleteModels(runtime, noClient),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, key, model, err := loadModel(runtime, args[0])
			if err != nil {
				return err
			}
			item := describe(cfg, key)
			provider := cfg.Providers[model.Provider]
			for _, protocol := range model.AdmittedProtocols() {
				item.Endpoints = append(item.Endpoints, endpointDetail{Protocol: protocol.Name(), URL: provider.Endpoints.For(protocol)})
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
				fields = append(fields, terminal.Field{Label: protocolLabel(endpoint.Protocol), Value: endpoint.URL})
			}
			r.Rows(fields...)
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the model as JSON")
	return cmd
}

// noClient completes every added model, for a command that names no client.
func noClient() string { return "" }
