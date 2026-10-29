package claude

import configuration "aigw-cli/internal/configuration"

// otherClient is a second admitted client for the fixtures that show Claude
// Code ignoring another client's binding.
var otherClient = configuration.ClientSpec{
	ID: "other", Label: "Other", EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolOpenAIResponses},
}

// testAdmission admits this adapter's own specification and otherClient.
func testAdmission() configuration.Admission {
	return configuration.NewAdmission(New().Spec(), otherClient)
}
