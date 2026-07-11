package account_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestDMXProbeReturnsAccountAndCurrentTokenDetails(t *testing.T) {
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
	report, err := account.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", account.Credential{SystemToken: "system-secret", UserID: "10000"})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || report.AccountBalance != 12.5 || report.TokenName != "Codex" || report.TokenRemaining != 5 || report.TokenUsed != 2 || report.TokenStatus != "enabled" || !report.TokenUnlimitedCount {
		t.Fatalf("report = %#v requests=%d", report, requests)
	}
}
