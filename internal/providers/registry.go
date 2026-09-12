// Package providers contains explicitly bundled, provider-native diagnostics.
// It is intentionally outside AIGW's Account/Profile/Route/Adapter core: an
// Account may declare a diagnostic provider that is not present in a given
// build while all ordinary routing and health checks continue to work.
package providers

import (
	"context"
	"fmt"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/providers/diagnostic"
	"aigw-cli/internal/providers/dmxapi"
	"aigw-cli/internal/secrets"
)

type probeFunc func(context.Context, credential.HTTPDoer, configuration.Account, string, secrets.DiagnosticCredential) (diagnostic.Report, error)

var registry = map[string]probeFunc{
	dmxapi.Kind: dmxapi.Probe,
}

// Supports reports whether this AIGW build has an explicit native diagnostic
// integration for kind. Unknown kinds remain valid manifest data; only their
// exact provider-native diagnostic command is unavailable.
func Supports(kind string) bool {
	_, ok := registry[kind]
	return ok
}

// Probe dispatches an optional account diagnostic to the provider adapter declared by the account.
func Probe(ctx context.Context, client credential.HTTPDoer, providerAccount configuration.Account, apiToken string, auth secrets.DiagnosticCredential) (diagnostic.Report, error) {
	if providerAccount.AccountProbe == nil {
		return diagnostic.Report{}, fmt.Errorf("account %q has no exact diagnostic provider", providerAccount.ID)
	}
	implementation, ok := registry[providerAccount.AccountProbe.Kind]
	if !ok {
		return diagnostic.Report{}, fmt.Errorf("exact diagnostics provider %q is not included in this AIGW build", providerAccount.AccountProbe.Kind)
	}
	return implementation(ctx, client, providerAccount, apiToken, auth)
}
