package cli

import (
	"aigw-cli/internal/cli/adapter"
	"aigw-cli/internal/cli/recovery"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/renaming"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandBoundaryConfigLoadFailures(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*App) *cobra.Command
		args []string
	}{
		{name: "adapter enable", cmd: func(app *App) *cobra.Command { return adapter.NewCommand(app.invocationContext()) }, args: []string{"enable", "claude", "--executable", "/claude"}},
		{name: "adapter auth", cmd: func(app *App) *cobra.Command { return adapter.NewCommand(app.invocationContext()) }, args: []string{"auth", "codex"}},
		{name: "adapter disable", cmd: func(app *App) *cobra.Command { return adapter.NewCommand(app.invocationContext()) }, args: []string{"disable", "codex"}},
		{name: "profile rename", cmd: func(app *App) *cobra.Command { return renaming.NewProfileCommand(app.renamingDependencies()) }, args: []string{"old", "new"}},
		{name: "sync", cmd: func(app *App) *cobra.Command { return recovery.NewSyncCommand(app.invocationContext()) }},
		{name: "rollback", cmd: func(app *App) *cobra.Command { return recovery.NewRollbackCommand(app.invocationContext()) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := configuredCommandApp(t, configuredCommandState())
			app.Config = configuration.NewStore(t.TempDir())
			cmd := test.cmd(app)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected config load error")
			}
		})
	}
}
