package configuration

import "aigw-cli/internal/domain"

// The built-in clients as this package's test fixtures. The store names no
// client, and its tests cannot import the adapters that own these records,
// because the adapters import the store; the composition root's tests pin the
// real records.
const (
	ClientClaude        = "claude"
	ClientClaudeDesktop = "claude-desktop"
	ClientCodex         = "codex"
	ClientHermes        = "hermes"
)

// testClientSpecs returns the built-in client records that this package's
// tests admit, in admission order.
func testClientSpecs() []domain.ClientSpec {
	anthropic, responses := domain.ProtocolAnthropic, domain.ProtocolOpenAIResponses
	return []domain.ClientSpec{
		{
			ID: ClientClaude, Label: "Claude", EndpointProtocols: []domain.EndpointProtocol{anthropic},
			ProductName: "Claude Code", ModelDiscoveryOptional: true, Doctor: domain.DoctorWording{AdapterStates: true},
			PublishedProfileProtocol: anthropic,
		},
		{
			ID: ClientClaudeDesktop, Label: "Claude Desktop", EndpointProtocols: []domain.EndpointProtocol{anthropic},
			RestartAfterProjection: true, QualifiedModes: []string{"Chat", "Cowork", "Code"}, QualifiedPlatforms: []string{"macOS"},
		},
		{
			ID: ClientCodex, Label: "Codex", EndpointProtocols: []domain.EndpointProtocol{responses},
			TargetFile: "config.toml", TargetsUsage: "repeat for multiple Codex homes", RetryBudgets: true,
			DefaultModelProvider: domain.ModelProviderAIGW, ClientNativeAuthentication: true,
			PublishedProfileProtocol: responses, Doctor: domain.DoctorWording{AdapterStates: true, RouteBinding: true},
		},
		{
			ID: ClientHermes, Label: "Hermes", SessionsKeepRoute: true,
			EndpointProtocols: []domain.EndpointProtocol{responses, anthropic, domain.ProtocolOpenAIChatCompletions},
		},
	}
}

// testAdmission admits the fixture built-in clients for this package's tests.
func testAdmission() domain.Admission { return domain.NewAdmission(testClientSpecs()...) }

// admitted governs a configuration literal by the fixture admission, which a
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
