package codex

import "aigw-cli/internal/domain"

// otherClient is a second admitted client for the fixtures that show Codex
// ignoring another client's binding.
var otherClient = domain.ClientSpec{
	ID: "other", Label: "Other", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolAnthropic},
}

// testAdmission admits this adapter's own specification and otherClient.
func testAdmission() domain.Admission {
	return domain.NewAdmission(New().Spec(), otherClient)
}
