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
	CompleteFlag(command, "protocol", completeProtocols)
}

// TestProtocolsFlagKeepsEachNamedProtocolOnce parses a repeatable
// `--protocol` as `aigw model edit` does: the names give their protocols in
// the order given, each once, a preview rebuilds them, and any other spelling
// fails flag parsing.
func TestProtocolsFlagKeepsEachNamedProtocolOnce(t *testing.T) {
	var protocols []domain.EndpointProtocol
	command := &cobra.Command{Use: "demo"}
	BindProtocols(command, &protocols, "Protocols")
	flag := command.Flags().Lookup("protocol")
	if flag.Usage != "Protocols: anthropic, responses, or chat" || flag.Value.String() != "" || flag.Value.Type() != "protocols" {
		t.Fatalf("protocols flag = %q, value %q, type %q", flag.Usage, flag.Value.String(), flag.Value.Type())
	}
	if err := command.ParseFlags([]string{"--protocol", "chat", "--protocol", "anthropic", "--protocol", "chat"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(protocols, []domain.EndpointProtocol{domain.ProtocolOpenAIChatCompletions, domain.ProtocolAnthropic}) || flag.Value.String() != "chat,anthropic" {
		t.Fatalf("parsed protocols = %q, value %q", protocols, flag.Value.String())
	}
	if line, _ := commandLine(command, nil); line != "demo --protocol chat --protocol anthropic" {
		t.Fatalf("rebuilt command line = %q", line)
	}
	slice, ok := flag.Value.(pflag.SliceValue)
	if !ok {
		t.Fatal("the protocols flag is not a slice value")
	}
	if err := slice.Replace([]string{"responses"}); err != nil || !slices.Equal(protocols, []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}) {
		t.Fatalf("replaced protocols = %q, %v", protocols, err)
	}
	if err := slice.Replace([]string{"openai_responses"}); !errors.Is(err, errProtocolName) {
		t.Fatalf("replacing with an old spelling = %v", err)
	}
	if err := command.ParseFlags([]string{"--protocol", "messages"}); err == nil || !strings.Contains(err.Error(), "expected anthropic, responses, or chat") {
		t.Fatalf("an unknown protocol = %v", err)
	}
}
