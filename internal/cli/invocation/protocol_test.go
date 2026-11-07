package invocation

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestProtocolFlagAcceptsOnlyProtocolNames parses `--protocol` as a command
// does: each protocol name selects its protocol, and an old spelling or an
// unknown name fails flag parsing with the valid names. The flag's help lists
// the names, it rebuilds a command line with its name, and it completes the
// names alone.
func TestProtocolFlagAcceptsOnlyProtocolNames(t *testing.T) {
	newCommand := func(target *domain.EndpointProtocol) *cobra.Command {
		command := &cobra.Command{Use: "demo", RunE: func(*cobra.Command, []string) error { return nil }}
		BindProtocol(command, target, "Endpoint protocol")
		return command
	}
	for name, want := range map[string]domain.EndpointProtocol{
		"anthropic": domain.ProtocolAnthropic, "responses": domain.ProtocolOpenAIResponses, "chat": domain.ProtocolOpenAIChatCompletions,
	} {
		var protocol domain.EndpointProtocol
		command := newCommand(&protocol)
		if err := command.ParseFlags([]string{"--protocol", name}); err != nil || protocol != want {
			t.Fatalf("--protocol %s = %q, %v; want %q", name, protocol, err, want)
		}
		if line, _ := commandLine(command, nil); line != "demo --protocol "+name {
			t.Fatalf("rebuilt command line = %q", line)
		}
	}
	for _, refused := range []string{"openai_responses", "openai_chat_completions", "messages"} {
		var protocol domain.EndpointProtocol
		err := newCommand(&protocol).ParseFlags([]string{"--protocol", refused})
		if _, invalid := errors.AsType[*pflag.InvalidValueError](err); !invalid || protocol != "" {
			t.Fatalf("--protocol %s = %q, %v; want an invalid flag value", refused, protocol, err)
		}
		if want := `invalid argument "` + refused + `" for "--protocol" flag: expected anthropic, responses, or chat`; err.Error() != want {
			t.Fatalf("refusal = %q, want %q", err, want)
		}
	}
	var protocol domain.EndpointProtocol
	command := newCommand(&protocol)
	flag := command.Flags().Lookup("protocol")
	if flag.Usage != "Endpoint protocol: anthropic, responses, or chat" || flag.DefValue != "" || flag.Value.Type() != "string" {
		t.Fatalf("protocol flag = %q, default %q, type %q", flag.Usage, flag.DefValue, flag.Value.Type())
	}
	complete, registered := command.GetFlagCompletionFunc("protocol")
	if !registered {
		t.Fatal("the protocol flag has no completion")
	}
	if names, directive := complete(command, nil, ""); !slices.Equal(names, []string{"anthropic", "responses", "chat"}) || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("protocol completion = %q, %v", names, directive)
	}
	defer func() {
		recovered, isError := recover().(error)
		if !isError || !strings.Contains(recovered.Error(), "already registered") {
			t.Fatalf("a second completion of one flag = %v, want a panic", recovered)
		}
	}()
	completeFlag(command, "protocol", completeProtocols)
}
