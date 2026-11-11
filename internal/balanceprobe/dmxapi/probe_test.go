package dmxapi_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/balanceprobe/diagnostic"
	"aigw-cli/internal/balanceprobe/dmxapi"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret/probe"
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
	providerAccount := domain.Provider{ID: "dmx", Label: "DMXAPI", BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	report, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "sk-abcd-middle-wxyz", diagnostic.Credential{SystemToken: "system-secret", UserID: "10000"})
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
	providerAccount := domain.Provider{ID: "dmx", Label: "DMXAPI", BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "api-token", diagnostic.Credential{SystemToken: systemToken, UserID: userID})
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
	providerAccount := domain.Provider{ID: "dmx", Label: "DMXAPI", BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "api-token", diagnostic.Credential{SystemToken: systemToken, UserID: userID})
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
	cred := diagnostic.Credential{}

	// nil BalanceProbe
	_, err := dmxapi.Probe(ctx, probe.Sender{Client: client}, domain.Provider{BalanceProbe: nil}, "", cred)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected error for nil AccountProbe, got %v", err)
	}

	// Wrong Kind
	_, err = dmxapi.Probe(ctx, probe.Sender{Client: client}, domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "other"}}, "", cred)
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
	providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "sk-no-match", diagnostic.Credential{})
	if err == nil || !strings.Contains(err.Error(), "not found at DMXAPI") {
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
	providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	report, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "sk-abcd-middle-wxyz", diagnostic.Credential{})
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
	providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "sk-abcd-middle-wxyz", diagnostic.Credential{})
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
		providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
		_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, c.input, diagnostic.Credential{})
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
			providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
			_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "api-token", diagnostic.Credential{})
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
	providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
	_, err := dmxapi.Probe(context.Background(), probe.Sender{Client: client}, providerAccount, "api-token", diagnostic.Credential{})
	if err == nil || !strings.Contains(err.Error(), "network error") {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestProbeRejectsInvalidBaseURL(t *testing.T) {
	providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "://bad"}}
	_, err := dmxapi.Probe(context.Background(), nil, providerAccount, "api-token", diagnostic.Credential{})
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
			providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: origin.URL}}
			_, err := dmxapi.Probe(t.Context(), probe.Sender{Client: client}, providerAccount, "sk-abcd-middle-wxyz", diagnostic.Credential{SystemToken: "platform-secret", UserID: "account-owner"})
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
			cfg := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostic.test"}}
			_, err := dmxapi.Probe(t.Context(), probe.Sender{Client: client}, cfg, "sk-abcd-middle-wxyz", diagnostic.Credential{})
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
	cfg := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostic.test"}}
	_, err := dmxapi.Probe(t.Context(), probe.Sender{Client: client}, cfg, "sk-abcd-middle-wxyz", diagnostic.Credential{})
	if err == nil || !strings.Contains(err.Error(), "incomplete") || !errors.Is(err, diagnostic.ErrSearchIncomplete) || tokenCalls != 100 {
		t.Fatalf("bounded pagination error = %v, token calls = %d", err, tokenCalls)
	}
}

// TestProbeFailuresKeepTheirKind shows that a caller can tell a status, a
// request without a response, a refused query and an unlisted Token apart.
func TestProbeFailuresKeepTheirKind(t *testing.T) {
	user := `{"success":true,"data":{"quota":0}}`
	for _, test := range []struct {
		name   string
		answer func(*http.Request) (*http.Response, error)
		check  func(error) bool
	}{
		{"status", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("denied"))}, nil
		}, func(err error) bool {
			status, ok := errors.AsType[*diagnostic.StatusError](err)
			return ok && status.Status == http.StatusForbidden && strings.Contains(err.Error(), "HTTP 403: denied")
		}},
		{"no response", func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }, func(err error) bool {
			_, ok := errors.AsType[*diagnostic.UnreachableError](err)
			return ok && errors.Unwrap(err).Error() == "offline"
		}},
		{"refused account query", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"success":false,"message":"no"}`))}, nil
		}, func(err error) bool {
			refused, ok := errors.AsType[*diagnostic.RefusedError](err)
			return ok && refused.Error() == "DMXAPI balance query failed: no"
		}},
		{"refused token query", func(request *http.Request) (*http.Response, error) {
			body := user
			if request.URL.Path == "/api/token/search" {
				body = `{"success":false,"message":"no"}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}, func(err error) bool {
			_, ok := errors.AsType[*diagnostic.RefusedError](err)
			return ok
		}},
		{"unlisted Token", func(request *http.Request) (*http.Response, error) {
			body := user
			if request.URL.Path == "/api/token/search" {
				body = `{"success":true,"data":{"items":[]}}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}, func(err error) bool {
			return errors.Is(err, diagnostic.ErrTokenNotListed) && err.Error() == "current API Token was not found at DMXAPI"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
			_, err := dmxapi.Probe(t.Context(), probe.Sender{Client: roundTrip(test.answer)}, providerAccount, "sk-abcd-middle-wxyz", diagnostic.Credential{})
			if err == nil || !test.check(err) {
				t.Fatalf("error = %v (%T)", err, err)
			}
		})
	}
}

// No failure of the probe names a retired word: the text reaches the user.
func TestProbeFailuresSpeakTheProductVocabulary(t *testing.T) {
	retired := regexp.MustCompile(`(?i)\b(accounts?|routes?|projections?|projected|bindings?)\b`)
	user := `{"success":true,"data":{"quota":0}}`
	for name, answer := range map[string]func(*http.Request) string{
		"refused query": func(*http.Request) string { return `{"success":false,"message":"no"}` },
		"unlisted Token": func(request *http.Request) string {
			if request.URL.Path == "/api/token/search" {
				return `{"success":true,"data":{"items":[]}}`
			}
			return user
		},
		"refused token query": func(request *http.Request) string {
			if request.URL.Path == "/api/token/search" {
				return `{"success":false,"message":"no"}`
			}
			return user
		},
		"incomplete search": func(request *http.Request) string {
			if request.URL.Path == "/api/token/search" {
				return `{"success":true,"data":{"items":[` + strings.TrimSuffix(strings.Repeat(`{"key":"other"},`, 100), ",") + `],"page_size":100}}`
			}
			return user
		},
	} {
		client := roundTrip(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(answer(request)))}, nil
		})
		providerAccount := domain.Provider{BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://example.com"}}
		_, err := dmxapi.Probe(t.Context(), probe.Sender{Client: client}, providerAccount, "sk-abcd-middle-wxyz", diagnostic.Credential{})
		if err == nil || retired.MatchString(err.Error()) {
			t.Errorf("%s: %v names a retired word", name, err)
		}
	}
}
