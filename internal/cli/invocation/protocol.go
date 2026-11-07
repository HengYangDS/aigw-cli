package invocation

import (
	"errors"

	"aigw-cli/internal/domain"

	"github.com/spf13/cobra"
)

// errProtocolName refuses a protocol spelling that the command line does not
// name. pflag quotes the refused value and its flag before it.
var errProtocolName = errors.New("expected " + domain.ProtocolUsage())

// protocolFlag is the value of a `--protocol` flag: one protocol as the
// command line names it. Any other spelling is refused while the flags are
// parsed, so the refusal names the command's help as every flag error does.
type protocolFlag struct{ protocol *domain.EndpointProtocol }

func (flag protocolFlag) Set(name string) error {
	protocol, named := domain.ProtocolNamed(name)
	if !named {
		return errProtocolName
	}
	*flag.protocol = protocol
	return nil
}

// String is the protocol's name, which a command line that rebuilds the flag
// writes, or nothing while the flag is unset.
func (flag protocolFlag) String() string {
	if *flag.protocol == "" {
		return ""
	}
	return flag.protocol.Name()
}

// Type names the value as help names any other text value.
func (protocolFlag) Type() string { return "string" }

// BindProtocol adds a `--protocol` flag to cmd that stores the protocol it
// names in target. Its help lists the protocol names after usage, and it
// completes them.
func BindProtocol(cmd *cobra.Command, target *domain.EndpointProtocol, usage string) {
	cmd.Flags().Var(protocolFlag{protocol: target}, "protocol", usage+": "+domain.ProtocolUsage())
	completeFlag(cmd, "protocol", completeProtocols)
}

// completeProtocols completes a protocol from the fixed list of names.
func completeProtocols(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return domain.ProtocolNames(), cobra.ShellCompDirectiveNoFileComp
}

// completeFlag registers the completion of a flag that cmd defines.
// Registration fails only for a flag that cmd lacks or that already has a
// completion, which is a defect of the command tree that any test of it shows.
func completeFlag(cmd *cobra.Command, name string, complete cobra.CompletionFunc) {
	if err := cmd.RegisterFlagCompletionFunc(name, complete); err != nil {
		panic(err)
	}
}
