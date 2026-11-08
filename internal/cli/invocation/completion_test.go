package invocation

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"

	"github.com/spf13/cobra"
)

// forbiddenStore fails the test on any credential access: completion reads
// no Token and no credential metadata.
type forbiddenStore struct{ t *testing.T }

func (s forbiddenStore) Get(string) (string, error) {
	s.t.Fatal("completion read a Token")
	return "", nil
}

func (s forbiddenStore) Set(string, string) error {
	s.t.Fatal("completion stored a Token")
	return nil
}

func (s forbiddenStore) Delete(string) error {
	s.t.Fatal("completion deleted a Token")
	return nil
}

func (s forbiddenStore) Exists(string) (bool, error) {
	s.t.Fatal("completion read credential metadata")
	return false, nil
}

// forbiddenHTTP fails the test on any request: completion sends none.
type forbiddenHTTP struct{ t *testing.T }

func (c forbiddenHTTP) Do(*http.Request) (*http.Response, error) {
	c.t.Fatal("completion sent a request")
	return nil, nil
}

// completionRuntime holds a configuration with a Messages-only provider, a
// Responses-only provider and a provider of both, whose credential store and
// HTTP client fail on any use.
func completionRuntime(t *testing.T) Context {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["messages"] = domain.Provider{Label: "Messages", Endpoints: domain.Endpoints{Anthropic: "https://messages.test"}}
	cfg.Providers["responses"] = domain.Provider{Label: "Responses", Endpoints: domain.Endpoints{OpenAIResponses: "https://responses.test/v1"}}
	cfg.Families["opus"] = domain.Family{Label: "Opus"}
	cfg.Models["messages-opus"] = domain.Model{Provider: "messages", Family: "opus", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["responses-sol"] = domain.Model{Provider: "responses", Family: "sol", UpstreamModel: "openai/gpt-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	// The model declares only anthropic, and its provider also serves
	// responses, which `aigw use` gives it for Codex.
	cfg.Providers["both"] = domain.Provider{Label: "Both", Endpoints: domain.Endpoints{Anthropic: "https://both.test", OpenAIResponses: "https://both.test/v1"}}
	cfg.Models["both-opus"] = domain.Model{Provider: "both", Family: "opus", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	// A second Route of the same name, on responses, which completion offers
	// once.
	cfg.Models["both-opus-responses"] = domain.Model{Provider: "both", Family: "opus", UpstreamModel: "opus", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return Context{Clients: clients.Registry(), Config: configStore, Secrets: forbiddenStore{t}, HTTP: forbiddenHTTP{t}}
}

func values(completions []cobra.Completion) []string {
	names := make([]string, 0, len(completions))
	for _, completion := range completions {
		name, _, _ := strings.Cut(completion, "\t")
		names = append(names, name)
	}
	return names
}

func TestCompletionReadsOnlyTheLocalConfiguration(t *testing.T) {
	runtime := completionRuntime(t)
	for _, test := range []struct {
		name     string
		complete cobra.CompletionFunc
		args     []string
		want     []string
	}{
		{"clients", CompleteClients(runtime), nil, []string{"claude", "claude-desktop", "codex", "hermes"}},
		{"providers", CompleteProviders(runtime), nil, []string{"both", "messages", "responses"}},
		{"every model", CompleteModels(runtime), nil, []string{"both/opus", "messages/opus", "responses/openai/gpt-sol"}},
		{"a given model", CompleteModels(runtime), []string{"messages/opus"}, []string{}},
		// `use <TAB>` completes the client, and `use <client> <TAB>` the models
		// that the client can use: Codex speaks only responses, so the
		// Messages-only provider's model is not offered.
		{"use's client", CompleteClientModels(runtime), nil, []string{"claude", "claude-desktop", "codex", "hermes"}},
		{"Codex's models", CompleteClientModels(runtime), []string{codex.ID}, []string{"both/opus", "responses/openai/gpt-sol"}},
		{"Claude's models", CompleteClientModels(runtime), []string{claude.ID}, []string{"both/opus", "messages/opus"}},
		{"an unknown client's models", CompleteClientModels(runtime), []string{"codx"}, []string{}},
		{"use's third argument", CompleteClientModels(runtime), []string{codex.ID, "both/opus"}, []string{}},
		// `check <client> --model <TAB>` completes the models that the named
		// client can use, and every model before a client is named.
		{"check's model for Codex", CompleteClientModel(runtime), []string{codex.ID}, []string{"both/opus", "responses/openai/gpt-sol"}},
		{"check's model without a client", CompleteClientModel(runtime), nil, []string{"both/opus", "messages/opus", "responses/openai/gpt-sol"}},
		{"families", CompleteFamilies(runtime), nil, []string{"opus", "sol"}},
		{"protocols", completeProtocols, nil, []string{"anthropic", "responses", "chat"}},
		{"capabilities", CompleteCapabilities, nil, []string{"text", "reasoning", "streaming", "tools", "structured_output", "continuation", "compaction", "multimodal_input", "tool_search"}},
		// A new name and a free-text query offer nothing.
		{"a new name", CompleteNothing, nil, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			completions, directive := test.complete(&cobra.Command{}, test.args, "")
			if got := values(completions); !slices.Equal(got, test.want) || directive != cobra.ShellCompDirectiveNoFileComp {
				t.Fatalf("completions = %q, directive %v; want %q without files", got, directive, test.want)
			}
		})
	}
	// Each model completes with its label as the description.
	if completions, _ := CompleteClientModels(runtime)(&cobra.Command{}, []string{claude.ID}, ""); !slices.Equal(completions, []cobra.Completion{"both/opus\tBoth · Opus", "messages/opus\tMessages · Opus"}) {
		t.Fatalf("model completion = %q", completions)
	}
}

func TestCompletionOffersNothingWithoutALoadableConfiguration(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := os.WriteFile(configStore.Path(), []byte("version = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := Context{Clients: clients.Registry(), Config: configStore, Secrets: forbiddenStore{t}, HTTP: forbiddenHTTP{t}}
	for name, complete := range map[string]cobra.CompletionFunc{
		"providers": CompleteProviders(runtime),
		"models":    CompleteModels(runtime),
		"families":  CompleteFamilies(runtime),
	} {
		if completions, directive := complete(&cobra.Command{}, nil, ""); len(completions) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("%s completion of a broken configuration = %q, %v", name, completions, directive)
		}
	}
}

// TestClientModelCompletionReadsTheClientArgument completes `demo <client>
// <TAB>` through Cobra, as a shell does for `aigw use`: the client given as
// the first argument narrows the models offered, and each name appears once.
func TestClientModelCompletionReadsTheClientArgument(t *testing.T) {
	runtime := completionRuntime(t)
	for client, want := range map[string]string{
		codex.ID:  "both/opus\nresponses/openai/gpt-sol\n:4\n",
		claude.ID: "both/opus\nmessages/opus\n:4\n",
	} {
		root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
		demo := &cobra.Command{Use: "demo <client> <provider/model>", RunE: func(*cobra.Command, []string) error { return nil }}
		demo.ValidArgsFunction = CompleteClientModels(runtime)
		root.AddCommand(demo)
		var out, errOut bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errOut)
		root.SetArgs([]string{cobra.ShellCompNoDescRequestCmd, "demo", client, ""})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Fatalf("demo %s <TAB> = %q, want %q", client, out.String(), want)
		}
	}
}
