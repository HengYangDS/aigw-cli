package configuration

import "fmt"

type UnsupportedConfigVersionError struct {
	Version         int
	ExpectedVersion int
}

func (e *UnsupportedConfigVersionError) Error() string {
	return fmt.Sprintf("unsupported config version %d; expected %d", e.Version, e.ExpectedVersion)
}

type RuntimeProfileClientMismatchError struct {
	ProfileID      string
	ExpectedClient string
	ActualClient   string
}

func (e *RuntimeProfileClientMismatchError) Error() string {
	return fmt.Sprintf("profile %q is for %s, not %s", e.ProfileID, e.ExpectedClient, e.ActualClient)
}

type RuntimeProfileUnknownAccountError struct {
	ProfileID string
	AccountID string
}

func (e *RuntimeProfileUnknownAccountError) Error() string {
	return fmt.Sprintf("profile %q references unknown account %q", e.ProfileID, e.AccountID)
}

type RuntimeMissingEndpointError struct {
	AccountID string
	Protocol  EndpointProtocol
}

func (e *RuntimeMissingEndpointError) Error() string {
	return fmt.Sprintf("account %q has no %s endpoint", e.AccountID, endpointProtocolName(e.Protocol))
}

func endpointProtocolName(protocol EndpointProtocol) string {
	switch protocol {
	case ProtocolAnthropic:
		return "Anthropic"
	case ProtocolOpenAIResponses:
		return "OpenAI Responses"
	default:
		return string(protocol)
	}
}
