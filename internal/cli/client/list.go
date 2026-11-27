package client

import (
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

func newListCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every client with its model, whether it is enabled and its program",
		Long: "List every client with its model as provider/model, whether it is enabled and its program, with the extensions directory that it declares.\n" +
			"It reads the configuration and looks for each client's program; it runs none, reads no Token and sends no request.",
		Example: "aigw client list\naigw client list --json",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := runtime.LoadConfiguration()
			if err != nil {
				return err
			}
			discovered, err := runtime.Discover()
			if err != nil {
				return err
			}
			document := listDocument{Clients: []clientItem{}}
			for _, spec := range runtime.Clients.Admission().Specs() {
				document.Clients = append(document.Clients, describe(cfg, discovered, runtime.Host, spec.ID))
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, listSchemaVersion, true, document)
			}
			return renderList(runtime, document)
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the clients as JSON")
	return cmd
}

// describe reports one client as list does: the program that the client uses
// under the one rule of adapter.ClientProgram, whether it is there, the
// extensions directory that the client declares, whether it is enabled, and
// its model. A declared program that is gone is reported as it is declared,
// never replaced by a discovered one.
func describe(cfg domain.Config, discovered discovery.Result, host discovery.Host, id string) clientItem {
	binding := cfg.Clients[id]
	program, stop := adapter.ClientProgram(cfg, id, discovered, host)
	item := clientItem{ID: id, Program: program.Path, Extensions: binding.Extensions, Enabled: binding.Enabled}
	item.Installed = program.Path != "" && stop == nil
	// A loaded configuration is valid, so a selection names a model.
	if model, selected := cfg.Models[cfg.SelectedModel(id)]; selected {
		item.Model = model.Ref().String()
	}
	return item
}

func renderList(runtime invocation.Context, document listDocument) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Clients")
	rows := make([]terminal.Field, 0, len(document.Clients))
	for _, item := range document.Clients {
		state, enabled := terminal.Info, "Disabled"
		if item.Enabled {
			state, enabled = terminal.OK, "Enabled"
		}
		model := item.Model
		if model == "" {
			model = "No model selected"
		}
		installed := item.Program
		if item.Extensions != "" {
			installed += " (the newest version in " + item.Extensions + ")"
		}
		if !item.Installed {
			switch {
			case item.Program != "":
				installed = "Missing " + item.Program
			case item.Extensions != "":
				installed = "No program in " + item.Extensions
			default:
				installed = "Not installed"
			}
			if item.Enabled {
				state = terminal.Warn
			}
		}
		rows = append(rows, terminal.Field{State: state, Label: runtime.Clients.Admission().Label(item.ID), Value: strings.Join([]string{model, enabled, installed}, " · ")})
	}
	r.Rows(rows...)
	r.Next("aigw use <client> <provider/model>")
	return r.Err()
}
