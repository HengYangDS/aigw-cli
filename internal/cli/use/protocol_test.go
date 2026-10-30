package use

import (
	"bytes"
	"encoding/json"
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
	runtime.Problem = terminal.ProblemError
	cfg.Routes["shared"] = domain.Route{
		Account: "gateway", Model: "shared-model",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--dry-run", "--json", "shared"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "--protocol") {
		t.Fatalf("ambiguous Hermes Route preview error = %v, want explicit protocol guidance", err)
	}
	var out bytes.Buffer
	renderer := terminal.NewRenderer(&out, false)
	terminal.RenderError(renderer, err, true)
	var rendered struct {
		NextAction string `json:"next_action"`
	}
	if decodeErr := json.Unmarshal(out.Bytes(), &rendered); decodeErr != nil || renderer.Err() != nil {
		t.Fatalf("JSON error = %q, %v, %v", out.String(), decodeErr, renderer.Err())
	}
	if want := "aigw use --for hermes --protocol <openai_responses|anthropic> shared"; rendered.NextAction != want {
		t.Fatalf("next_action = %q, want %q", rendered.NextAction, want)
	}
}

func TestUseSelectsExplicitHermesProtocol(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	cfg.Routes["shared"] = domain.Route{
		Account: "gateway", Model: "shared-model",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	selector := &promptStub{selected: "shared"}
	runtime.Prompt = selector
	if _, err := chooseRoute(runtime, cfg, hermes.ID, "Select Route"); err != nil ||
		!slices.ContainsFunc(selector.choices, func(choice terminal.Choice) bool { return choice.Value == "shared" }) {
		t.Fatalf("Hermes Route choice = %#v, %v", selector.choices, err)
	}

	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "shared"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--protocol") {
		t.Fatalf("ambiguous Hermes Route error = %v, want explicit protocol guidance", err)
	}
	unchanged, err := runtime.Config.Load()
	if err != nil || unchanged.SelectedRoute(hermes.ID) != "" {
		t.Fatalf("ambiguous selection changed Hermes binding: %#v, %v", unchanged.Clients, err)
	}

	runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	command = NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--protocol", string(domain.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--protocol anthropic") {
		t.Fatalf("missing Token recovery lost the protocol choice: %v", err)
	}
	runtime.Secrets = store

	out.Reset()
	command = NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--protocol", string(domain.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	selected, err := runtime.Config.Load()
	if err != nil || selected.SelectedRoute(hermes.ID) != "shared" ||
		selected.Clients[hermes.ID].Protocol != domain.ProtocolAnthropic ||
		selected.SelectedRoute(codex.ID) != "codex" {
		t.Fatalf("explicit Hermes protocol selection = %#v, %v", selected.Clients, err)
	}
	if output := out.String(); !strings.Contains(output, "Install Hermes if needed, then run `aigw sync`") || strings.Contains(output, "aigw check") {
		t.Fatalf("deferred Hermes selection = %q", output)
	}

	runtime.Interactive = true
	runtime.Prompt = &promptStub{selected: string(domain.ProtocolOpenAIResponses)}
	interactive, err := resolveUseRuntime(runtime, cfg, hermes.ID, "shared", "", domain.ConnectionChange{})
	if err != nil || interactive.Protocol != domain.ProtocolOpenAIResponses {
		t.Fatalf("interactive Hermes protocol = %#v, %v", interactive, err)
	}
}

func TestUseValidatesSelectedProtocolWhenAcquiringToken(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	cfg.Routes["shared"] = domain.Route{
		Account: "gateway", Model: "shared-model",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
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
	command.SetArgs([]string{"--for", hermes.ID, "--protocol", string(domain.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err != nil || requests != 1 {
		t.Fatalf("selected-protocol Token validation: requests=%d error=%v", requests, err)
	}
}
