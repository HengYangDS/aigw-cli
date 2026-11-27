package domain

// The built-in clients as this package's test fixtures. Domain names no client,
// and its tests cannot import the adapters that own these records, because the
// adapters import domain; TestFixtureClientsMatchTheBuiltInAdmission, an
// external test, keeps these records equal to the adapters' specifications.
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
			ProductName: "Claude Code", ModelDiscoveryOptional: true,
			Extension: ClientExtension{ID: "anthropic.claude-code", Program: "resources/native-binary/claude"},
		},
		{
			ID: ClientClaudeDesktop, Label: "Claude Desktop", EndpointProtocols: []EndpointProtocol{ProtocolAnthropic},
			RestartAfterProjection: true, QualifiedModes: []string{"Chat", "Cowork", "Code"}, QualifiedPlatforms: []string{"macOS"},
			OptionalTarget: "configLibrary", InAppAcceptance: true,
		},
		{
			ID: ClientCodex, Label: "Codex", EndpointProtocols: []EndpointProtocol{ProtocolResponses},
			TargetFile: "config.toml", TargetsUsage: "repeat for multiple Codex homes", RetryBudgets: true, OperatorCreatesTargets: true,
			ContextWindow: true,
			Extension: ClientExtension{
				ID: "openai.chatgpt", Program: "bin/{os}-{arch}/codex",
				OS: map[string]string{"darwin": "macos", "linux": "linux"}, Arch: map[string]string{"arm64": "aarch64", "amd64": "x86_64"},
			},
		},
		{
			ID: ClientHermes, Label: "Hermes", SessionsKeepModel: true, OptionalTarget: "config.yaml", ContextWindow: true,
			EndpointProtocols: []EndpointProtocol{ProtocolResponses, ProtocolAnthropic, ProtocolChat},
		},
	}
}
