package route

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if !strings.Contains(out.String(), `"protocol_clients"`) || strings.Contains(out.String(), `"compatible_clients"`) {
		t.Fatalf("protocol overlap was presented as client qualification: %s", out)
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
	if !strings.Contains(out.String(), `"protocol_clients"`) || strings.Contains(out.String(), `"compatible_clients"`) {
		t.Fatalf("route detail implied native client qualification: %s", out)
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

func TestRouteAddSeparatesCanonicalModelFromExactProviderWireID(t *testing.T) {
	for _, test := range []struct {
		name, upstream, wantWire string
	}{
		{name: "ordinary model", wantWire: "minimax-m3"},
		{name: "provider case", upstream: "MiniMax-M3", wantWire: "MiniMax-M3"},
		{name: "channel variant", upstream: "coding-minimax-m3", wantWire: "coding-minimax-m3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			cfg := configuration.NewConfig()
			cfg.Accounts["ucloud"] = configuration.Account{Label: "UCloud", Endpoints: configuration.Endpoints{OpenAIResponses: "https://provider.test/v1"}}
			cfg.Models["minimax-m3"] = configuration.Model{Label: "MiniMax M3"}
			cfg.Routes["ucloud-minimax-m3"] = configuration.Route{
				Account: "ucloud", Model: "minimax-m3", UpstreamModel: "MiniMax-M3",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
			}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			out := &bytes.Buffer{}
			command := newAddCommand(invocation.Context{Config: store, Secrets: secrets.NewMemoryStore(), Out: out, RenderOut: out})
			args := []string{"ucloud-minimax-new", "--account", "ucloud", "--model", "minimax-m3", "--protocol", "openai_responses"}
			if test.upstream != "" {
				args = append(args, "--upstream-model", test.upstream)
			}
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatalf("add exact provider ID: %v", err)
			}
			got, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			added := got.Routes["ucloud-minimax-new"]
			if added.Model != "minimax-m3" || added.UpstreamModelID() != test.wantWire {
				t.Fatalf("canonical/wire identity = %#v", added)
			}
			if len(got.Models) != 1 || got.Models["minimax-m3"].Label != "MiniMax M3" || len(got.Clients) != 0 || got.Routes["ucloud-minimax-m3"].UpstreamModelID() != "MiniMax-M3" {
				t.Fatalf("route addition changed existing identities or selections: %#v", got)
			}
		})
	}
}

func TestRouteAddRejectsEmptyExplicitUpstreamBeforeConfigurationAccess(t *testing.T) {
	for _, upstream := range []string{"", " \t"} {
		store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
		original := []byte("invalid configuration [")
		if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		command := newAddCommand(invocation.Context{Config: store, Out: &bytes.Buffer{}})
		command.SetArgs([]string{"ucloud-minimax-new", "--account", "ucloud", "--model", "minimax-m3", "--protocol", "openai_responses", "--upstream-model", upstream})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--upstream-model requires a non-empty value") {
			t.Fatalf("empty upstream was not rejected before loading configuration: %v", err)
		}
		if got, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("invalid upstream changed configuration: %v", err)
		}
	}
}
