package dmxapi_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/providers/dmxapi"
	"aigw-cli/internal/secrets"
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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{ID: "dmx", Label: "DMXAPI", AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	report, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{SystemToken: "system-secret", UserID: "10000"})
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
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", secrets.DiagnosticCredential{SystemToken: systemToken, UserID: userID})
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
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", secrets.DiagnosticCredential{SystemToken: systemToken, UserID: userID})
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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	cred := secrets.DiagnosticCredential{}

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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-no-match", secrets.DiagnosticCredential{})
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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	report, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{})
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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{})
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
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
		_, err := dmxapi.Probe(context.Background(), client, providerAccount, c.input, secrets.DiagnosticCredential{})
		if err != nil {
			t.Errorf("input %q: unexpected error %v", c.input, err)
		}
	}
}

func TestProbeFetchTokensFailures(t *testing.T) {
	for _, test := range []struct {
		name, body, message string
		status              int
	}{
		{"HTTP error", "error", "HTTP 500", http.StatusInternalServerError},
		{"rejected search", `{"success":false,"message":"token search failed"}`, "token query failed", http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := roundTrip(func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/user/self") {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"quota":0}}`))}, nil
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
			_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", secrets.DiagnosticCredential{})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Errorf("token search error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestProbeNetworkError(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	})
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), client, providerAccount, "api-token", secrets.DiagnosticCredential{})
	if err == nil || !strings.Contains(err.Error(), "network error") {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestProbeRejectsInvalidBaseURL(t *testing.T) {
	providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "://bad"}}
	_, err := dmxapi.Probe(context.Background(), nil, providerAccount, "api-token", secrets.DiagnosticCredential{})
	if err == nil || !strings.Contains(err.Error(), "missing protocol scheme") {
		t.Fatalf("invalid base URL error = %v", err)
	}
}

func TestProbeKeepsPlatformCredentialsAtTheSelectedEndpoint(t *testing.T) {
	for _, redirectedPath := range []string{"/api/user/self", "/api/token/search"} {
		t.Run(redirectedPath, func(t *testing.T) {
			var destinationCalls atomic.Int32
			redirectChecks := 0
			respond := func(w http.ResponseWriter, r *http.Request) {
				body := `{"success":true,"data":{"quota":500000}}`
				if r.URL.Path == "/api/token/search" {
					body = `{"success":true,"data":{"items":[{"key":"abcd**********wxyz","status":1}]}}`
				}
				_, _ = io.WriteString(w, body)
			}
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationCalls.Add(1)
				respond(w, r)
			}))
			defer destination.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == redirectedPath {
					http.Redirect(w, r, destination.URL+r.URL.RequestURI(), http.StatusFound)
					return
				}
				respond(w, r)
			}))
			defer origin.Close()
			client := origin.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { redirectChecks++; return nil }
			providerAccount := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: origin.URL}}
			_, err := dmxapi.Probe(t.Context(), client, providerAccount, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{SystemToken: "platform-secret", UserID: "account-owner"})
			if err == nil || !strings.Contains(err.Error(), "HTTP 302") || destinationCalls.Load() != 0 || redirectChecks != 0 {
				t.Fatalf("redirect result = %v, destination calls = %d, client redirect checks = %d", err, destinationCalls.Load(), redirectChecks)
			}
			if client.CheckRedirect == nil {
				t.Fatal("probe removed the caller's redirect policy")
			}
			if err := client.CheckRedirect(nil, nil); err != nil || redirectChecks != 1 {
				t.Fatalf("probe changed the caller's redirect policy: %v", err)
			}
		})
	}
}

type observedBody struct {
	io.Reader
	closeErr error
	closed   int
}

func (body *observedBody) Close() error { body.closed++; return body.closeErr }

type failedRead struct{ err error }

func (reader failedRead) Read([]byte) (int, error) { return 0, reader.err }

func TestProbeRequiresOneCompleteBoundedResponse(t *testing.T) {
	const user = `{"success":true,"data":{"quota":500000}}`
	readErr, closeErr := errors.New("body read failed"), errors.New("body close failed")
	for _, test := range []struct {
		name     string
		reader   io.Reader
		closeErr error
		wantErr  error
	}{
		{"extra document", strings.NewReader(user + `{}`), nil, nil},
		{"oversized response", strings.NewReader(user + strings.Repeat(" ", 2<<20)), nil, nil},
		{"read failure after JSON", io.MultiReader(strings.NewReader(user), failedRead{readErr}), nil, readErr},
		{"close failure", strings.NewReader(user), closeErr, closeErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &observedBody{Reader: test.reader, closeErr: test.closeErr}
			requests := 0
			client := roundTrip(func(*http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"items":[{"key":"abcd**********wxyz"}]}}`))}, nil
			})
			cfg := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostic.test"}}
			_, err := dmxapi.Probe(t.Context(), client, cfg, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{})
			if err == nil || (test.wantErr != nil && !errors.Is(err, test.wantErr)) || requests != 1 || body.closed != 1 {
				t.Fatalf("probe error = %v; want %v, requests = %d, close count = %d", err, test.wantErr, requests, body.closed)
			}
		})
	}
}

func TestProbeReportsIncompletePaginationInsteadOfTokenAbsence(t *testing.T) {
	tokenCalls := 0
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		body := `{"success":true,"data":{"quota":0}}`
		if req.URL.Path == "/api/token/search" {
			tokenCalls++
			body = `{"success":true,"data":{"items":[` + strings.TrimSuffix(strings.Repeat(`{"key":"other"},`, 100), ",") + `],"page_size":100}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	cfg := configuration.Account{AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostic.test"}}
	_, err := dmxapi.Probe(t.Context(), client, cfg, "sk-abcd-middle-wxyz", secrets.DiagnosticCredential{})
	if err == nil || !strings.Contains(err.Error(), "incomplete") || tokenCalls != 100 {
		t.Fatalf("bounded pagination error = %v, token calls = %d", err, tokenCalls)
	}
}
