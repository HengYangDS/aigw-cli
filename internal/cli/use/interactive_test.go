package use

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// interactiveRuntime is a terminal session whose gateway provider has a Token.
func interactiveRuntime(t *testing.T, selector *promptStub) (invocation.Context, domain.Config) {
	t.Helper()
	runtime, cfg, _ := configuredRuntime(t)
	credentials := secret.NewMemoryStore()
	if err := credentials.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets, runtime.Prompt, runtime.Interactive = credentials, selector, true
	return runtime, cfg
}

// The model prompt offers each added model that the client can use once, by
// its name and label, the client's recommended models first and marked; a
// model whose provider serves none of the client's protocols is not offered.
func TestUsePromptsForTheModelRecommendedFirst(t *testing.T) {
	selector := &promptStub{answers: []string{"gateway/gpt-second"}}
	runtime, cfg := interactiveRuntime(t, selector)
	unlabeled := cfg.Models["codex"]
	unlabeled.Label = ""
	cfg.Models["codex"] = unlabeled
	cfg.Families["gpt-test"] = domain.Family{Label: "GPT Test"}
	cfg.Families["gpt-second"] = domain.Family{Label: "GPT Second"}
	cfg.Models["second"] = domain.Model{Label: "Second", Provider: "gateway", Family: "gpt-second", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Providers["messages"] = domain.Provider{Label: "Messages", Endpoints: domain.Endpoints{Anthropic: "https://messages.example.test"}}
	cfg.Models["opus"] = domain.Model{Label: "Opus", Provider: "messages", Family: "opus", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Recommendations[codex.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: "second", Protocol: domain.ProtocolOpenAIResponses}}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(runtime, codex.ID); err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, choice := range selector.choices {
		labels = append(labels, choice.Value+" = "+choice.Label)
	}
	if want := "gateway/gpt-second = gateway/gpt-second · Second · recommended|gateway/gpt-test = gateway/gpt-test · Gateway · GPT Test"; strings.Join(labels, "|") != want {
		t.Fatalf("model choices = %q, want %q", labels, want)
	}
	if stored, err := runtime.Config.Load(); err != nil || stored.SelectedModel(codex.ID) != "second" {
		t.Fatalf("interactive model selection = %#v, %v", stored.Clients, err)
	}
}

// A model without a client asks for a client that speaks a protocol that its
// provider serves, and no arguments ask for the client and then the model.
func TestUsePromptsForAClientAndThenAModel(t *testing.T) {
	selector := &promptStub{answers: []string{claude.ID}}
	runtime, cfg := interactiveRuntime(t, selector)
	cfg.Models["shared"] = domain.Model{
		Label: "Shared", Provider: "gateway", Family: "shared-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {}},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(runtime, "gateway/shared-model"); err != nil {
		t.Fatal(err)
	}
	clientChoices := map[string]bool{}
	for _, choice := range selector.choices {
		clientChoices[choice.Value] = true
	}
	if !clientChoices[claude.ID] || !clientChoices[codex.ID] {
		t.Fatalf("client choices = %#v", selector.choices)
	}
	selector.answers = []string{codex.ID, "gateway/shared-model"}
	if err := execute(runtime); err != nil {
		t.Fatal(err)
	}
	stored, err := runtime.Config.Load()
	if err != nil || stored.SelectedModel(claude.ID) != "shared" || stored.SelectedModel(codex.ID) != "shared" {
		t.Fatalf("interactive selections = %#v, %v", stored.Clients, err)
	}
}

func TestUseInteractiveFailures(t *testing.T) {
	selector := &promptStub{selected: codex.ID}
	runtime, _ := interactiveRuntime(t, selector)
	for _, test := range []struct {
		name    string
		args    []string
		runtime func(invocation.Context) invocation.Context
		want    string
	}{
		{name: "arguments required", runtime: func(value invocation.Context) invocation.Context { value.Interactive = false; return value },
			want: "aigw use without a terminal needs a <client> and a <provider/model> argument"},
		{name: "unknown model", args: []string{codex.ID, "gateway/missing"}, want: "unknown model gateway/missing"},
		{name: "load", args: []string{codex.ID, "gateway/gpt-test"}, runtime: func(value invocation.Context) invocation.Context {
			value.Config = store.NewStore(t.TempDir(), clients.Admission())
			return value
		}, want: "read"},
		{name: "selection", runtime: func(value invocation.Context) invocation.Context {
			value.Prompt = &promptStub{err: errors.New("cancelled")}
			return value
		}, want: "cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := runtime
			if test.runtime != nil {
				value = test.runtime(value)
			}
			if err := execute(value, test.args...); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
