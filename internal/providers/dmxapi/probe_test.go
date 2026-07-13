package dmxapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers/dmxapi"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestProbeReturnsAccountAndCurrentTokenDetails(t *testing.T) {
	requests := 0
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Header.Get("Authorization") != "Bearer system-secret" || req.Header.Get("Rix-Api-User") != "10000" {
			t.Fatalf("unsafe/wrong headers: %#v", req.Header)
		}
		body := `{"success":true,"data":{"quota":6250000}}`
		if strings.Contains(req.URL.Path, "/api/token/search") {
			body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":1,"used_quota":1000000,"remain_quota":2500000,"unlimited_quota":false,"unlimited_count":true,"expired_time":-1}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := domain.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	report, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", account.Credential{SystemToken: "system-secret", UserID: "10000"})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || report.AccountBalance != 12.5 || report.TokenName != "Codex" || report.TokenRemaining != 5 || report.TokenUsed != 2 || report.TokenStatus != "enabled" || !report.TokenUnlimitedCount {
		t.Fatalf("report = %#v requests=%d", report, requests)
	}
}

func TestProbeRedactsPlatformCredentialsEchoedByTheProvider(t *testing.T) {
	const systemToken = "platform-token-must-not-leak"
	const userID = "sensitive-user-id"
	client := roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("provider echoed " + systemToken + " for " + userID)),
		}, nil
	})
	providerAccount := domain.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{SystemToken: systemToken, UserID: userID})
	if err == nil {
		t.Fatal("provider probe unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), systemToken) || strings.Contains(err.Error(), userID) {
		t.Fatalf("provider error leaked platform credential: %v", err)
	}
}

func TestProbeRedactsPlatformCredentialsFromJSONFailureMessage(t *testing.T) {
	const systemToken = "platform-message-token-must-not-leak"
	const userID = "sensitive-message-user"
	client := roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"provider echoed platform-message-token-must-not-leak for sensitive-message-user"}`)),
		}, nil
	})
	providerAccount := domain.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{SystemToken: systemToken, UserID: userID})
	if err == nil {
		t.Fatal("provider probe unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), systemToken) || strings.Contains(err.Error(), userID) {
		t.Fatalf("provider JSON error leaked platform credential: %v", err)
	}
}
