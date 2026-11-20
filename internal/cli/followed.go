package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// reportingCommands save nothing and report the working configuration:
// status, which bare aigw runs too, and check. Their output names every
// choice of a client that the configuration follows; a command that saves or
// previews names the choices that its transition keeps.
var reportingCommands = []string{"", "status", "check"}

// reports reports whether command is a reporting command.
func reports(command *cobra.Command) bool {
	path := strings.TrimPrefix(strings.TrimPrefix(command.CommandPath(), command.Root().Name()), " ")
	return slices.Contains(reportingCommands, path)
}
