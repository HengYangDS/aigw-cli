// Package providers contains explicitly bundled, provider-native diagnostics.
// It is intentionally outside AIGW's Account/Profile/Route/Adapter core: an
// Account may declare a diagnostic provider that is not present in a given
// build while all ordinary routing and health checks continue to work.
package providers

import (
	"context"
	"fmt"
	"net/http"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/providers/dmxapi"
)

// httpDoer is the provider-neutral transport required by every bundled
// account diagnostic. Leaf integrations satisfy it structurally.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type probeFunc func(context.Context, httpDoer, configuration.Account, string, account.Credential) (account.Report, error)

var registry = map[string]probeFunc{
	dmxapi.Kind: func(ctx context.Context, client httpDoer, providerAccount configuration.Account, apiToken string, credential account.Credential) (account.Report, error) {
		return dmxapi.Probe(ctx, client, providerAccount, apiToken, credential)
	},
}

// Supports reports whether this AIGW build has an explicit native diagnostic
// integration for kind. Unknown kinds remain valid manifest data; only their
// exact provider-native diagnostic command is unavailable.
func Supports(kind string) bool {
	_, ok := registry[kind]
	return ok
}

func Probe(ctx context.Context, client httpDoer, providerAccount configuration.Account, apiToken string, credential account.Credential) (account.Report, error) {
	if providerAccount.AccountProbe == nil {
		return account.Report{}, fmt.Errorf("account %q has no exact diagnostic provider", providerAccount.ID)
	}
	implementation, ok := registry[providerAccount.AccountProbe.Kind]
	if !ok {
		return account.Report{}, fmt.Errorf("exact diagnostics provider %q is not included in this AIGW build", providerAccount.AccountProbe.Kind)
	}
	return implementation(ctx, client, providerAccount, apiToken, credential)
}
