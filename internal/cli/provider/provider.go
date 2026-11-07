// Package provider implements `aigw provider`, which lists, shows, adds, edits
// and removes providers, each a service with an ID, a label, one endpoint per
// protocol and a Token, rotates a provider's Token and reports provider
// balances.
package provider

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs the provider command group.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "provider", Short: "List, show, add, edit and remove providers; rotate Tokens; report balances"}
	root.AddCommand(
		newListCommand(runtime), newShowCommand(runtime), newAddCommand(runtime), newEditCommand(runtime),
		newRotateCommand(runtime), newBalanceCommand(runtime), newRemoveCommand(runtime),
	)
	return root
}

// providerArgument checks that a command has its one <provider> argument. The
// configuration resolves it, refusing an unknown one with the closest IDs.
func providerArgument(cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("%s needs a <provider> argument; run `%s --help`", cmd.CommandPath(), cmd.CommandPath())
	case 1:
		return nil
	default:
		return fmt.Errorf("%s takes one <provider> argument, not %d; run `%s --help`", cmd.CommandPath(), len(args), cmd.CommandPath())
	}
}

// completeProvider completes the one <provider> argument of a command from the
// configured providers, and nothing once it is given.
func completeProvider(runtime invocation.Context) cobra.CompletionFunc {
	providers := invocation.CompleteProviders(runtime)
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return providers(cmd, args, toComplete)
	}
}

// loadProvider loads the configuration and resolves the provider that id
// names.
func loadProvider(runtime invocation.Context, id string) (domain.Config, domain.Provider, error) {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return domain.Config{}, domain.Provider{}, err
	}
	provider, err := invocation.ResolveProvider(cfg, id)
	return cfg, provider, err
}

// describe reports one provider as list and show do: its ID, label, endpoints
// and balance probe, and its models' names, each once, in configuration order.
func describe(cfg domain.Config, id string) providerItem {
	provider := cfg.Providers[id]
	item := providerItem{ID: id, Label: provider.Label, Endpoints: endpointItems(provider.Endpoints), Models: providerModels(cfg, id)}
	if provider.BalanceProbe != nil {
		item.BalanceProbe = provider.BalanceProbe.Kind
	}
	return item
}

// providerModels returns the names of the provider's models, each once, in
// configuration order.
func providerModels(cfg domain.Config, id string) []string {
	names := []string{}
	for _, key := range cfg.CatalogModelIDs() {
		if model := cfg.Models[key]; model.Provider == id && !slices.Contains(names, model.Ref().String()) {
			names = append(names, model.Ref().String())
		}
	}
	return names
}

// selectingClients returns the clients, enabled or disabled, that select one of
// the provider's models, in admission order.
func selectingClients(cfg domain.Config, id string) []string {
	clients := []string{}
	for _, client := range cfg.Admission().IDs() {
		if model, selected := cfg.Models[cfg.SelectedModel(client)]; selected && model.Provider == id {
			clients = append(clients, client)
		}
	}
	return clients
}

// endpointItems lists endpoints in display order: anthropic, responses, chat.
func endpointItems(endpoints domain.Endpoints) []endpointItem {
	items := []endpointItem{}
	for _, name := range domain.ProtocolNames() {
		protocol, _ := domain.ProtocolNamed(name)
		if url := endpoints.For(protocol); url != "" {
			items = append(items, endpointItem{Protocol: name, URL: url})
		}
	}
	return items
}

// endpointFields are the rows that name endpoints by their protocols' labels.
func endpointFields(items []endpointItem) []terminal.Field {
	fields := make([]terminal.Field, 0, len(items))
	for _, item := range items {
		fields = append(fields, terminal.Field{Label: protocolLabel(item.Protocol), Value: item.URL})
	}
	return fields
}

// protocolLabel names the protocol with the command-line name name by its
// label, as human output does.
func protocolLabel(name string) string {
	protocol, _ := domain.ProtocolNamed(name)
	return domain.ProtocolLabel(protocol)
}

// clientLabels names clients in human output by their labels.
func clientLabels(admission domain.Admission, clients []string) string {
	labels := make([]string, 0, len(clients))
	for _, client := range clients {
		labels = append(labels, admission.Label(client))
	}
	return strings.Join(labels, ", ")
}

// endpoint is one --endpoint value: a protocol and the URL of its endpoint.
type endpoint struct {
	protocol domain.EndpointProtocol
	url      string
}

// bindEndpoints adds the repeatable --endpoint flag to cmd. Its values keep
// their order, and a comma in a URL stays part of it.
func bindEndpoints(cmd *cobra.Command, target *[]string, usage string) {
	cmd.Flags().StringArrayVar(target, "endpoint", nil, usage+" as `<protocol>=<url>`; repeatable: "+domain.ProtocolUsage())
	invocation.CompleteFlag(cmd, "endpoint", completeEndpoint)
}

// completeEndpoint completes the protocol of an --endpoint value, after which
// the user types the URL.
func completeEndpoint(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions := []cobra.Completion{}
	for _, name := range domain.ProtocolNames() {
		completions = append(completions, name+"=")
	}
	return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// parseEndpoints checks each --endpoint value before any configuration access:
// a protocol name, `=` and a URL that a provider endpoint may use, each
// protocol once. A refusal never repeats the value, which may hold a
// credential that the URL check refuses.
func parseEndpoints(cmd *cobra.Command, values []string) ([]endpoint, error) {
	endpoints := make([]endpoint, 0, len(values))
	for _, value := range values {
		name, url, found := strings.Cut(value, "=")
		protocol, named := domain.ProtocolNamed(name)
		switch {
		case !found:
			return nil, fmt.Errorf("--endpoint takes <protocol>=<url>; run `%s --help`", cmd.CommandPath())
		case !named:
			return nil, fmt.Errorf("--endpoint needs a protocol before =: %s; run `%s --help`", domain.ProtocolUsage(), cmd.CommandPath())
		case slices.ContainsFunc(endpoints, func(given endpoint) bool { return given.protocol == protocol }):
			return nil, fmt.Errorf("--endpoint names %s twice; run `%s --help`", name, cmd.CommandPath())
		}
		if err := domain.ValidateEndpoint(url); err != nil {
			return nil, fmt.Errorf("--endpoint %s: %w; run `%s --help`", name, err, cmd.CommandPath())
		}
		endpoints = append(endpoints, endpoint{protocol: protocol, url: strings.TrimRight(url, "/")})
	}
	return endpoints, nil
}

// setEndpoint sets the endpoint for one protocol.
func setEndpoint(endpoints *domain.Endpoints, protocol domain.EndpointProtocol, url string) {
	switch protocol {
	case domain.ProtocolAnthropic:
		endpoints.Anthropic = url
	case domain.ProtocolOpenAIResponses:
		endpoints.OpenAIResponses = url
	case domain.ProtocolOpenAIChatCompletions:
		endpoints.OpenAIChatCompletions = url
	}
}
