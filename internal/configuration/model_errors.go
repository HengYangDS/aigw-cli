package configuration

import "fmt"

// RuntimeBindingUnselectedError reports that a client has no selected Profile.
type RuntimeBindingUnselectedError struct {
	Client string
}

// Error formats a missing client selection without implying invalid configuration.
func (e *RuntimeBindingUnselectedError) Error() string {
	return fmt.Sprintf("no Profile selected for client %q", e.Client)
}

// UnsupportedConfigVersionError reports a configuration schema newer or older than the one accepted by this build.
type UnsupportedConfigVersionError struct {
	Version         int
	ExpectedVersion int
}

// Error formats the unsupported-version failure without exposing configuration contents.
func (e *UnsupportedConfigVersionError) Error() string {
	action := "restore a configuration supported by this program or use the matching AIGW release"
	if e.Version == LegacyConfigVersion && e.ExpectedVersion == ConfigVersion {
		action = "run `aigw config migrate --dry-run`, review the replacement, then run `aigw config migrate`"
	}
	return fmt.Sprintf(
		"unsupported config version %d; expected %d; AIGW does not reinterpret configuration schemas; %s",
		e.Version,
		e.ExpectedVersion,
		action,
	)
}

// RuntimeRouteClientMismatchError reports a profile selected for a different client.
type RuntimeRouteClientMismatchError struct {
	RouteID        string
	ExpectedClient string
	ActualClient   string
}

// Error formats the expected and actual client identities for a mismatched profile.
func (e *RuntimeRouteClientMismatchError) Error() string {
	return fmt.Sprintf("profile %q is for %s, not %s", e.RouteID, e.ExpectedClient, e.ActualClient)
}

// RuntimeRouteUnknownAccountError reports a profile whose referenced account is absent.
type RuntimeRouteUnknownAccountError struct {
	RouteID   string
	AccountID string
}

// Error formats the unresolved account reference for a profile.
func (e *RuntimeRouteUnknownAccountError) Error() string {
	return fmt.Sprintf("profile %q references unknown account %q", e.RouteID, e.AccountID)
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
	case ProtocolOpenAIChatCompletions:
		return "OpenAI Chat Completions"
	default:
		return string(protocol)
	}
}
