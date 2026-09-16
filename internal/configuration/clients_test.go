package configuration

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestAdmittedClientRegistryIsTheSingleProtocolBoundary(t *testing.T) {
	want := []ClientSpec{
		{ID: ClientClaude, Label: "Claude", EndpointProtocol: ProtocolAnthropic},
		{ID: ClientCodex, Label: "Codex", EndpointProtocol: ProtocolOpenAIResponses},
	}
	if got := AdmittedClientSpecs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("client specs = %#v, want %#v", got, want)
	}
	if got := AdmittedClientIDs(); !reflect.DeepEqual(got, []string{ClientClaude, ClientCodex}) {
		t.Fatalf("client ids = %#v", got)
	}
	if !IsAdmittedClient(ClientClaude) || IsAdmittedClient("gemini") {
		t.Fatal("registry must admit only implemented adapters")
	}
	if spec, ok := ClientSpecFor(ClientCodex); !ok || spec.EndpointProtocol != ProtocolOpenAIResponses {
		t.Fatalf("Codex client spec = %#v, %v", spec, ok)
	}
}

func TestAdmittedClientRegistryReturnsDefensiveCopies(t *testing.T) {
	clients := AdmittedClientSpecs()
	clients[0].ID = "mutated"
	if AdmittedClientSpecs()[0].ID != ClientClaude {
		t.Fatal("caller mutation changed the registered client boundary")
	}
}

func TestProfileOwnsOneClientAndOneModel(t *testing.T) {
	profile := Profile{Client: ClientClaude, Model: "claude-test"}
	if profile.Client != ClientClaude || profile.Model != "claude-test" {
		t.Fatalf("profile = %#v", profile)
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
			spec, ok := ClientSpecFor(test.client)
			if !ok {
				t.Fatalf("missing client spec %q", test.client)
			}
			got, err := spec.Endpoint(account)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("endpoint = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAdmittedClientUsageIsDerivedFromRegistry(t *testing.T) {
	if got := naturalChoices(nil); got != "" {
		t.Fatalf("empty choices = %q", got)
	}
	if got := naturalChoices([]string{"codex"}); got != "codex" {
		t.Fatalf("single choice = %q", got)
	}
	if got := AdmittedClientUsage(); got != "claude or codex" {
		t.Fatalf("usage = %q", got)
	}
	if got := AdmittedClientUsage("all"); got != "claude, codex, or all" {
		t.Fatalf("usage with extra choice = %q", got)
	}
	if got := AdmittedClientLabelUsage("all"); got != "Claude, Codex, or all" {
		t.Fatalf("label usage = %q", got)
	}
}

func TestClientSpecRejectsUnimplementedProtocol(t *testing.T) {
	_, err := (ClientSpec{ID: "future", EndpointProtocol: "future"}).Endpoint(Account{})
	if err == nil || !strings.Contains(err.Error(), "unsupported endpoint protocol") {
		t.Fatalf("unsupported protocol error = %v", err)
	}
}

func TestExplicitCredentialCommandSurvivesHostConfigRoundTrip(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	for _, client := range AdmittedClientIDs() {
		command := filepath.Join(t.TempDir(), "credential adapter")
		var adapter AdapterConfig
		if err := toml.Unmarshal([]byte(fmt.Sprintf("enabled = true\ncredential_command = %q\n", command)), &adapter); err != nil {
			t.Fatal(err)
		}
		cfg.Adapters[client] = adapter
		runtime, err := cfg.ResolveRuntime(client, "")
		if err != nil || runtime.CredentialCommand != command {
			t.Fatalf("explicit %s command = %q, %v", client, runtime.CredentialCommand, err)
		}
		encoded, err := toml.Marshal(cfg.Clone())
		if err != nil {
			t.Fatal(err)
		}
		var loaded Config
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
		var adapter AdapterConfig
		encoded, err := toml.Marshal(map[string]any{"enabled": true, "credential_command": command})
		if err != nil {
			t.Fatal(err)
		}
		if err := toml.Unmarshal(encoded, &adapter); err != nil {
			t.Fatal(err)
		}
		cfg.Adapters[ClientCodex] = adapter
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
