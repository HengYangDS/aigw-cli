package providers_test

import (
	"context"
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
