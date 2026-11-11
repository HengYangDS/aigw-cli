package use

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"

	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

func TestNoninteractiveProtocolChoiceNamesItsRetry(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	cfg.Models[domain.NewModelRef("gateway", "shared-model")] = domain.Model{
		Provider: "gateway", Family: "shared-model", ID: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{hermes.ID, "gateway/shared-model", "--dry-run", "--json"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "the protocol must be chosen") {
		t.Fatalf("ambiguous Hermes preview error = %v, want explicit protocol guidance", err)
	}
	var out bytes.Buffer
	renderer := terminal.NewRenderer(&out, false)
	terminal.RenderError(renderer, err, true)
	// The retry needs a choice, so the failure gives it as next_detail.
	var rendered struct {
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	}
	if decodeErr := json.Unmarshal(out.Bytes(), &rendered); decodeErr != nil || renderer.Err() != nil {
		t.Fatalf("JSON error = %q, %v, %v", out.String(), decodeErr, renderer.Err())
	}
	if want := "aigw use hermes gateway/shared-model --protocol <responses|anthropic>"; rendered.NextAction != "" || rendered.NextDetail != want {
		t.Fatalf("next step = %q, %q; want %q", rendered.NextAction, rendered.NextDetail, want)
	}
}

func TestUseSelectsExplicitHermesProtocol(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	cfg.Models[domain.NewModelRef("gateway", "shared-model")] = domain.Model{
		Provider: "gateway", Family: "shared-model", ID: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	selector := &promptStub{selected: "gateway/shared-model"}
	runtime.Prompt = selector
	hermesSpec, _ := runtime.Clients.Admission().Spec(hermes.ID)
	if _, err := chooseModel(runtime, cfg, hermesSpec); err != nil ||
		!slices.ContainsFunc(selector.choices, func(choice terminal.Choice) bool { return choice.Value == "gateway/shared-model" }) {
		t.Fatalf("Hermes model choice = %#v, %v", selector.choices, err)
	}

	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{hermes.ID, "gateway/shared-model"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "the protocol must be chosen") {
		t.Fatalf("ambiguous Hermes error = %v, want explicit protocol guidance", err)
	}
	unchanged, err := runtime.Config.Load()
	if err != nil || !unchanged.SelectedModel(hermes.ID).IsZero() {
		t.Fatalf("ambiguous selection changed Hermes binding: %#v, %v", unchanged.Clients, err)
	}

	runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	command = NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{hermes.ID, "gateway/shared-model", "--protocol", "anthropic"})
	if err := command.Execute(); err == nil || !strings.Contains(terminal.ProblemFor(err).Fix, "--protocol anthropic") {
		t.Fatalf("missing Token recovery lost the protocol choice: %v", err)
	}
	runtime.Secrets = store

	out.Reset()
	command = NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{hermes.ID, "gateway/shared-model", "--protocol", "anthropic"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	selected, err := runtime.Config.Load()
	if err != nil || selected.SelectedModel(hermes.ID) != domain.NewModelRef("gateway", "shared-model") ||
		selected.Clients[hermes.ID].Protocol != domain.ProtocolAnthropic ||
		selected.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-test") {
		t.Fatalf("explicit Hermes protocol selection = %#v, %v", selected.Clients, err)
	}
	if output := out.String(); !strings.Contains(output, "Install Hermes if needed, then run `aigw sync`") || strings.Contains(output, "aigw check") {
		t.Fatalf("deferred Hermes selection = %q", output)
	}

	// In a terminal, a prompt chooses, and a later selection keeps the
	// client's current protocol without asking.
	runtime.Interactive = true
	runtime.Prompt = &promptStub{selected: "responses"}
	interactive, err := chooseProtocol(runtime, cfg, hermesSpec, domain.NewModelRef("gateway", "shared-model"), "")
	if err != nil || interactive.protocol != domain.ProtocolResponses || !interactive.chosen {
		t.Fatalf("interactive Hermes protocol = %#v, %v", interactive, err)
	}
	runtime.Prompt = &promptStub{err: errors.New("the current protocol was asked for")}
	current, err := chooseProtocol(runtime, selected, hermesSpec, domain.NewModelRef("gateway", "shared-model"), "")
	if err != nil || current.protocol != domain.ProtocolAnthropic {
		t.Fatalf("Hermes's current protocol = %#v, %v", current, err)
	}
}

func TestUseValidatesSelectedProtocolWhenAcquiringToken(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	cfg.Models[domain.NewModelRef("gateway", "shared-model")] = domain.Model{
		Provider: "gateway", Family: "shared-model", ID: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = secret.NewMemoryStore()
	runtime.Interactive = true
	runtime.Prompt = &promptStub{secret: "token"}
	requests := 0
	runtime.HTTP = doerFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Header.Get("X-Api-Key") != "token" || request.Header.Get("Authorization") != "" {
			t.Fatal("Hermes Token validation used the wrong protocol authentication")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
	})
	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{hermes.ID, "gateway/shared-model", "--protocol", "anthropic"})
	if err := command.Execute(); err != nil || requests != 1 {
		t.Fatalf("selected-protocol Token validation: requests=%d error=%v", requests, err)
	}
}

// A selection that no protocol can carry is refused before any credential or
// network access, with the fix for its cause: --protocol that the client does
// not speak names the command's help, a protocol that the provider does not
// serve names the provider's endpoint, and a provider that serves none of the
// client's protocols names an endpoint on one of them.
func TestUseRefusesAProtocolThatCannotCarryTheSelection(t *testing.T) {
	for _, test := range []struct {
		name, client, model, protocol, problem, fix string
		endpoints                                   domain.Endpoints
	}{
		{name: "not spoken", client: codex.ID, model: "gateway/gpt-test", protocol: "anthropic",
			problem: "Codex does not speak Anthropic Messages; it speaks OpenAI Responses", fix: "aigw use --help"},
		{name: "not served", client: hermes.ID, model: "gateway/gpt-test", protocol: "chat",
			problem: "provider gateway has no OpenAI Chat Completions endpoint for gateway/gpt-test", fix: "aigw provider edit gateway --endpoint chat=<url>"},
		{name: "none served", client: codex.ID, model: "gateway/gpt-test", endpoints: domain.Endpoints{Anthropic: "https://gateway.example.test"},
			problem: "Codex cannot use gateway/gpt-test: it speaks OpenAI Responses, and provider gateway has no endpoint for any of them",
			fix:     "aigw provider edit gateway --endpoint responses=<url>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			if test.endpoints != (domain.Endpoints{}) {
				provider := cfg.Providers["gateway"]
				provider.Endpoints = test.endpoints
				cfg.Providers["gateway"] = provider
				delete(cfg.Models, domain.NewModelRef("gateway", "gpt-test"))
				delete(cfg.Clients, codex.ID)
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
			}
			before, err := runtime.Config.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			counted := &countingStore{Store: secret.NewMemoryStore()}
			runtime.Secrets = counted
			runtime.HTTP = doerFunc(func(request *http.Request) (*http.Response, error) {
				t.Fatalf("a refused selection sent %s", request.URL)
				return nil, nil
			})
			args := []string{test.client, test.model}
			if test.protocol != "" {
				args = append(args, "--protocol", test.protocol)
			}
			err = execute(runtime, args...)
			if p := terminal.ProblemFor(err); err == nil || p.Title != test.problem || p.Fix != test.fix {
				t.Fatalf("%s = %q, fix %q (%v)", test.name, p.Title, p.Fix, err)
			}
			if counted.accesses != 0 {
				t.Fatalf("a refused selection read or wrote %d Tokens", counted.accesses)
			}
			after, err := runtime.Config.CaptureSnapshot()
			if err != nil || !slices.Equal(after.Config.Data, before.Config.Data) || out.Len() != 0 {
				t.Fatalf("a refused selection changed the configuration or wrote %q: %v", out, err)
			}
		})
	}
}

// countingStore counts the Token reads and writes of the store it wraps.
type countingStore struct {
	secret.Store
	accesses int
}

func (store *countingStore) Get(provider string) (string, error) {
	store.accesses++
	return store.Store.Get(provider)
}

func (store *countingStore) Set(provider, value string) error {
	store.accesses++
	return store.Store.Set(provider, value)
}
