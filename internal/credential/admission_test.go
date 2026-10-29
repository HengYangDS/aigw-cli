package credential

import "aigw-cli/internal/domain"

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
func testAdmission() domain.Admission {
	anthropic, responses := domain.ProtocolAnthropic, domain.ProtocolOpenAIResponses
	return domain.NewAdmission(
		domain.ClientSpec{ID: clientClaude, Label: "Claude", EndpointProtocols: []domain.EndpointProtocol{anthropic}},
		domain.ClientSpec{ID: clientClaudeDesktop, Label: "Claude Desktop", EndpointProtocols: []domain.EndpointProtocol{anthropic}},
		domain.ClientSpec{ID: clientCodex, Label: "Codex", EndpointProtocols: []domain.EndpointProtocol{responses}},
		domain.ClientSpec{
			ID: clientHermes, Label: "Hermes",
			EndpointProtocols: []domain.EndpointProtocol{responses, anthropic, domain.ProtocolOpenAIChatCompletions},
		},
	)
}
