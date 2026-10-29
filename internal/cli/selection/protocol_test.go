package selection

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
)

func TestNoninteractiveProtocolChoiceNamesItsRetry(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	runtime.Problem = presentation.ProblemError
	cfg.Routes["shared"] = configuration.Route{
		Account: "gateway", Model: "shared-model",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic: {}, configuration.ProtocolOpenAIResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--dry-run", "--json", "shared"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "--protocol") {
		t.Fatalf("ambiguous Hermes Route preview error = %v, want explicit protocol guidance", err)
	}
	var out bytes.Buffer
	renderer := presentation.New(&out, false)
	presentation.RenderError(renderer, err, true)
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
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	cfg.Routes["shared"] = configuration.Route{
		Account: "gateway", Model: "shared-model",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic: {}, configuration.ProtocolOpenAIResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	selector := &promptStub{selected: "shared"}
	runtime.Prompt = selector
	if _, err := chooseRoute(runtime, cfg, hermes.ID, "Select Route"); err != nil ||
		!slices.ContainsFunc(selector.choices, func(choice prompt.Choice) bool { return choice.Value == "shared" }) {
		t.Fatalf("Hermes Route choice = %#v, %v", selector.choices, err)
	}

	command := NewUseCommand(runtime)
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

	runtime.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--protocol", string(configuration.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--protocol anthropic") {
		t.Fatalf("missing Token recovery lost the protocol choice: %v", err)
	}
	runtime.Secrets = store

	out.Reset()
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--protocol", string(configuration.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	selected, err := runtime.Config.Load()
	if err != nil || selected.SelectedRoute(hermes.ID) != "shared" ||
		selected.Clients[hermes.ID].Protocol != configuration.ProtocolAnthropic ||
		selected.SelectedRoute(codex.ID) != "codex" {
		t.Fatalf("explicit Hermes protocol selection = %#v, %v", selected.Clients, err)
	}
	if output := out.String(); !strings.Contains(output, "Install Hermes if needed, then run `aigw sync`") || strings.Contains(output, "aigw check") {
		t.Fatalf("deferred Hermes selection = %q", output)
	}

	runtime.Interactive = true
	runtime.Prompt = &promptStub{selected: string(configuration.ProtocolOpenAIResponses)}
	interactive, err := resolveUseRuntime(runtime, cfg, hermes.ID, "shared", "", configuration.ConnectionChange{})
	if err != nil || interactive.Protocol != configuration.ProtocolOpenAIResponses {
		t.Fatalf("interactive Hermes protocol = %#v, %v", interactive, err)
	}
}

func TestUseValidatesSelectedProtocolWhenAcquiringToken(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	cfg.Routes["shared"] = configuration.Route{
		Account: "gateway", Model: "shared-model",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic: {}, configuration.ProtocolOpenAIResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = secrets.NewMemoryStore()
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
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", hermes.ID, "--protocol", string(configuration.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err != nil || requests != 1 {
		t.Fatalf("selected-protocol Token validation: requests=%d error=%v", requests, err)
	}
}
