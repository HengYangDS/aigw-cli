// Package providers contains explicitly bundled, provider-native diagnostics.
// It is intentionally outside AIGW's Account/Profile/Route/Adapter core: an
// Account may declare a diagnostic provider that is not present in a given
// build while all ordinary routing and health checks continue to work.
package providers

import (
	"context"
	"fmt"
	"net/http"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers/dmxapi"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Supports reports whether this AIGW build has an explicit native diagnostic
// integration for kind. Unknown kinds remain valid manifest data; only their
// exact provider-native diagnostic command is unavailable.
func Supports(kind string) bool {
	switch kind {
	case "dmxapi":
		return true
	default:
		return false
	}
}

func Probe(ctx context.Context, client HTTPDoer, providerAccount domain.Account, apiToken string, credential account.Credential) (account.Report, error) {
	if providerAccount.AccountProbe == nil {
		return account.Report{}, fmt.Errorf("account %q has no exact diagnostic provider", providerAccount.ID)
	}
	switch providerAccount.AccountProbe.Kind {
	case "dmxapi":
		return dmxapi.Probe(ctx, client, providerAccount, apiToken, credential)
	default:
		return account.Report{}, fmt.Errorf("exact diagnostics provider %q is not included in this AIGW build", providerAccount.AccountProbe.Kind)
	}
}
