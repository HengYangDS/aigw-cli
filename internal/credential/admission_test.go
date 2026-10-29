package credential

import configuration "aigw-cli/internal/configuration"

// The built-in clients as fixtures: this package's tests cannot import the
// adapters that own their records, because the adapters import this package.
const (
	clientClaude        = "claude"
	clientClaudeDesktop = "claude-desktop"
	clientCodex         = "codex"
	clientHermes        = "hermes"
)

// testAdmission admits fixture records of the built-in clients with the fields
// that credential validation reads: identity, label and endpoint protocols.
func testAdmission() configuration.Admission {
	anthropic, responses := configuration.ProtocolAnthropic, configuration.ProtocolOpenAIResponses
	return configuration.NewAdmission(
		configuration.ClientSpec{ID: clientClaude, Label: "Claude", EndpointProtocols: []configuration.EndpointProtocol{anthropic}},
		configuration.ClientSpec{ID: clientClaudeDesktop, Label: "Claude Desktop", EndpointProtocols: []configuration.EndpointProtocol{anthropic}},
		configuration.ClientSpec{ID: clientCodex, Label: "Codex", EndpointProtocols: []configuration.EndpointProtocol{responses}},
		configuration.ClientSpec{
			ID: clientHermes, Label: "Hermes",
			EndpointProtocols: []configuration.EndpointProtocol{responses, anthropic, configuration.ProtocolOpenAIChatCompletions},
		},
	)
}
