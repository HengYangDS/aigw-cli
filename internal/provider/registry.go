// Package provider contains explicitly bundled, provider-native diagnostics.
// It is intentionally outside AIGW's Account/Model/Route/Adapter core: an
// Account may declare a diagnostic provider that is not present in a given
// build while all ordinary routing and health checks continue to work.
package provider

import (
	"context"
	"fmt"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/provider/diagnostic"
	"aigw-cli/internal/provider/dmxapi"
)

type probeFunc func(context.Context, diagnostic.Sender, domain.Account, string, diagnostic.Credential) (diagnostic.Report, error)

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
func Probe(ctx context.Context, sender diagnostic.Sender, providerAccount domain.Account, apiToken string, auth diagnostic.Credential) (diagnostic.Report, error) {
	if providerAccount.AccountProbe == nil {
		return diagnostic.Report{}, fmt.Errorf("account %q has no exact diagnostic provider", providerAccount.ID)
	}
	implementation, ok := registry[providerAccount.AccountProbe.Kind]
	if !ok {
		return diagnostic.Report{}, fmt.Errorf("exact diagnostics provider %q is not included in this AIGW build", providerAccount.AccountProbe.Kind)
	}
	return implementation(ctx, sender, providerAccount, apiToken, auth)
}
