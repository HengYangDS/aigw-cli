package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CredentialView is the part of the configuration that the credential reader
// reads: each provider's ID and endpoints, the protocols that its models
// declare, and each client's enabled state and authentication. Any other
// field can change in a later release without a reader noticing, so that a
// session keeps its Token across upgrades; changing one of these takes a new
// file name.
type CredentialView struct {
	Providers []CredentialProvider
	Clients   map[string]CredentialClient
}

// CredentialProvider is one provider as the reader sees it. Each entry of
// Models is the protocols that one of its models declares, among those that
// the reader knows.
type CredentialProvider struct {
	ID        string
	Endpoints Endpoints
	Models    [][]EndpointProtocol
}

// CredentialClient is one client as the reader sees it.
type CredentialClient struct {
	Enabled        bool
	Authentication Authentication
}

var (
	// ErrCredentialClientDisabled refuses a client that is not enabled.
	ErrCredentialClientDisabled = errors.New("the client is not enabled")
	// ErrCredentialClientOwned refuses a client that authenticates itself,
	// for which AIGW supplies no Token.
	ErrCredentialClientOwned = errors.New("the client uses its own authentication")
	// errCredentialUnmatched refuses a fingerprint that no endpoint of a
	// provider's model matches.
	errCredentialUnmatched = errors.New("no provider endpoint matches the fingerprint")
)

// UnknownAuthenticationError refuses a client whose authentication this
// reader does not know, because it cannot tell whether a Token is wanted.
type UnknownAuthenticationError struct{ Authentication Authentication }

// Error names the authentication.
func (e *UnknownAuthenticationError) Error() string {
	return fmt.Sprintf("the client's authentication %q is unknown to this reader", e.Authentication)
}

// TokenProvider returns the provider whose Token the credential command of
// the client that spec admits asks for with fingerprint: the provider whose
// endpoint, on a protocol that the client speaks and one of the provider's
// models declares, has that fingerprint, which a client's runtime computes
// with CredentialProjectionFingerprint. It checks no selection, so a running
// session keeps its provider's Token while the provider and the endpoint
// exist. A disabled client, a client that authenticates itself and one whose
// authentication is unknown are refused before any fingerprint is compared.
func (view CredentialView) TokenProvider(spec ClientSpec, fingerprint string) (string, error) {
	client := view.Clients[spec.ID]
	if !client.Enabled {
		return "", ErrCredentialClientDisabled
	}
	switch client.Authentication {
	case "", AuthenticationProviderToken:
	case AuthenticationClientNative:
		return "", ErrCredentialClientOwned
	default:
		return "", &UnknownAuthenticationError{Authentication: client.Authentication}
	}
	for _, provider := range view.Providers {
		for _, protocol := range spec.EndpointProtocols {
			endpoint := provider.Endpoints.For(protocol)
			declared := slices.ContainsFunc(provider.Models, func(protocols []EndpointProtocol) bool { return slices.Contains(protocols, protocol) })
			if endpoint != "" && declared && upstreamIdentity(spec.ID, provider.ID, strings.TrimRight(endpoint, "/")) == fingerprint {
				return provider.ID, nil
			}
		}
	}
	return "", errCredentialUnmatched
}
