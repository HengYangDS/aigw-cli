package dmxapi_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/providers/dmxapi"
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
	providerAccount := configuration.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
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
	providerAccount := configuration.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
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
	providerAccount := configuration.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{SystemToken: systemToken, UserID: userID})
	if err == nil {
		t.Fatal("provider probe unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), systemToken) || strings.Contains(err.Error(), userID) {
		t.Fatalf("provider JSON error leaked platform credential: %v", err)
	}
}

func TestProbeHandlesConfigurationErrors(t *testing.T) {
	ctx := context.Background()
	client := roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	cred := account.Credential{}

	// nil AccountProbe
	_, err := dmxapi.Probe(ctx, client, configuration.Account{AccountProbe: nil}, "", cred)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected error for nil AccountProbe, got %v", err)
	}

	// Wrong Kind
	_, err = dmxapi.Probe(ctx, client, configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "other"}}, "", cred)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected error for wrong Kind, got %v", err)
	}
}

func TestProbeTokenNotFound(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		body := `{"success":true,"data":{"quota":0}}`
		if strings.Contains(req.URL.Path, "/api/token/search") {
			body = `{"success":true,"data":{"items":[]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-no-match", account.Credential{})
	if err == nil || !strings.Contains(err.Error(), "not found in the DMXAPI account") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestProbeTokenDisabled(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		body := `{"success":true,"data":{"quota":0}}`
		if strings.Contains(req.URL.Path, "/api/token/search") {
			body = `{"success":true,"data":{"items":[{"key":"abcd**********wxyz","status":0}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	report, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", account.Credential{})
	if err != nil {
		t.Fatal(err)
	}
	if report.TokenStatus != "disabled" {
		t.Errorf("expected status disabled, got %q", report.TokenStatus)
	}
}

func TestProbePagination(t *testing.T) {
	calls := 0
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{"success":true,"data":{"quota":0}}`
		if strings.Contains(req.URL.Path, "/api/token/search") {
			if strings.Contains(req.URL.RawQuery, "page=1") {
				items := make([]string, 100)
				for i := range items {
					items[i] = `{"key":"other"}`
				}
				body = `{"success":true,"data":{"items":[` + strings.Join(items, ",") + `],"page_size":100}}`
			} else {
				body = `{"success":true,"data":{"items":[{"key":"abcd**********wxyz","status":1}]}}`
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", account.Credential{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 { // 1 for user info, 2 for tokens
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestMaskedToken(t *testing.T) {
	// Internal function testing via public Probe or if it was exported.
	// Since it's not exported, we use Probe with specific tokens.

	cases := []struct {
		input    string
		expected string
	}{
		{"12345", "12345"}, // < 8
		{"sk-abcd-middle-wxyz", "abcd**********wxyz"},
		{"a%20b%20c%20d-middle-wxyz", "a b **********wxyz"},
	}

	for _, c := range cases {
		client := roundTrip(func(req *http.Request) (*http.Response, error) {
			body := `{"success":true,"data":{"quota":0}}`
			if strings.Contains(req.URL.Path, "/api/token/search") {
				body = `{"success":true,"data":{"items":[{"key":"` + c.expected + `","status":1}]}}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
		_, err := dmxapi.Probe(context.Background(), client, providerAccount, c.input, account.Credential{})
		if err != nil {
			t.Errorf("input %q: unexpected error %v", c.input, err)
		}
	}
}

func TestProbeFetchTokensError(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/api/user/self") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"quota":0}}`))}, nil
		}
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`error`))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected error from fetchTokens, got %v", err)
	}
}

func TestProbeFetchTokensSuccessFalse(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/api/user/self") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"quota":0}}`))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":false,"message":"token search failed"}`))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{})
	if err == nil || !strings.Contains(err.Error(), "token query failed") {
		t.Errorf("expected error from fetchTokens, got %v", err)
	}
}

func TestProbeNetworkError(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{})
	if err == nil || !strings.Contains(err.Error(), "network error") {
		t.Errorf("expected network error, got %v", err)
	}
}
