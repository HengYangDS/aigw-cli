package route

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"github.com/spf13/cobra"
)

func TestRouteInventoryDerivesUnlabeledTeamRouteName(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{OpenAIResponses: "https://ucloud.test/v1"}}
	cfg.Families["gpt-6-sol"] = domain.Family{Label: "GPT-6 Sol"}
	cfg.Models["ucloud-gpt-6-sol"] = domain.Model{
		Provider: "ucloud", Family: "gpt-6-sol", UpstreamModel: "GPT-6-Sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}},
	}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Config: configStore, Out: out}
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

func TestRouteHumanOutputNamesClientsByLabel(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["messages"] = domain.Model{
		Provider: "gateway", Family: "model-a",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(claudedesktop.ID, "messages", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Config: configStore, Out: out}
	for _, command := range []*cobra.Command{newListCommand(runtime), newShowCommand(runtime)} {
		out.Reset()
		if command.Name() == "show" {
			command.SetArgs([]string{"messages"})
		} else {
			command.SetArgs([]string{})
		}
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(out.String()), " ")
		for _, want := range []string{"Selected for Claude Desktop", "Claude, Claude Desktop, Hermes"} {
			if !strings.Contains(text, want) {
				t.Fatalf("route %s output lacks %q:\n%s", command.Name(), want, out.String())
			}
		}
		if strings.Contains(out.String(), "claude-desktop") {
			t.Fatalf("route %s output named a client by its identifier:\n%s", command.Name(), out.String())
		}
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
	route := domain.Model{Label: "UCloud · Grok 4.6", Purpose: "Coding"}
	if got := choiceLabel(route); got != "UCloud · Grok 4.6 · Coding" {
		t.Fatalf("choice label = %q", got)
	}
}

type commandExecutor interface {
	SetArgs(args []string)
	Execute() error
}

func TestRemoveRouteRemovesOnlyItsRecommendation(t *testing.T) {
	for _, alternatives := range [][]domain.ClientSelection{nil, {
		{Model: "backup", Protocol: domain.ProtocolAnthropic},
		{Model: "reserve", Protocol: domain.ProtocolAnthropic},
	}} {
		t.Run(fmt.Sprint(len(alternatives)), func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			cfg := domain.NewConfig(clients.Admission())
			cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
			cfg.Models["claude"] = domain.Model{
				Label: "Claude", Provider: "team", Family: "claude-test",
				Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
			}
			cfg.Models["codex"] = domain.Model{
				Label: "Codex", Provider: "team", Family: "gpt-test",
				Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
			}
			cfg.SetRecommendedModel(claude.ID, "claude")
			cfg.SetRecommendedModel(codex.ID, "codex")
			cfg.SetSelectedModel(codex.ID, "codex", "")
			for _, selection := range alternatives {
				cfg.Models[selection.Model] = cfg.Models["claude"]
			}
			cfg.Recommendations[claude.ID] = domain.ClientRecommendation{
				Primary: domain.ClientSelection{Model: "claude", Protocol: domain.ProtocolAnthropic}, Alternatives: alternatives,
			}
			if err := configStore.Save(cfg); err != nil {
				t.Fatal(err)
			}
			secretStore := secret.NewMemoryStore()
			if err := secretStore.Set("team", "retained-token"); err != nil {
				t.Fatal(err)
			}
			command := newRemoveCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secretStore, Out: &bytes.Buffer{}})
			command.SetArgs([]string{"claude"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			after, err := configStore.Load()
			want := domain.ClientRecommendation{}
			if len(alternatives) != 0 {
				want = domain.ClientRecommendation{Primary: alternatives[0], Alternatives: alternatives[1:]}
			}
			if err != nil || !reflect.DeepEqual(after.Recommendations[claude.ID], want) || after.RecommendedModel(codex.ID) != "codex" {
				t.Fatalf("removal recommendation state = %#v, %v", after.Recommendations, err)
			}
			if token, err := secretStore.Get("team"); err != nil || token != "retained-token" || !reflect.DeepEqual(after.Providers, cfg.Providers) || !reflect.DeepEqual(after.Clients, cfg.Clients) {
				t.Fatalf("removal changed unrelated account, credential or client state: %v", err)
			}
		})
	}
}

// TestRemovePreviewNamesTheRecommendationsThatItChanges removes a Route that a
// client recommends as an alternative: the preview names that client and
// writes nothing, and the removal keeps the client's first choice and writes
// its document without them.
func TestRemovePreviewNamesTheRecommendationsThatItChanges(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	for _, name := range []string{"first", "spare"} {
		cfg.Models[name] = domain.Model{
			Label: name, Provider: "team", Family: name + "-model",
			Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
		}
	}
	first := domain.ClientSelection{Model: "first", Protocol: domain.ProtocolAnthropic}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{
		Primary: first, Alternatives: []domain.ClientSelection{{Model: "spare", Protocol: domain.ProtocolAnthropic}},
	}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(configStore.Path())
	if err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	command := newRemoveCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: out})
	command.SetArgs([]string{"spare", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var document struct {
		DryRun          bool     `json:"dry_run"`
		Recommendations []string `json:"recommendations"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil || !document.DryRun || !slices.Equal(document.Recommendations, []string{claude.ID}) {
		t.Fatalf("removal preview = %s, %v", out.String(), err)
	}
	rendered := &bytes.Buffer{}
	command = newRemoveCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: rendered})
	command.SetArgs([]string{"spare", "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "Route removal preview") || !strings.Contains(rendered.String(), "Recommendations") {
		t.Fatalf("removal preview text = %s", rendered.String())
	}
	if after, err := os.ReadFile(configStore.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("removal preview changed the configuration: %v", err)
	}
	out.Reset()
	command = newRemoveCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: out})
	command.SetArgs([]string{"spare", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	// Only a preview names the changed recommendations.
	if strings.Contains(out.String(), "recommendations") {
		t.Fatalf("removal document = %s", out.String())
	}
	after, err := configStore.Load()
	if err != nil || !reflect.DeepEqual(after.Recommendations[claude.ID], domain.ClientRecommendation{Primary: first}) {
		t.Fatalf("removal recommendation state = %#v, %v", after.Recommendations, err)
	}
}

func blockedRouteRuntime(t *testing.T) invocation.Context {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(path, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["current"] = domain.Provider{
		Label:     "Current",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://current.test/v1"},
	}
	cfg.Models["current"] = domain.Model{
		Label:    "Current",
		Provider: "current",
		Family:   "gpt-current",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.SetSelectedModel(codex.ID, "current", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	backup := path + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	return invocation.Context{Clients: clients.Registry(),
		Config:  configStore,
		Secrets: secret.NewMemoryStore(),
		Out:     &bytes.Buffer{},
	}
}

func TestRouteAddSeparatesFamilyFromExactProviderWireID(t *testing.T) {
	for _, test := range []struct {
		name, upstream, wantWire string
	}{
		{name: "ordinary model", wantWire: "minimax-m3"},
		{name: "provider case", upstream: "MiniMax-M3", wantWire: "MiniMax-M3"},
		{name: "channel variant", upstream: "coding-minimax-m3", wantWire: "coding-minimax-m3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			cfg := domain.NewConfig(clients.Admission())
			cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{OpenAIResponses: "https://provider.test/v1"}}
			cfg.Families["minimax-m3"] = domain.Family{Label: "MiniMax M3"}
			cfg.Models["ucloud-minimax-m3"] = domain.Model{
				Provider: "ucloud", Family: "minimax-m3", UpstreamModel: "MiniMax-M3",
				Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
			}
			if err := configStore.Save(cfg); err != nil {
				t.Fatal(err)
			}
			out := &bytes.Buffer{}
			command := newAddCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: out})
			args := []string{"ucloud-minimax-new", "--account", "ucloud", "--model", "minimax-m3", "--protocol", "openai_responses"}
			if test.upstream != "" {
				args = append(args, "--upstream-model", test.upstream)
			}
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatalf("add exact provider ID: %v", err)
			}
			got, err := configStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			added := got.Models["ucloud-minimax-new"]
			if added.Family != "minimax-m3" || added.UpstreamModelID() != test.wantWire {
				t.Fatalf("canonical/wire identity = %#v", added)
			}
			if len(got.Families) != 1 || got.Families["minimax-m3"].Label != "MiniMax M3" || len(got.Clients) != 0 || got.Models["ucloud-minimax-m3"].UpstreamModelID() != "MiniMax-M3" {
				t.Fatalf("route addition changed existing identities or selections: %#v", got)
			}
		})
	}
}

func TestRouteAddRejectsEmptyExplicitUpstreamBeforeConfigurationAccess(t *testing.T) {
	for _, upstream := range []string{"", " \t"} {
		configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
		original := []byte("invalid configuration [")
		if err := os.WriteFile(configStore.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		command := newAddCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Out: &bytes.Buffer{}})
		command.SetArgs([]string{"ucloud-minimax-new", "--account", "ucloud", "--model", "minimax-m3", "--protocol", "openai_responses", "--upstream-model", upstream})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--upstream-model requires a non-empty value") {
			t.Fatalf("empty upstream was not rejected before loading configuration: %v", err)
		}
		if got, err := os.ReadFile(configStore.Path()); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("invalid upstream changed configuration: %v", err)
		}
	}
}
