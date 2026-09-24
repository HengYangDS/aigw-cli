package route

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestRouteInventoryDerivesUnlabeledTeamRouteName(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["ucloud"] = configuration.Account{Label: "UCloud", Endpoints: configuration.Endpoints{OpenAIResponses: "https://ucloud.test/v1"}}
	cfg.Models["gpt-6-sol"] = configuration.Model{Label: "GPT-6 Sol"}
	cfg.Routes["ucloud-gpt-6-sol"] = configuration.Route{
		Account: "ucloud", Model: "gpt-6-sol", UpstreamModel: "GPT-6-Sol",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {configuration.CapabilityText}},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Config: store, Out: out, RenderOut: out}
	list := newListCommand(runtime)
	list.SetArgs([]string{"--json"})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	var listed routeListOutput
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Routes) != 1 || listed.Routes[0].Label != "UCloud · GPT-6 Sol" {
		t.Fatalf("derived route inventory = %+v", listed.Routes)
	}
	out.Reset()
	show := newShowCommand(runtime)
	show.SetArgs([]string{"ucloud-gpt-6-sol", "--json"})
	if err := show.Execute(); err != nil {
		t.Fatal(err)
	}
	var shown struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(out.Bytes(), &shown); err != nil || shown.Label != "UCloud · GPT-6 Sol" {
		t.Fatalf("derived route detail = %+v, %v", shown, err)
	}
}

func TestRouteMutationsReturnConfigurationTransactionFailures(t *testing.T) {
	tests := []struct {
		name    string
		command func(invocation.Context) commandExecutor
		args    []string
	}{
		{
			name: "add",
			command: func(runtime invocation.Context) commandExecutor {
				return newAddCommand(runtime)
			},
			args: []string{"new", "--account", "current", "--model", "gpt-new", "--protocol", "openai_responses"},
		},
		{
			name: "edit",
			command: func(runtime invocation.Context) commandExecutor {
				return newEditCommand(runtime)
			},
			args: []string{"current", "--label", "Renamed"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := blockedRouteRuntime(t)
			command := test.command(runtime)
			command.SetArgs(test.args)
			if err := command.Execute(); err == nil {
				t.Fatal("configuration transaction failure was accepted")
			}
		})
	}
}

func TestChoiceLabelUsesRouteMetadataWithoutGlobalRanking(t *testing.T) {
	route := configuration.Route{Label: "UCloud · Grok 4.6", Purpose: "Coding"}
	if got := choiceLabel(route); got != "UCloud · Grok 4.6 · Coding" {
		t.Fatalf("choice label = %q", got)
	}
}

type commandExecutor interface {
	SetArgs(args []string)
	Execute() error
}

func TestRemoveRouteRemovesOnlyItsRecommendation(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
	cfg.Routes["claude"] = configuration.Route{
		Label: "Claude", Account: "team", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.Routes["codex"] = configuration.Route{
		Label: "Codex", Account: "team", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetRecommendedRoute(configuration.ClientClaude, "claude")
	cfg.SetRecommendedRoute(configuration.ClientCodex, "codex")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := newRemoveCommand(invocation.Context{Config: store, Secrets: secrets.NewMemoryStore(), Out: &bytes.Buffer{}, RenderOut: &bytes.Buffer{}})
	command.SetArgs([]string{"claude"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil || after.RecommendedRoute(configuration.ClientClaude) != "" || after.RecommendedRoute(configuration.ClientCodex) != "codex" {
		t.Fatalf("removal recommendation state = %#v, %v", after.Recommendations, err)
	}
}

func blockedRouteRuntime(t *testing.T) invocation.Context {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	cfg := configuration.NewConfig()
	cfg.Accounts["current"] = configuration.Account{
		Label:     "Current",
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://current.test/v1"},
	}
	cfg.Routes["current"] = configuration.Route{
		Label:   "Current",
		Account: "current",
		Model:   "gpt-current",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolOpenAIResponses: {},
		},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	backup := path + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	return invocation.Context{
		Config:    store,
		Secrets:   secrets.NewMemoryStore(),
		Out:       &bytes.Buffer{},
		RenderOut: &bytes.Buffer{},
	}
}
