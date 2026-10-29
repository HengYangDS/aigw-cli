package configuration

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestRouteOwnsModelIdentityWithoutClientState(t *testing.T) {
	route := Route{Model: "claude-test"}
	if route.Model != "claude-test" {
		t.Fatalf("route = %#v", route)
	}
}

func TestClientSpecResolvesItsDeclaredEndpoint(t *testing.T) {
	account := Account{ID: "team", Endpoints: Endpoints{
		Anthropic:       "https://anthropic.example/v1/",
		OpenAIResponses: "https://responses.example/v1/",
	}}
	tests := []struct {
		client string
		want   string
	}{
		{client: ClientClaude, want: "https://anthropic.example/v1"},
		{client: ClientCodex, want: "https://responses.example/v1"},
	}
	for _, test := range tests {
		t.Run(test.client, func(t *testing.T) {
			spec, ok := testAdmission().Spec(test.client)
			if !ok {
				t.Fatalf("missing client spec %q", test.client)
			}
			got, _, err := spec.ResolveEndpoint(account, "")
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("endpoint = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAdmissionUsageNamesTheAdmittedClientsInOrder(t *testing.T) {
	if got := naturalChoices(nil); got != "" {
		t.Fatalf("empty choices = %q", got)
	}
	if got := naturalChoices([]string{"codex"}); got != "codex" {
		t.Fatalf("single choice = %q", got)
	}
	if got := testAdmission().Usage(); got != "claude, claude-desktop, codex, or hermes" {
		t.Fatalf("usage = %q", got)
	}
	if got := testAdmission().Usage("all"); got != "claude, claude-desktop, codex, hermes, or all" {
		t.Fatalf("usage with extra choice = %q", got)
	}
}

func TestAdmissionLabelsEachClientByItsSpecification(t *testing.T) {
	for id, want := range map[string]string{
		ClientClaude:        "Claude",
		ClientClaudeDesktop: "Claude Desktop",
		ClientCodex:         "Codex",
		ClientHermes:        "Hermes",
		"future":            "future",
	} {
		if got := testAdmission().Label(id); got != want {
			t.Fatalf("Label(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestClientSpecRejectsUnimplementedProtocol(t *testing.T) {
	_, _, err := (ClientSpec{ID: "future", EndpointProtocols: []EndpointProtocol{"future"}}).ResolveEndpoint(Account{}, "")
	if err == nil || !strings.Contains(err.Error(), "unsupported endpoint protocol") {
		t.Fatalf("unsupported protocol error = %v", err)
	}
}

func TestExplicitCredentialCommandSurvivesHostConfigRoundTrip(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	cfg.Routes[ClientClaudeDesktop] = testRoute("Claude Desktop", "dmx", "model", ProtocolAnthropic)
	cfg.SetSelectedRoute(ClientClaudeDesktop, ClientClaudeDesktop, "")
	cfg.Routes[ClientHermes] = testRoute("Hermes", "backup", "model", ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ClientHermes, ClientHermes, "")
	for _, client := range testAdmission().IDs() {
		command := filepath.Join(t.TempDir(), "credential adapter")
		var adapter ClientBinding
		if err := toml.Unmarshal([]byte(fmt.Sprintf("enabled = true\ncredential_command = %q\n", command)), &adapter); err != nil {
			t.Fatal(err)
		}
		adapter.Route = cfg.SelectedRoute(client)
		cfg.Clients[client] = adapter
		runtime, err := cfg.ResolveRuntime(client, "")
		if err != nil || runtime.CredentialCommand != command {
			t.Fatalf("explicit %s command = %q, %v", client, runtime.CredentialCommand, err)
		}
		encoded, err := toml.Marshal(cfg.Clone())
		if err != nil {
			t.Fatal(err)
		}
		loaded := Config{admission: testAdmission()}
		if err := toml.Unmarshal(encoded, &loaded); err != nil {
			t.Fatal(err)
		}
		resolved, err := loaded.ResolveRuntime(client, "")
		if err != nil || resolved.CredentialCommand != command {
			t.Fatalf("roundtrip %s = %q, %v", client, resolved.CredentialCommand, err)
		}
	}
}

func TestCredentialCommandRejectsAmbiguousPaths(t *testing.T) {
	for _, command := range []string{"relative-helper", " /absolute/helper", "/absolute/helper\nargument", "/absolute/helper\x00"} {
		cfg := validConfig()
		cfg.Normalize()
		var adapter ClientBinding
		encoded, err := toml.Marshal(map[string]any{"enabled": true, "credential_command": command})
		if err != nil {
			t.Fatal(err)
		}
		if err := toml.Unmarshal(encoded, &adapter); err != nil {
			t.Fatal(err)
		}
		cfg.Clients[ClientCodex] = adapter
		if err := cfg.Validate(); err == nil {
			t.Errorf("accepted ambiguous command %q", command)
		}
	}
}

func TestCredentialRetrievalDoesNotChangeWireAuthentication(t *testing.T) {
	for _, test := range []struct {
		runtime Runtime
		account bool
		native  bool
	}{
		{runtime: Runtime{}, account: true, native: true},
		{runtime: Runtime{CredentialCommand: filepath.Join(t.TempDir(), "helper")}, account: true},
		{runtime: Runtime{Authentication: AuthenticationClientNative}},
	} {
		if test.runtime.RequiresAccountToken() != test.account || test.runtime.UsesAIGWCredentialStore() != test.native {
			t.Fatalf("credential ownership differs from protocol: %#v", test)
		}
	}
}
