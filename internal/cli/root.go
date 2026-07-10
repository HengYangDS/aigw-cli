package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "0.1.0-dev"

func NewRoot(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "aigw",
		Short:         "Manage team AI API profiles, secrets, and client routes",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd, app, false)
		},
	}
	root.SetOut(app.Out)
	root.SetErr(app.Err)
	root.Version = Version
	root.AddCommand(
		newSetupCommand(app),
		newAddCommand(app),
		newUseCommand(app),
		newRotateCommand(app),
		newStatusCommand(app),
		newTestCommand(app),
		newDoctorCommand(app),
		newSyncCommand(app),
		newProfileCommand(app),
		newRouteCommand(app),
		newAdapterCommand(app),
		newConfigCommand(app),
	)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w", err)
	})
	hiddenClaude := &cobra.Command{
		Use:    "__run-claude",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return RunClaude(app, args)
		},
	}
	hiddenClaude.DisableFlagParsing = true
	root.AddCommand(hiddenClaude)
	return root
}
