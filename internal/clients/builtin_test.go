package clients

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

type fixedDiscoverer struct{ result discovery.Result }

func (discoverer fixedDiscoverer) Discover() discovery.Result { return discoverer.result }

func qualifiedRoute(label, account, model string, protocols ...configuration.EndpointProtocol) configuration.Route {
	interfaces := make(map[configuration.EndpointProtocol][]configuration.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []configuration.Capability{}
	}
	return configuration.Route{
		Label: label, Account: account, Model: model, UpstreamModel: model, Interfaces: interfaces,
	}
}

func codexConfiguration(target string) configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Route: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func TestChangedClientsRetainsAdmissionOrderAndIndependentResults(t *testing.T) {
	before := codexConfiguration("/target")
	account := before.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	before.Accounts["gateway"] = account
	before.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	after := before.Clone()
	account.Endpoints = configuration.Endpoints{OpenAIResponses: "https://replacement.test/v1", Anthropic: "https://replacement.test"}
	after.Accounts["gateway"] = account
	registry := Registry()
	changed := registry.ChangedClients(before, after)
	want := before.EnabledClientIDs()
	if !slices.Equal(changed, want) {
		t.Fatalf("shared Account change scope = %v, want admission order %v", changed, want)
	}
	changed[0] = "caller edit"
	if got := registry.ChangedClients(before, after); !slices.Equal(got, want) {
		t.Fatalf("caller changed registry scope: %v", got)
	}
}

func TestRegistryPreparesEveryClientBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	codexTarget := filepath.Join(dir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	originalCodex := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(codexTarget, originalCodex, 0o600); err != nil {
		t.Fatal(err)
	}
	claudeSettings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudeSettings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeSettings, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", configuration.ProtocolAnthropic)
	before.Routes["codex"] = qualifiedRoute("Codex", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	before.SetSelectedRoute(configuration.ClientCodex, "codex", "")
	after := before.Clone()
	after.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	after.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{codexTarget})
	deps := client.Dependencies{
		Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID: "codex-home-default", Authority: "aigw", ConfigPath: codexTarget, Present: true, AutoManaged: true,
		}}}},
		ClaudeSettingsPath: claudeSettings,
		AIGWExecutable:     "/opt/aigw",
	}

	_, _, err := Registry().Apply(t.Context(), deps, before, after)
	if err == nil || !strings.Contains(err.Error(), "parse Claude settings") {
		t.Fatalf("Apply() error = %v", err)
	}
	got, readErr := os.ReadFile(codexTarget)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, originalCodex) {
		t.Fatalf("Codex changed before Claude preparation completed:\n%s", got)
	}
	for _, path := range []string{codexTarget + ".aigw-state.json", filepath.Join(filepath.Dir(codexTarget), "models.json")} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("Codex artifact %s exists after preparation failure: %v", path, statErr)
		}
	}
}
