package selection

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestInteractiveRouteChoiceDerivesUnlabeledRouteName(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	route := cfg.Routes["codex"]
	route.Label = ""
	cfg.Routes["codex"] = route
	cfg.Models["gpt-test"] = configuration.Model{Label: "GPT Test"}
	choice := &promptStub{selected: "codex"}
	runtime.Prompt = choice
	selected, err := chooseRoute(runtime, cfg, configuration.ClientCodex, "Select Route")
	if err != nil || selected != "codex" || len(choice.choices) != 1 || choice.choices[0].Label != "Gateway · GPT Test" {
		t.Fatalf("derived interactive Route choice = %q, %+v, %v", selected, choice.choices, err)
	}
}

func TestUsePromptsForACompatibleClientOnASharedRoute(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	cfg.Routes["shared"] = configuration.Route{
		Label: "Shared", Account: "gateway", Model: "shared-model",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic:       {},
			configuration.ProtocolOpenAIResponses: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "synthetic-token"); err != nil {
		t.Fatal(err)
	}
	selector := &promptStub{selected: configuration.ClientClaude}
	runtime.Secrets = store
	runtime.Prompt = selector
	runtime.Interactive = true
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"shared"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	choices := map[string]bool{}
	for _, choice := range selector.choices {
		choices[choice.Value] = true
	}
	if !choices[configuration.ClientClaude] || !choices[configuration.ClientCodex] {
		t.Fatalf("shared Route client choices = %#v", selector.choices)
	}
	stored, err := runtime.Config.Load()
	if err != nil || stored.Clients[configuration.ClientClaude].Route != "shared" {
		t.Fatalf("interactive client selection = %#v, %v", stored.Clients, err)
	}
}

func TestUseInteractiveSelectionAndValidationFailures(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	secretStore := secrets.NewMemoryStore()
	runtime.Secrets = secretStore
	selector := &promptStub{selected: "codex"}
	runtime.Prompt = selector
	runtime.Interactive = true
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.Routes["purpose"] = configuration.Route{
		Label: "Purpose", Purpose: "Research", Account: "gateway", Model: "claude-research",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(selector.choices) != 2 || selector.choices[0].Value != "codex" || selector.choices[0].Label != "Codex" || selector.choices[1].Value != "purpose" || selector.choices[1].Label != "Purpose · Research" {
		t.Fatalf("choices = %#v", selector.choices)
	}

	for _, test := range []struct {
		name    string
		args    []string
		runtime func(invocation.Context) invocation.Context
		want    string
	}{
		{name: "route required", runtime: func(value invocation.Context) invocation.Context { value.Interactive = false; return value }, want: "requires a Route"},
		{name: "unknown route", args: []string{"missing"}, want: "unknown route"},
		{name: "load", args: []string{"codex"}, runtime: func(value invocation.Context) invocation.Context {
			value.Config = configuration.NewStore(t.TempDir())
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
			command := NewUseCommand(value)
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetArgs(test.args)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
