package providers_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers"
)

func TestUnknownDiagnosticProviderIsRejectedAtExecutionNotConfiguration(t *testing.T) {
	providerAccount := domain.Account{
		ID:           "future",
		Label:        "Future Gateway",
		AccountProbe: &domain.AccountProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"},
	}
	if providers.Supports(providerAccount.AccountProbe.Kind) {
		t.Fatal("an unbundled provider must not report support")
	}
	_, err := providers.Probe(context.Background(), nil, providerAccount, "api-token", account.Credential{SystemToken: "platform-token", UserID: "1"})
	if err == nil || !strings.Contains(err.Error(), "not included in this AIGW build") {
		t.Fatalf("error = %v", err)
	}
}

func TestBundledDMXAPIDiagnosticsAreExplicitlyRegistered(t *testing.T) {
	if !providers.Supports("dmxapi") {
		t.Fatal("DMXAPI diagnostics should be an explicit bundled provider integration")
	}
}

func TestProbeRequiresAccountProbeConfiguration(t *testing.T) {
	_, err := providers.Probe(context.Background(), nil, domain.Account{ID: "no-probe"}, "api-token", account.Credential{})
	if err == nil || !strings.Contains(err.Error(), "no exact diagnostic provider") {
		t.Fatalf("error = %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestProbeDispatchesToDMXAPI(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"quota":100}}`))}, nil
	})
	providerAccount := domain.Account{
		ID:           "dmx",
		AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"},
	}
	// We expect an error because the second call (fetchTokens) will fail due to our simple mock,
	// but this confirms it reached the dmxapi case.
	_, err := providers.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{})
	if err == nil || (!strings.Contains(err.Error(), "DMXAPI token query failed") && !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "not found in the DMXAPI account")) {
		t.Fatalf("error = %v", err)
	}
}
