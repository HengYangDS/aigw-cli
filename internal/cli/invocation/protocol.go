package invocation

import (
	"errors"
	"slices"
	"strings"

	"aigw-cli/internal/domain"

	"github.com/spf13/cobra"
)

// errProtocolName refuses a protocol spelling that the command line does not
// name. pflag quotes the refused value and its flag before it.
var errProtocolName = errors.New("expected " + domain.ProtocolUsage())

// errProtocolRepeated refuses a second protocol for a flag that takes one.
var errProtocolRepeated = errors.New("this flag takes one protocol")

// protocolFlag is the value of a `--protocol` flag: one protocol as the
// command line names it. Any other spelling, and a second protocol, is
// refused while the flags are parsed, so the refusal names the command's help
// as every flag error does.
type protocolFlag struct{ protocol *domain.EndpointProtocol }

func (flag protocolFlag) Set(name string) error {
	protocol, named := domain.ProtocolNamed(name)
	if !named {
		return errProtocolName
	}
	if *flag.protocol != "" {
		return errProtocolRepeated
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
	CompleteFlag(cmd, "protocol", completeProtocols)
}

// protocolsFlag is the value of a repeatable `--protocol` flag: the protocols
// that it names, in the order given, each once. A preview's command line
// repeats each of them.
type protocolsFlag struct{ protocols *[]domain.EndpointProtocol }

func (flag protocolsFlag) Set(name string) error { return flag.Append(name) }

// Append adds the protocol that name names.
func (flag protocolsFlag) Append(name string) error {
	protocol, named := domain.ProtocolNamed(name)
	if !named {
		return errProtocolName
	}
	if !slices.Contains(*flag.protocols, protocol) {
		*flag.protocols = append(*flag.protocols, protocol)
	}
	return nil
}

// Replace makes the protocols those that names name.
func (flag protocolsFlag) Replace(names []string) error {
	*flag.protocols = nil
	for _, name := range names {
		if err := flag.Append(name); err != nil {
			return err
		}
	}
	return nil
}

// GetSlice returns the names of the protocols.
func (flag protocolsFlag) GetSlice() []string {
	names := make([]string, 0, len(*flag.protocols))
	for _, protocol := range *flag.protocols {
		names = append(names, protocol.Name())
	}
	return names
}

// String lists the protocols' names, or nothing while the flag is unset.
func (flag protocolsFlag) String() string { return strings.Join(flag.GetSlice(), ",") }

func (protocolsFlag) Type() string { return "protocols" }

// BindProtocols adds a repeatable `--protocol` flag to cmd that stores the
// protocols it names in target. Its help lists the protocol names after
// usage, and it completes them.
func BindProtocols(cmd *cobra.Command, target *[]domain.EndpointProtocol, usage string) {
	cmd.Flags().Var(protocolsFlag{protocols: target}, "protocol", usage+": "+domain.ProtocolUsage())
	CompleteFlag(cmd, "protocol", completeProtocols)
}

// completeProtocols completes a protocol from the fixed list of names.
func completeProtocols(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return domain.ProtocolNames(), cobra.ShellCompDirectiveNoFileComp
}

// CompleteFlag registers the completion of cmd's flag name. Registration fails
// only for a flag that cmd lacks or that already has a completion, which is a
// defect of the command tree that any test of it shows.
func CompleteFlag(cmd *cobra.Command, name string, complete cobra.CompletionFunc) {
	if err := cmd.RegisterFlagCompletionFunc(name, complete); err != nil {
		panic(err)
	}
}
