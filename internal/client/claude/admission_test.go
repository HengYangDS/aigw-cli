package claude

import "aigw-cli/internal/domain"

// otherClient is a second admitted client for the fixtures that show Claude
// Code ignoring another client's binding.
var otherClient = domain.ClientSpec{
	ID: "other", Label: "Other", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses},
}

// testAdmission admits this adapter's own specification and otherClient.
func testAdmission() domain.Admission {
	return domain.NewAdmission(New().Spec(), otherClient)
}
