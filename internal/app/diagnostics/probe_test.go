package diagnostics_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/app/diagnostics"
	"aigw-cli/internal/domain"
)

type clientFunc func(*http.Request) (*http.Response, error)

func (f clientFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func assertDiagnosticOmits(t *testing.T, result diagnostics.Result, forbidden ...string) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range forbidden {
		if strings.Contains(string(data), fragment) {
			t.Fatalf("diagnostic exposed private content %q: %s", fragment, data)
		}
	}
}

func runtime() domain.Runtime {
	return domain.Runtime{Model: domain.NewModelRef("dmx", "dmx"), ModelLabel: "DMXAPI", ProviderID: "dmx", ProviderLabel: "DMXAPI", Client: codex.ID, Protocol: domain.ProtocolResponses, Endpoint: "https://service.test/v1"}
}

func TestProbeClassifiesUsefulFailureCauses(t *testing.T) {
	tests := []struct {
		status int
		body   string
		kind   diagnostics.Kind
	}{
		{401, `{"message":"invalid api key"}`, diagnostics.InvalidToken},
		{403, `{"message":"token quota is insufficient"}`, diagnostics.QuotaExhausted},
		{403, `{"message":"token disabled"}`, diagnostics.TokenDisabled},
		{403, `{"message":"forbidden"}`, diagnostics.TokenRestricted},
		{429, `{"message":"too many requests"}`, diagnostics.RateLimited},
		{503, `{"message":"no available channel for model"}`, diagnostics.ModelUnavailable},
		{500, `{"message":"internal error"}`, diagnostics.UpstreamFailure},
		{404, `{"message":"not found"}`, diagnostics.EndpointMismatch},
	}
	for _, tt := range tests {
		result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
			return response(tt.status, tt.body), nil
		}), runtime(), "secret", diagnostics.ScopeEndpoint)
		if result.Kind != tt.kind || result.Summary == "" {
			t.Errorf("status %d body %s => %#v", tt.status, tt.body, result)
		}
	}
}

func TestProbeSeparatesHardQuotaFromTransientRateLimit(t *testing.T) {
	for _, test := range []struct {
		name      string
		body      string
		kind      diagnostics.Kind
		retryable bool
	}{
		{"insufficient quota code", `{"error":{"code":"insufficient_quota","message":"Current quota exceeded"}}`, diagnostics.QuotaExhausted, false},
		{"quota exhausted code", `{"error":{"code":"quota_exhausted"}}`, diagnostics.QuotaExhausted, false},
		{"insufficient balance", `{"message":"Insufficient balance"}`, diagnostics.QuotaExhausted, false},
		{"exhausted balance", `{"message":"Balance exhausted"}`, diagnostics.QuotaExhausted, false},
		{"exhausted credits", `{"message":"Credits exhausted"}`, diagnostics.QuotaExhausted, false},
		{"rate limit code", `{"error":{"code":"rate_limit_exceeded","message":"Too many requests"}}`, diagnostics.RateLimited, true},
		{"concurrency quota", `{"message":"Concurrency quota exhausted; retry later"}`, diagnostics.RateLimited, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := diagnostics.Probe(t.Context(), clientFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusTooManyRequests, test.body), nil
			}), runtime(), "secret", diagnostics.ScopeEndpoint)
			if result.Kind != test.kind || result.Retryable != test.retryable || result.Attempts != 1 || result.HTTPStatus != http.StatusTooManyRequests {
				t.Fatalf("Probe() = %#v, want kind %s retryable %t", result, test.kind, test.retryable)
			}
		})
	}
}

func TestProbeUsesModelsEndpointAndNeverReturnsCredential(t *testing.T) {
	secret := "never-return-this-token"
	var requestURL, authorization string
	result := diagnostics.Probe(context.Background(), clientFunc(func(req *http.Request) (*http.Response, error) {
		requestURL = req.URL.String()
		authorization = req.Header.Get("Authorization")
		return response(200, `{"data":"provider-private-marker"}`), nil
	}), runtime(), secret, diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.Healthy || requestURL != "https://service.test/v1/models" || authorization != "Bearer "+secret {
		t.Fatalf("result=%#v url=%q auth=%q", result, requestURL, authorization)
	}
	if result.Summary != "the provider accepted the Token" {
		t.Fatalf("summary = %q", result.Summary)
	}
	assertDiagnosticOmits(t, result, secret, "provider-private-marker")
}

func TestProbeDoesNotPublishProviderResponseContent(t *testing.T) {
	secret := "aigw-test-account-token-never-leaks"
	const fragment = "account-token-never"
	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, `{"message":"rejected token fragment account-token-never"}`), nil
	}), runtime(), secret, diagnostics.ScopeEndpoint)
	assertDiagnosticOmits(t, result, fragment)
}

// Each failure is summarized in the product's words, naming no gateway,
// Account or Route; the command that sent the request gives the fix.
func TestProbeUsesEndpointNeutralFailures(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantKind    diagnostics.Kind
		wantSummary string
	}{
		{name: "invalid token", status: http.StatusUnauthorized, body: `{}`, wantKind: diagnostics.InvalidToken, wantSummary: "the provider rejected the Token"},
		{name: "quota exhausted", status: http.StatusForbidden, body: `{"message":"quota exhausted"}`, wantKind: diagnostics.QuotaExhausted, wantSummary: "the Token's quota is exhausted"},
		{name: "token disabled", status: http.StatusForbidden, body: `{"message":"token disabled"}`, wantKind: diagnostics.TokenDisabled, wantSummary: "the provider disabled the Token"},
		{name: "token restricted", status: http.StatusForbidden, body: `{"message":"forbidden"}`, wantKind: diagnostics.TokenRestricted, wantSummary: "the provider restricted what the Token may do"},
		{name: "missing protocol path", status: http.StatusNotFound, body: `{}`, wantKind: diagnostics.EndpointMismatch, wantSummary: "the endpoint has no such path for this protocol"},
		{name: "upstream failure", status: http.StatusInternalServerError, body: `{}`, wantKind: diagnostics.UpstreamFailure, wantSummary: "the provider failed"},
		{name: "request timeout", status: http.StatusRequestTimeout, body: `{}`, wantKind: diagnostics.Timeout, wantSummary: "the endpoint gave up waiting for the request"},
		{name: "unexpected response", status: http.StatusTeapot, body: `{}`, wantKind: diagnostics.Unexpected, wantSummary: "the endpoint gave an answer that AIGW does not expect"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
				return response(test.status, test.body), nil
			}), runtime(), "secret", diagnostics.ScopeEndpoint)
			if result.Kind != test.wantKind || result.Summary != test.wantSummary || result.HTTPStatus != test.status {
				t.Fatalf("Probe() = %#v", result)
			}
			for _, word := range []string{"gateway", "account", "route"} {
				if strings.Contains(strings.ToLower(result.Summary), word) {
					t.Fatalf("the summary names %s: %#v", word, result)
				}
			}
		})
	}
}

func TestProbeClassifiesNetworkFailure(t *testing.T) {
	const privatePath = "/private/account-store"
	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: network unreachable at " + privatePath)
	}), runtime(), "secret", diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.NetworkFailure || !result.Retryable {
		t.Fatalf("result = %#v", result)
	}
	assertDiagnosticOmits(t, result, privatePath)
}

func TestProbeOmitsMalformedEndpointText(t *testing.T) {
	selected := runtime()
	selected.Endpoint = "://private-account-store"
	result := diagnostics.Probe(t.Context(), clientFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("malformed endpoint reached the network")
		return nil, nil
	}), selected, "synthetic-token", diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.EndpointMismatch || result.Attempts != 0 {
		t.Fatalf("result = %#v", result)
	}
	assertDiagnosticOmits(t, result, "private-account-store")
}

func TestProbeBoundedDoesNotRetryCredentialRejection(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   diagnostics.Kind
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"message":"invalid api key"}`, want: diagnostics.InvalidToken},
		{name: "forbidden", status: http.StatusForbidden, body: `{"message":"forbidden"}`, want: diagnostics.TokenRestricted},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			client := clientFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return response(test.status, test.body), nil
			})
			result := diagnostics.ProbeBounded(context.Background(), client, runtime(), "secret", diagnostics.ScopeEndpoint)
			if calls != 1 || result.Attempts != 1 || result.Kind != test.want {
				t.Fatalf("calls=%d result=%#v, want one terminal %s request", calls, result, test.name)
			}
		})
	}
}

func TestProbeBoundedPreflightDoesNotClaimHTTPAttempt(t *testing.T) {
	missingModel := inferenceRuntime()
	missingModel.UpstreamModel = ""
	for _, test := range []struct {
		name    string
		runtime domain.Runtime
		scope   diagnostics.Scope
		want    diagnostics.Kind
	}{
		{name: "missing endpoint", runtime: domain.Runtime{Client: codex.ID}, scope: diagnostics.ScopeEndpoint, want: diagnostics.EndpointMismatch},
		{name: "missing model", runtime: missingModel, scope: diagnostics.ScopeInference, want: diagnostics.ModelUnresolved},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			result := diagnostics.ProbeBounded(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return response(http.StatusOK, `{"data":[]}`), nil
			}), test.runtime, "secret", test.scope)
			if calls != 0 || result.Attempts != 0 || result.Kind != test.want {
				t.Fatalf("calls=%d result=%#v, want no HTTP attempt", calls, result)
			}
		})
	}
}

func TestProbeBoundedUsesEndpointDeadline(t *testing.T) {
	var remaining time.Duration
	result := diagnostics.ProbeBounded(context.Background(), clientFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("endpoint request has no deadline")
		}
		remaining = time.Until(deadline)
		return response(http.StatusOK, `{"data":[]}`), nil
	}), runtime(), "secret", diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.Healthy || remaining < 4*time.Second || remaining > 6*time.Second {
		t.Fatalf("result=%#v remaining=%s, want one bounded 5s attempt", result, remaining)
	}
}

func TestProbeBoundedClosesResponse(t *testing.T) {
	closed := 0
	result := diagnostics.ProbeBounded(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: &countingReadCloser{Reader: strings.NewReader(`{"message":"invalid api key"}`), closed: &closed}}, nil
	}), runtime(), "secret", diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.InvalidToken || closed != 1 {
		t.Fatalf("result=%#v closed=%d, want one closed response", result, closed)
	}
}

func TestProbeBoundedNeverReturnsCredential(t *testing.T) {
	secret := "aigw-stability-token-never-leaks"
	result := diagnostics.ProbeBounded(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusUnauthorized, `{"message":"rejected aigw-stability-token-never-leaks"}`), nil
	}), runtime(), secret, diagnostics.ScopeEndpoint)
	assertDiagnosticOmits(t, result, secret)
}

type countingReadCloser struct {
	io.Reader
	closed *int
}

func (c *countingReadCloser) Close() error {
	*c.closed++
	return nil
}
