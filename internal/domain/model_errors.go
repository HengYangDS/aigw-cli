package domain

import "fmt"

// RuntimeBindingUnselectedError reports that a client has no selected Route.
type RuntimeBindingUnselectedError struct {
	Client string
}

// Error formats a missing client selection without implying invalid configuration.
func (e *RuntimeBindingUnselectedError) Error() string {
	return fmt.Sprintf("no Route selected for client %q", e.Client)
}

// UnsupportedConfigVersionError reports a configuration schema newer or older than the one accepted by this build.
type UnsupportedConfigVersionError struct {
	Version         int
	ExpectedVersion int
}

// Error formats the unsupported-version failure without exposing configuration contents.
func (e *UnsupportedConfigVersionError) Error() string {
	action := "restore a configuration supported by this program or use the matching AIGW release"
	if (e.Version == LegacyConfigVersion || e.Version == PublishedConfigVersion) && e.ExpectedVersion == ConfigVersion {
		action = "run `aigw config migrate --dry-run`, review the replacement, then run `aigw config migrate`"
	}
	return fmt.Sprintf(
		"unsupported config version %d; expected %d; AIGW does not reinterpret configuration schemas; %s",
		e.Version,
		e.ExpectedVersion,
		action,
	)
}

// RuntimeModelClientMismatchError reports a route selected for a different client.
type RuntimeModelClientMismatchError struct {
	ModelID        string
	ExpectedClient string
	ActualClient   string
}

// Error formats the expected and actual client identities for a mismatched route.
func (e *RuntimeModelClientMismatchError) Error() string {
	return fmt.Sprintf("route %q is for %s, not %s", e.ModelID, e.ExpectedClient, e.ActualClient)
}

// RuntimeModelUnknownProviderError reports a route whose referenced account is absent.
type RuntimeModelUnknownProviderError struct {
	ModelID    string
	ProviderID string
}

// Error formats the unresolved account reference for a route.
func (e *RuntimeModelUnknownProviderError) Error() string {
	return fmt.Sprintf("route %q references unknown account %q", e.ModelID, e.ProviderID)
}

// RuntimeMissingEndpointError reports that an account lacks the protocol endpoint required by its client.
type RuntimeMissingEndpointError struct {
	ProviderID string
	Protocol   EndpointProtocol
}

// Error formats the account and missing endpoint protocol.
func (e *RuntimeMissingEndpointError) Error() string {
	return fmt.Sprintf("account %q has no %s endpoint", e.ProviderID, ProtocolLabel(e.Protocol))
}

// ProtocolLabel returns the native display name of an endpoint protocol.
func ProtocolLabel(protocol EndpointProtocol) string {
	switch protocol {
	case ProtocolAnthropic:
		return "Anthropic Messages"
	case ProtocolOpenAIResponses:
		return "OpenAI Responses"
	case ProtocolOpenAIChatCompletions:
		return "OpenAI Chat Completions"
	default:
		return string(protocol)
	}
}
