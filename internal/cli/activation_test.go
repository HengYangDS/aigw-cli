package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

type noClientDiscovery struct{}

func (noClientDiscovery) Discover() discovery.Result {
	return discovery.Result{Executables: map[string]string{}}
}

func TestActivationNextActionIsConsistentAcrossPublicReadOnlyCommands(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "team", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secret.NewMemoryStore()
	accounts, err := secret.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name  string
		token string
		want  string
	}{
		{name: "missing Token", want: "run `aigw rotate team`"},
		// The fixture selects Claude without enabling it; sync skips that binding.
		{name: "connected Account with a disabled selection", token: "token", want: "aigw use --for claude claude"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.token != "" {
				if err := secretStore.Set("team", scenario.token); err != nil {
					t.Fatal(err)
				}
			}
			for _, args := range [][]string{{"status", "--json"}, {"sync", "--dry-run", "--json"}, {"check", "--json"}, {"doctor", "--json"}} {
				t.Run(args[0], func(t *testing.T) {
					out := &bytes.Buffer{}
					app := &App{Clients: clients.Registry(), Config: configStore, Secrets: secretStore, Accounts: accounts, Discovery: noClientDiscovery{}, Out: out, Err: io.Discard}
					commandErr := Execute(app, args)
					if (commandErr != nil) != (args[0] == "check") {
						t.Fatalf("%s outcome = %v", args[0], commandErr)
					}
					var result struct {
						NextAction string `json:"next_action"`
						State      string `json:"state"`
					}
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatalf("%s did not return JSON: %v; output=%q", args[0], err, out.String())
					}
					if result.NextAction != scenario.want || result.State != "deferred" {
						t.Fatalf("%s state and next action = %q, %q; want deferred, %q", args[0], result.State, result.NextAction, scenario.want)
					}
					out.Reset()
					humanArgs := append([]string(nil), args[:len(args)-1]...)
					if err := Execute(app, humanArgs); (err != nil) != (args[0] == "check") {
						t.Fatalf("%s human outcome = %v", args[0], err)
					}
					if !strings.Contains(out.String(), scenario.want) {
						t.Fatalf("%s human continuation differs from JSON: %q", args[0], out.String())
					}
				})
			}
		})
	}
}

func TestActivationPrefersUsableRecommendedClientAcrossCommands(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["missing"] = domain.Account{Label: "Missing", Endpoints: domain.Endpoints{Anthropic: "https://missing.test"}}
	cfg.Accounts["connected"] = domain.Account{Label: "Connected", Endpoints: domain.Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "missing", Model: "fable", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["codex"] = domain.Route{Label: "Codex", Account: "connected", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetRecommendedRoute(codex.ID, "codex")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("connected") {
			return "available-token"
		}
		return ""
	})
	accounts, err := secret.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	want := "Install Codex if needed, then run `aigw sync`"
	for _, args := range [][]string{{"status", "--json"}, {"sync", "--dry-run", "--json"}, {"check", "--json"}, {"doctor", "--json"}} {
		t.Run(args[0], func(t *testing.T) {
			out := &bytes.Buffer{}
			app := &App{Clients: clients.Registry(), Config: configStore, Secrets: secretStore, Accounts: accounts, Discovery: noClientDiscovery{}, Out: out, Err: io.Discard}
			commandErr := Execute(app, args)
			if (commandErr != nil) != (args[0] == "check") {
				t.Fatalf("%s outcome = %v", args[0], commandErr)
			}
			var result struct {
				NextAction string `json:"next_action"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("%s did not return JSON: %v; output=%q", args[0], err, out.String())
			}
			if result.NextAction != want {
				t.Fatalf("%s next action = %q, want %q", args[0], result.NextAction, want)
			}
		})
	}
}
