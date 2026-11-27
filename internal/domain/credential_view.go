package domain

import (
	"errors"
	"strings"
)

// CredentialView is the part of the configuration that the credential reader
// reads: each provider's ID and endpoints, and each client's enabled state.
// Any other field can change in a later release without a reader noticing,
// so that a session keeps its Token across upgrades; changing one of these
// takes a new file name. The view once also held each client's
// `authentication`; removing that field kept the name, because every reader
// since 0.7.0 serves a client without it with its provider's Token.
type CredentialView struct {
	Providers []CredentialProvider
	Clients   map[string]CredentialClient
}

// CredentialProvider is one provider as the reader sees it.
type CredentialProvider struct {
	ID        string
	Endpoints Endpoints
}

// CredentialClient is one client as the reader sees it.
type CredentialClient struct {
	Enabled bool
}

var (
	// ErrCredentialClientDisabled refuses a client that is not enabled.
	ErrCredentialClientDisabled = errors.New("the client is not enabled")
	// errCredentialUnmatched refuses a fingerprint that no provider's
	// endpoint matches.
	errCredentialUnmatched = errors.New("no provider endpoint matches the fingerprint")
)

// TokenProvider returns the provider whose Token the credential command of
// the client that spec admits asks for with fingerprint: the provider whose
// endpoint, on a protocol that the client speaks, has that fingerprint, which
// a client's runtime computes with CredentialProjectionFingerprint. It checks
// no selection and no model, so a running session keeps its provider's Token
// while the provider and the endpoint exist. A disabled client is refused
// before any fingerprint is compared.
func (view CredentialView) TokenProvider(spec ClientSpec, fingerprint string) (string, error) {
	if !view.Clients[spec.ID].Enabled {
		return "", ErrCredentialClientDisabled
	}
	for _, provider := range view.Providers {
		for _, protocol := range spec.EndpointProtocols {
			endpoint := provider.Endpoints.For(protocol)
			if endpoint != "" && upstreamIdentity(spec.ID, provider.ID, strings.TrimRight(endpoint, "/")) == fingerprint {
				return provider.ID, nil
			}
		}
	}
	return "", errCredentialUnmatched
}
