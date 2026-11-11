package domain

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestModelOwnsFamilyIdentityWithoutClientState(t *testing.T) {
	route := Model{Family: "claude-test", ID: "claude-test"}
	if route.Family != "claude-test" {
		t.Fatalf("route = %#v", route)
	}
}

func TestClientSpecResolvesItsDeclaredEndpoint(t *testing.T) {
	account := Provider{ID: "team", Endpoints: Endpoints{
		Anthropic: "https://anthropic.example/v1/",
		Responses: "https://responses.example/v1/",
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
	_, _, err := (ClientSpec{ID: "future", EndpointProtocols: []EndpointProtocol{"future"}}).ResolveEndpoint(Provider{}, "")
	if err == nil || !strings.Contains(err.Error(), "unsupported endpoint protocol") {
		t.Fatalf("unsupported protocol error = %v", err)
	}
}

func TestExplicitCredentialCommandSurvivesCloneAndResolution(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	desktop, hermes := NewModelRef("dmx", "desktop-model"), NewModelRef("backup", "hermes-model")
	cfg.Models[desktop] = testModel("Claude Desktop", "dmx", "desktop-model", ProtocolAnthropic)
	cfg.SetSelectedModel(ClientClaudeDesktop, desktop, "")
	cfg.Models[hermes] = testModel("Hermes", "backup", "hermes-model", ProtocolResponses)
	cfg.SetSelectedModel(ClientHermes, hermes, "")
	for _, client := range testAdmission().IDs() {
		command := filepath.Join(t.TempDir(), "credential adapter")
		cfg.Clients[client] = ClientBinding{Enabled: true, CredentialCommand: command, Model: cfg.SelectedModel(client)}
		runtime, err := cfg.ResolveRuntime(client, ModelRef{})
		if err != nil || runtime.CredentialCommand != command {
			t.Fatalf("explicit %s command = %q, %v", client, runtime.CredentialCommand, err)
		}
		clone := cfg.Clone()
		resolved, err := clone.ResolveRuntime(client, ModelRef{})
		if err != nil || resolved.CredentialCommand != command {
			t.Fatalf("cloned %s = %q, %v", client, resolved.CredentialCommand, err)
		}
	}
}

func TestCredentialCommandRejectsAmbiguousPaths(t *testing.T) {
	for _, command := range []string{"relative-helper", " /absolute/helper", "/absolute/helper\nargument", "/absolute/helper\x00"} {
		cfg := validConfig()
		cfg.Normalize()
		cfg.Clients[ClientCodex] = ClientBinding{Enabled: true, CredentialCommand: command}
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
		if test.runtime.RequiresProviderToken() != test.account || test.runtime.UsesAIGWCredentialStore() != test.native {
			t.Fatalf("credential ownership differs from protocol: %#v", test)
		}
	}
}
