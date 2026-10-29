package codex

import configuration "aigw-cli/internal/configuration"

// otherClient is a second admitted client for the fixtures that show Codex
// ignoring another client's binding.
var otherClient = configuration.ClientSpec{
	ID: "other", Label: "Other", EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic},
}

// testAdmission admits this adapter's own specification and otherClient.
func testAdmission() configuration.Admission {
	return configuration.NewAdmission(New().Spec(), otherClient)
}
