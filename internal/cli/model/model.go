// Package model implements `aigw model`, which lists, shows, adds, edits and
// removes the models that the configuration holds, each named
// `provider/model`.
package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs the model command group.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "model", Short: "List, show, add, edit and remove models"}
	root.AddCommand(newListCommand(runtime), newShowCommand(runtime), newAddCommand(runtime), newEditCommand(runtime), newRemoveCommand(runtime))
	return root
}

// modelArgument checks a command's one `provider/model` argument before any
// configuration access. An argument without a slash is left to the command,
// which lists the models that it could mean.
func modelArgument(cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("%s needs a <provider/model> argument; run `%s --help`", cmd.CommandPath(), cmd.CommandPath())
	case 1:
	default:
		return fmt.Errorf("%s takes one <provider/model> argument, not %d; run `%s --help`", cmd.CommandPath(), len(args), cmd.CommandPath())
	}
	if _, err := domain.ParseModelRef(args[0]); err != nil && !errors.Is(err, domain.ErrModelRefWithoutProvider) {
		return fmt.Errorf("%w; run `%s --help`", err, cmd.CommandPath())
	}
	return nil
}

// loadModel loads the configuration and resolves the command's model
// argument to the configured model and its key. The protocol that cmd's
// --protocol filter names selects among the models of one name; a command
// without that filter names the model's Route when the name is ambiguous.
func loadModel(cmd *cobra.Command, runtime invocation.Context, protocol domain.EndpointProtocol, text string) (domain.Config, string, domain.Model, error) {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return domain.Config{}, "", domain.Model{}, err
	}
	ref, err := invocation.ParseModelArgument(cfg, text)
	if err != nil {
		return domain.Config{}, "", domain.Model{}, err
	}
	var rerun func(string) string
	if filter := cmd.Flags().Lookup("protocol"); filter != nil && filter.Value.Type() == "string" {
		rerun = func(choice string) string {
			return invocation.Rerun(cmd, cmd.Flags().Args(), []string{"protocol"}, "--protocol", choice)
		}
	}
	key, model, err := invocation.ResolveModel(cfg, ref, protocol, rerun)
	return cfg, key, model, err
}

// describe reports one added model as list and show do: its name, label,
// family, protocols with their capabilities, the clients that can use it and
// the clients that select it. Several keys are the models of one name, one per
// protocol, which it reports as one model with their protocols merged; the
// first supplies the label and family.
func describe(cfg domain.Config, keys ...string) modelItem {
	first := cfg.Models[keys[0]]
	item := modelItem{
		ID: first.Ref().String(), Provider: first.Provider, Label: cfg.ModelLabel(keys[0]), Family: first.Family,
		Protocols: []protocolItem{}, ProtocolClients: []string{}, SelectedBy: []string{}, Duplicate: len(keys) > 1,
	}
	protocols := map[domain.EndpointProtocol][]domain.Capability{}
	for _, key := range keys {
		model := cfg.Models[key]
		for protocol, capabilities := range model.Protocols {
			protocols[protocol] = append(protocols[protocol], capabilities...)
		}
		// A loaded configuration is valid, so every model has its provider.
		clients, _ := cfg.ProtocolClientIDs(key)
		item.ProtocolClients = append(item.ProtocolClients, clients...)
		item.SelectedBy = append(item.SelectedBy, cfg.SelectedClientsForModel(key)...)
		item.Deprecated = item.Deprecated || model.LifecycleState() == domain.ModelDeprecated
	}
	item.ProtocolClients = inAdmissionOrder(cfg.Admission(), item.ProtocolClients)
	item.SelectedBy = inAdmissionOrder(cfg.Admission(), item.SelectedBy)
	for _, protocol := range displayProtocols(domain.Model{Protocols: protocols}) {
		capabilities := []string{}
		for _, capability := range domain.Capabilities() {
			if slices.Contains(protocols[protocol], capability) {
				capabilities = append(capabilities, string(capability))
			}
		}
		item.Protocols = append(item.Protocols, protocolItem{Protocol: protocol.Name(), Capabilities: capabilities})
	}
	return item
}

// inAdmissionOrder returns the clients among ids, each once, in admission
// order.
func inAdmissionOrder(admission domain.Admission, ids []string) []string {
	ordered := []string{}
	for _, id := range admission.IDs() {
		if slices.Contains(ids, id) {
			ordered = append(ordered, id)
		}
	}
	return ordered
}

// displayProtocols returns a model's protocols in display order: anthropic,
// responses, chat.
func displayProtocols(model domain.Model) []domain.EndpointProtocol {
	return slices.SortedFunc(slices.Values(model.AdmittedProtocols()), domain.CompareProtocols)
}

// protocolLabels names protocols by their labels, in a list for prose.
func protocolLabels(protocols []domain.EndpointProtocol) string {
	names := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		names = append(names, domain.ProtocolLabel(protocol))
	}
	switch len(names) {
	case 0:
		return "no protocol"
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
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

// modelFields are the rows of one model in human output.
func modelFields(runtime invocation.Context, item modelItem) []terminal.Field {
	protocols := make([]string, 0, len(item.Protocols))
	for _, protocol := range item.Protocols {
		text := protocolLabel(protocol.Protocol)
		if len(protocol.Capabilities) > 0 {
			text += " (" + strings.Join(protocol.Capabilities, ", ") + ")"
		}
		protocols = append(protocols, text)
	}
	fields := []terminal.Field{
		{Label: "Model", Value: item.ID},
		{Label: "Label", Value: item.Label},
		{Label: "Family", Value: item.Family},
		{Label: "Protocols", Value: strings.Join(protocols, ", ")},
	}
	if len(item.SelectedBy) > 0 {
		fields = append(fields, terminal.Field{Label: "Selected by", Value: clientLabels(runtime.Clients.Admission(), item.SelectedBy)})
	}
	if item.Deprecated {
		fields = append(fields, terminal.Field{State: terminal.Warn, Label: "Deprecated", Value: "Kept for the clients that select it"})
	}
	return fields
}
