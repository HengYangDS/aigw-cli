package selection

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
)

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
	if _, err := chooseRoute(runtime, cfg, configuration.ClientHermes, "Select Route"); err != nil ||
		!slices.ContainsFunc(selector.choices, func(choice prompt.Choice) bool { return choice.Value == "shared" }) {
		t.Fatalf("Hermes Route choice = %#v, %v", selector.choices, err)
	}

	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientHermes, "shared"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--protocol") {
		t.Fatalf("ambiguous Hermes Route error = %v, want explicit protocol guidance", err)
	}
	unchanged, err := runtime.Config.Load()
	if err != nil || unchanged.SelectedRoute(configuration.ClientHermes) != "" {
		t.Fatalf("ambiguous selection changed Hermes binding: %#v, %v", unchanged.Clients, err)
	}

	runtime.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientHermes, "--protocol", string(configuration.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--protocol anthropic") {
		t.Fatalf("missing Token recovery lost the protocol choice: %v", err)
	}
	runtime.Secrets = store

	out.Reset()
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientHermes, "--protocol", string(configuration.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	selected, err := runtime.Config.Load()
	if err != nil || selected.SelectedRoute(configuration.ClientHermes) != "shared" ||
		selected.Clients[configuration.ClientHermes].Protocol != configuration.ProtocolAnthropic ||
		selected.SelectedRoute(configuration.ClientCodex) != "codex" {
		t.Fatalf("explicit Hermes protocol selection = %#v, %v", selected.Clients, err)
	}
	if output := out.String(); !strings.Contains(output, "Install Hermes if needed, then run `aigw sync`") || strings.Contains(output, "aigw check") {
		t.Fatalf("deferred Hermes selection = %q", output)
	}

	runtime.Interactive = true
	runtime.Prompt = &promptStub{selected: string(configuration.ProtocolOpenAIResponses)}
	interactive, err := resolveUseRuntime(runtime, cfg, configuration.ClientHermes, "shared", "", nil)
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
	command.SetArgs([]string{"--for", configuration.ClientHermes, "--protocol", string(configuration.ProtocolAnthropic), "shared"})
	if err := command.Execute(); err != nil || requests != 1 {
		t.Fatalf("selected-protocol Token validation: requests=%d error=%v", requests, err)
	}
}
