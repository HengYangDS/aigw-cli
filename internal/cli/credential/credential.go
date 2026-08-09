// Package credential exposes the narrow token-helper surface consumed by
// admitted native clients. It never prints diagnostics to standard output.
package credential

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"github.com/spf13/cobra"
)

// NewCommand builds the private credential-helper command used by Claude Code.
func NewCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use:    "credential claude",
		Short:  "Read the active Claude gateway credential",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != configuration.ClientClaude {
				return fmt.Errorf("credential helper is available only for Claude")
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			adapter := cfg.Adapters[configuration.ClientClaude]
			if !adapter.Enabled {
				return fmt.Errorf("Claude adapter is not enabled")
			}
			clientRuntime, _, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
			if err != nil {
				return err
			}
			token, err := runtime.Secrets.Get(clientRuntime.AccountID)
			if err != nil {
				return fmt.Errorf("Claude gateway credential is unavailable: %w", err)
			}
			_, err = fmt.Fprintln(runtime.Out, token)
			return err
		},
	}
}
