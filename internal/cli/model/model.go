// Package model implements `aigw model`, which lists, shows, adds, edits and
// removes the models that the configuration holds, each named
// `provider/model`, and searches the models that providers' catalogs list.
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
	root := &cobra.Command{Use: "model", Short: "List, search, show, add, edit and remove models"}
	root.AddCommand(newListCommand(runtime), newSearchCommand(runtime), newShowCommand(runtime), newAddCommand(runtime), newEditCommand(runtime), newRemoveCommand(runtime))
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
// argument to the configured model. A given protocol must be one that the
// model declares.
func loadModel(runtime invocation.Context, text string, protocol domain.EndpointProtocol) (domain.Config, domain.Model, error) {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return domain.Config{}, domain.Model{}, err
	}
	ref, err := invocation.ParseModelArgument(cfg, text)
	if err != nil {
		return domain.Config{}, domain.Model{}, err
	}
	model, err := invocation.ResolveModel(cfg, ref, protocol)
	return cfg, model, err
}

// describe reports one added model as list and show do: its name, label,
// family, protocols with their capabilities, the clients that can use it and
// the clients that select it.
func describe(cfg domain.Config, ref domain.ModelRef) modelItem {
	model := cfg.Models[ref]
	// A loaded configuration is valid, so every model has its provider.
	clients, _ := cfg.ProtocolClientIDs(ref)
	item := modelItem{
		ID: ref.String(), Provider: model.Provider, Label: cfg.ModelLabel(ref), Family: model.FamilyID(),
		Protocols: []protocolItem{}, ProtocolClients: append([]string{}, clients...),
		SelectedBy: append([]string{}, cfg.SelectedClientsForModel(ref)...), Deprecated: model.Deprecated,
	}
	for _, protocol := range displayProtocols(model) {
		capabilities := []string{}
		for _, capability := range domain.Capabilities() {
			if slices.Contains(model.Protocols[protocol], capability) {
				capabilities = append(capabilities, string(capability))
			}
		}
		item.Protocols = append(item.Protocols, protocolItem{Protocol: protocol.Name(), Capabilities: capabilities})
	}
	return item
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
