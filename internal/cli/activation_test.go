package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

type noClientDiscovery struct{}

func (noClientDiscovery) Discover() discovery.Result {
	return discovery.Result{Executables: map[string]string{}}
}

func TestActivationNextActionIsConsistentAcrossPublicReadOnlyCommands(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = configuration.Route{Label: "Claude", Account: "team", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	accounts, err := secrets.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name  string
		token string
		want  string
	}{
		{name: "missing Token", want: "run `aigw rotate team`"},
		{name: "connected Account without client", token: "token", want: "Install Claude if needed, then run `aigw sync`"},
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
					app := &App{Config: store, Secrets: secretStore, Accounts: accounts, Discovery: noClientDiscovery{}, Out: out, Err: io.Discard}
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
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["missing"] = configuration.Account{Label: "Missing", Endpoints: configuration.Endpoints{Anthropic: "https://missing.test"}}
	cfg.Accounts["connected"] = configuration.Account{Label: "Connected", Endpoints: configuration.Endpoints{OpenAIResponses: "https://connected.test/v1"}}
	cfg.Routes["claude"] = configuration.Route{Label: "Claude", Account: "missing", Model: "fable", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.Routes["codex"] = configuration.Route{Label: "Codex", Account: "connected", Model: "gpt", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetRecommendedRoute(configuration.ClientCodex, "codex")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("connected") {
			return "available-token"
		}
		return ""
	})
	accounts, err := secrets.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	want := "Install Codex if needed, then run `aigw sync`"
	for _, args := range [][]string{{"status", "--json"}, {"sync", "--dry-run", "--json"}, {"check", "--json"}, {"doctor", "--json"}} {
		t.Run(args[0], func(t *testing.T) {
			out := &bytes.Buffer{}
			app := &App{Config: store, Secrets: secretStore, Accounts: accounts, Discovery: noClientDiscovery{}, Out: out, Err: io.Discard}
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
