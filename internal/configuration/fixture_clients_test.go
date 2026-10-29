package configuration

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// The built-in clients by their adapters' identifiers. Neither the composition
// root nor an adapter imports this package, so its tests admit the composition
// root's real client records instead of copies that could drift from them.
const (
	ClientClaude        = claude.ID
	ClientClaudeDesktop = claudedesktop.ID
	ClientCodex         = codex.ID
	ClientHermes        = hermes.ID
)

// testClientSpecs returns the built-in client records in admission order.
func testClientSpecs() []domain.ClientSpec { return clients.Admission().Specs() }

// testAdmission admits the built-in clients, as the program does.
func testAdmission() domain.Admission { return clients.Admission() }

// admitted governs a configuration literal by the built-in admission, which a
// literal of another package cannot set.
func admitted(cfg domain.Config) domain.Config {
	cfg.SetAdmission(testAdmission())
	return cfg
}

func testRoute(label, account, model string, protocols ...domain.EndpointProtocol) domain.Route {
	interfaces := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []domain.Capability{}
	}
	return domain.Route{Label: label, Account: account, Model: model, Interfaces: interfaces}
}

// validConfig is the two-Account configuration that the store's tests start
// from: Claude selects the DMXAPI Route and Codex the Backup Route.
func validConfig() domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts = map[string]domain.Account{
		"dmx":    {Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}},
		"backup": {Label: "Backup", Endpoints: domain.Endpoints{OpenAIResponses: "https://backup.test/v1"}},
	}
	cfg.Routes = map[string]domain.Route{
		"dmx":    testRoute("DMXAPI", "dmx", "claude-test", domain.ProtocolAnthropic),
		"backup": testRoute("Backup", "backup", "gpt-test", domain.ProtocolOpenAIResponses),
	}
	cfg.Clients = map[string]domain.ClientBinding{
		ClientClaude: {Route: "dmx"},
		ClientCodex:  {Route: "backup"},
	}
	return cfg
}
