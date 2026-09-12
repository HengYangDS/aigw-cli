package configuration

import "fmt"

// UnsupportedConfigVersionError reports a configuration schema newer or older than the one accepted by this build.
type UnsupportedConfigVersionError struct {
	Version         int
	ExpectedVersion int
}

// Error formats the unsupported-version failure without exposing configuration contents.
func (e *UnsupportedConfigVersionError) Error() string {
	return fmt.Sprintf("unsupported config version %d; expected %d", e.Version, e.ExpectedVersion)
}

// RuntimeProfileClientMismatchError reports a profile selected for a different client.
type RuntimeProfileClientMismatchError struct {
	ProfileID      string
	ExpectedClient string
	ActualClient   string
}

// Error formats the expected and actual client identities for a mismatched profile.
func (e *RuntimeProfileClientMismatchError) Error() string {
	return fmt.Sprintf("profile %q is for %s, not %s", e.ProfileID, e.ExpectedClient, e.ActualClient)
}

// RuntimeProfileUnknownAccountError reports a profile whose referenced account is absent.
type RuntimeProfileUnknownAccountError struct {
	ProfileID string
	AccountID string
}

// Error formats the unresolved account reference for a profile.
func (e *RuntimeProfileUnknownAccountError) Error() string {
	return fmt.Sprintf("profile %q references unknown account %q", e.ProfileID, e.AccountID)
}

// RuntimeMissingEndpointError reports that an account lacks the protocol endpoint required by its client.
type RuntimeMissingEndpointError struct {
	AccountID string
	Protocol  EndpointProtocol
}

// Error formats the account and missing endpoint protocol.
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
