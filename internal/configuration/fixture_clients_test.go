package configuration

// The built-in clients as this package's test fixtures. Configuration names no
// client, and its tests cannot import the adapters that own these records,
// because the adapters import configuration; the composition root's tests pin
// the real records.
const (
	ClientClaude        = "claude"
	ClientClaudeDesktop = "claude-desktop"
	ClientCodex         = "codex"
	ClientHermes        = "hermes"
)

// testClientSpecs returns the built-in client records that this package's
// tests admit, in admission order.
func testClientSpecs() []ClientSpec {
	return []ClientSpec{
		{
			ID: ClientClaude, Label: "Claude", EndpointProtocols: []EndpointProtocol{ProtocolAnthropic},
			ProductName: "Claude Code", ModelDiscoveryOptional: true, Doctor: DoctorWording{AdapterStates: true},
			PublishedProfileProtocol: ProtocolAnthropic,
		},
		{
			ID: ClientClaudeDesktop, Label: "Claude Desktop", EndpointProtocols: []EndpointProtocol{ProtocolAnthropic},
			RestartAfterProjection: true, QualifiedModes: []string{"Chat", "Cowork", "Code"}, QualifiedPlatforms: []string{"macOS"},
		},
		{
			ID: ClientCodex, Label: "Codex", EndpointProtocols: []EndpointProtocol{ProtocolOpenAIResponses},
			TargetFile: "config.toml", TargetsUsage: "repeat for multiple Codex homes", RetryBudgets: true,
			DefaultModelProvider: ModelProviderAIGW, ClientNativeAuthentication: true,
			PublishedProfileProtocol: ProtocolOpenAIResponses, Doctor: DoctorWording{AdapterStates: true, RouteBinding: true},
		},
		{
			ID: ClientHermes, Label: "Hermes", SessionsKeepRoute: true,
			EndpointProtocols: []EndpointProtocol{ProtocolOpenAIResponses, ProtocolAnthropic, ProtocolOpenAIChatCompletions},
		},
	}
}
