package client

import (
	"strings"

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
		Short: "List every client with its model, whether it is enabled and where it is installed",
		Long: "List every client with its model as provider/model, whether it is enabled and where it is installed.\n" +
			"It reads the configuration and looks for each client's executable; it reads no Token and sends no request.",
		Example: "aigw client list\naigw client list --json",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			discovered, err := runtime.Discover()
			if err != nil {
				return err
			}
			document := listDocument{Clients: []clientItem{}}
			for _, spec := range runtime.Clients.Admission().Specs() {
				document.Clients = append(document.Clients, describe(cfg, discovered, spec.ID))
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

// describe reports one client as list does: the executable that synchronization
// keeps for it, whether it is enabled, and its model.
func describe(cfg domain.Config, discovered discovery.Result, id string) clientItem {
	binding := cfg.Clients[id]
	item := clientItem{ID: id, Executable: installedExecutable(binding.Executable, discovered.Executable(id)), Enabled: binding.Enabled}
	item.Installed = item.Executable != ""
	// A loaded configuration is valid, so a selection names a model.
	if model, selected := cfg.Models[cfg.SelectedModel(id)]; selected {
		item.Model = model.Ref().String()
	}
	return item
}

// installedExecutable is the client's executable as synchronization keeps it:
// the configured one while it is still there, otherwise the one that discovery
// finds, and nothing when neither exists.
func installedExecutable(configured, discovered string) string {
	if configured != "" {
		if available, err := discovery.ExecutableAvailable(configured); err == nil && available {
			return configured
		}
	}
	return discovered
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
		installed := item.Executable
		if !item.Installed {
			installed = "Not installed"
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
