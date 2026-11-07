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
	cfg.Models["shared"] = domain.Model{
		Provider: "gateway", Family: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
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
	cfg.Models["shared"] = domain.Model{
		Provider: "gateway", Family: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
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
	if err != nil || unchanged.SelectedModel(hermes.ID) != "" {
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
	if err != nil || selected.SelectedModel(hermes.ID) != "shared" ||
		selected.Clients[hermes.ID].Protocol != domain.ProtocolAnthropic ||
		selected.SelectedModel(codex.ID) != "codex" {
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
	if err != nil || interactive.protocol != domain.ProtocolOpenAIResponses || !interactive.chosen {
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
	cfg.Models["shared"] = domain.Model{
		Provider: "gateway", Family: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
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
	command.SetArgs([]string{hermes.ID, "gateway/shared-model", "--protocol", "anthropic"})
	if err := command.Execute(); err != nil || requests != 1 {
		t.Fatalf("selected-protocol Token validation: requests=%d error=%v", requests, err)
	}
}
