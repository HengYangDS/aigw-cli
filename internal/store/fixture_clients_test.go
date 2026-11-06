package store

import "aigw-cli/internal/domain"

// The built-in clients as this package's test fixtures. The adapters import
// this package for its file transactions, so its own tests cannot import them
// or the composition root; TestFixtureClientsMatchTheBuiltInAdmission, an
// external test, keeps these records equal to the adapters' specifications.
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
			DefaultModelProvider: "aigw", ClientNativeAuthentication: true,
			PublishedProfileProtocol: responses, Doctor: domain.DoctorWording{AdapterStates: true, ModelBinding: true},
		},
		{
			ID: ClientHermes, Label: "Hermes", SessionsKeepModel: true,
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

func testModel(label, account, model string, protocols ...domain.EndpointProtocol) domain.Model {
	interfaces := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []domain.Capability{}
	}
	return domain.Model{Label: label, Provider: account, Family: model, Protocols: interfaces}
}

// validConfig is the two-Account configuration that the store's tests start
// from: Claude selects the DMXAPI Route and Codex the Backup Route.
func validConfig() domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers = map[string]domain.Provider{
		"dmx":    {Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}},
		"backup": {Label: "Backup", Endpoints: domain.Endpoints{OpenAIResponses: "https://backup.test/v1"}},
	}
	cfg.Models = map[string]domain.Model{
		"dmx":    testModel("DMXAPI", "dmx", "claude-test", domain.ProtocolAnthropic),
		"backup": testModel("Backup", "backup", "gpt-test", domain.ProtocolOpenAIResponses),
	}
	cfg.Clients = map[string]domain.ClientBinding{
		ClientClaude: {Model: "dmx"},
		ClientCodex:  {Model: "backup"},
	}
	return cfg
}
