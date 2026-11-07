package invocation

import (
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
		{"every model", CompleteModels(runtime, func() string { return "" }), nil, []string{"both/opus", "messages/opus", "responses/openai/gpt-sol"}},
		// Codex speaks only responses, so the Messages-only provider's model is
		// not offered.
		{"Codex's models", CompleteModels(runtime, func() string { return codex.ID }), nil, []string{"both/opus", "responses/openai/gpt-sol"}},
		{"Claude's models", CompleteModels(runtime, func() string { return claude.ID }), nil, []string{"both/opus", "messages/opus"}},
		{"a given model", CompleteModels(runtime, func() string { return "" }), []string{"messages/opus"}, []string{}},
		{"families", CompleteFamilies(runtime), nil, []string{"opus", "sol"}},
		{"protocols", completeProtocols, nil, []string{"anthropic", "responses", "chat"}},
		{"depths", completeDepths, nil, []string{"local", "endpoint", "model", "client"}},
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
	if completions, _ := CompleteModels(runtime, func() string { return claude.ID })(&cobra.Command{}, nil, ""); !slices.Equal(completions, []cobra.Completion{"both/opus\tBoth · Opus", "messages/opus\tMessages · Opus"}) {
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
		"models":    CompleteModels(runtime, func() string { return "" }),
		"families":  CompleteFamilies(runtime),
	} {
		if completions, directive := complete(&cobra.Command{}, nil, ""); len(completions) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("%s completion of a broken configuration = %q, %v", name, completions, directive)
		}
	}
}
