package domain

import (
	"errors"
	"fmt"
)

// RuntimeBindingUnselectedError reports that a client has no selected model.
type RuntimeBindingUnselectedError struct {
	Client string
}

// Error formats a missing client selection without implying invalid configuration.
func (e *RuntimeBindingUnselectedError) Error() string {
	return fmt.Sprintf("no model selected for client %q", e.Client)
}

// UnsupportedConfigVersionError reports a configuration schema newer or older than the one accepted by this build.
type UnsupportedConfigVersionError struct {
	Version         int
	ExpectedVersion int
}

// Error formats the unsupported-version failure without exposing configuration contents.
func (e *UnsupportedConfigVersionError) Error() string {
	return fmt.Sprintf(
		"unsupported config version %d; expected %d; AIGW does not reinterpret configuration schemas; restore a configuration supported by this program or use the matching AIGW release",
		e.Version,
		e.ExpectedVersion,
	)
}

// RuntimeModelUnknownProviderError reports a model whose provider is absent.
type RuntimeModelUnknownProviderError struct {
	Model      ModelRef
	ProviderID string
}

// Error formats the unresolved provider of a model.
func (e *RuntimeModelUnknownProviderError) Error() string {
	return fmt.Sprintf("model %s names the unknown provider %q", e.Model, e.ProviderID)
}

// RuntimeMissingEndpointError reports that an account lacks the protocol endpoint required by its client.
type RuntimeMissingEndpointError struct {
	ProviderID string
	Protocol   EndpointProtocol
}

// Error formats the account and missing endpoint protocol.
func (e *RuntimeMissingEndpointError) Error() string {
	return fmt.Sprintf("provider %q has no %s endpoint", e.ProviderID, ProtocolLabel(e.Protocol))
}

// ProtocolLabel returns the native display name of an endpoint protocol.
func ProtocolLabel(protocol EndpointProtocol) string {
	switch protocol {
	case ProtocolAnthropic:
		return "Anthropic Messages"
	case ProtocolResponses:
		return "OpenAI Responses"
	case ProtocolChat:
		return "OpenAI Chat Completions"
	default:
		return string(protocol)
	}
}

// entryError is a failure at one entry of a configuration or a team file,
// named as the operator finds it there, such as provider `a`, model `a/m` or
// the field `providers.bogus`. It reads as the failure it wraps, so only a
// presentation that tells the operator which entry to correct uses the entry.
type entryError struct {
	Entry string
	Err   error
}

// NewEntryError names entry as the place of err, unless err is nil or names
// an entry already.
func NewEntryError(entry string, err error) error {
	if err == nil {
		return nil
	}
	if _, named := EntryOf(err); named {
		return err
	}
	return &entryError{Entry: entry, Err: err}
}

// Error is the wrapped failure's text.
func (e *entryError) Error() string { return e.Err.Error() }

// Unwrap returns the wrapped failure.
func (e *entryError) Unwrap() error { return e.Err }

// EntryOf returns the entry that err names, and whether it names one.
func EntryOf(err error) (string, bool) {
	entry, named := errors.AsType[*entryError](err)
	if !named {
		return "", false
	}
	return entry.Entry, true
}
