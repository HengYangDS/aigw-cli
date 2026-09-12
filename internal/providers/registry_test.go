package providers_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/providers"
	"aigw-cli/internal/secrets"
)

func TestUnknownDiagnosticProviderIsRejectedAtExecutionNotConfiguration(t *testing.T) {
	providerAccount := configuration.Account{
		ID:           "future",
		Label:        "Future Gateway",
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"},
	}
	if providers.Supports(providerAccount.AccountProbe.Kind) {
		t.Fatal("an unbundled provider must not report support")
	}
	_, err := providers.Probe(context.Background(), nil, providerAccount, "api-token", secrets.DiagnosticCredential{SystemToken: "platform-token", UserID: "1"})
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
	_, err := providers.Probe(context.Background(), nil, configuration.Account{ID: "no-probe"}, "api-token", secrets.DiagnosticCredential{})
	if err == nil || !strings.Contains(err.Error(), "no exact diagnostic provider") {
		t.Fatalf("error = %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestProbeDispatchesToDMXAPI(t *testing.T) {
	var paths []string
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		if req.Header.Get("Authorization") != "Bearer platform-token" {
			t.Fatal("diagnostic dispatch lost its platform credential")
		}
		body := `{"success":true,"data":{"quota":6250000}}`
		if req.URL.Path == "/api/token/search" {
			body = `{"success":true,"data":{"items":[{"key":"abcd**********wxyz","name":"Selected Token","status":1,"remain_quota":2500000}]}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{
		ID:           "dmx",
		AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostic.test"},
	}
	report, err := providers.Probe(t.Context(), client, providerAccount, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{SystemToken: "platform-token"})
	if err != nil || report.AccountBalance != 12.5 || report.TokenRemaining != 5 || report.TokenName != "Selected Token" || report.TokenStatus != "enabled" {
		t.Fatalf("dispatched diagnostic = %#v, %v", report, err)
	}
	if strings.Join(paths, ",") != "/api/user/self,/api/token/search" {
		t.Fatalf("diagnostic request sequence = %v", paths)
	}
}
