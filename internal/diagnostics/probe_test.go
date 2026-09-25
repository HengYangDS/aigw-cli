package diagnostics_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/diagnostics"
)

type clientFunc func(*http.Request) (*http.Response, error)

func (f clientFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func runtime() configuration.Runtime {
	return configuration.Runtime{RouteID: "dmx", RouteLabel: "DMXAPI", AccountID: "dmx", AccountLabel: "DMXAPI", Client: configuration.ClientCodex, Endpoint: "https://service.test/v1"}
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
		if result.Kind != tt.kind || result.Fix == "" || result.Summary == "" {
			t.Errorf("status %d body %s => %#v", tt.status, tt.body, result)
		}
	}
}

func TestProbeUsesModelsEndpointAndNeverReturnsCredential(t *testing.T) {
	secret := "never-return-this-token"
	var requestURL, authorization string
	result := diagnostics.Probe(context.Background(), clientFunc(func(req *http.Request) (*http.Response, error) {
		requestURL = req.URL.String()
		authorization = req.Header.Get("Authorization")
		return response(200, `{"data":[]}`), nil
	}), runtime(), secret, diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.Healthy || requestURL != "https://service.test/v1/models" || authorization != "Bearer "+secret {
		t.Fatalf("result=%#v url=%q auth=%q", result, requestURL, authorization)
	}
	if result.Summary != "Endpoint diagnostic returned a successful response" {
		t.Fatalf("summary = %q", result.Summary)
	}
	if strings.Contains(result.Summary+result.Detail+result.Fix, secret) {
		t.Fatalf("credential leaked: %#v", result)
	}
}

func TestProbeRedactsAnAccountTokenEchoedByTheEndpoint(t *testing.T) {
	secret := "aigw-test-account-token-never-leaks"
	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, `{"message":"rejected token aigw-test-account-token-never-leaks"}`), nil
	}), runtime(), secret, diagnostics.ScopeEndpoint)
	if strings.Contains(result.Detail, secret) {
		t.Fatalf("endpoint response leaked Account Token: %#v", result)
	}
}

func TestProbeUsesEndpointNeutralFailures(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantKind    diagnostics.Kind
		wantSummary string
		wantFix     string
	}{
		{
			name:        "invalid token",
			status:      http.StatusUnauthorized,
			body:        `{}`,
			wantKind:    diagnostics.InvalidToken,
			wantSummary: "Account Token is invalid or does not belong to the configured endpoint",
			wantFix:     "Run `aigw rotate dmx` to enter the token again, and confirm that the configured endpoint belongs to this Account",
		},
		{
			name:        "quota exhausted",
			status:      http.StatusForbidden,
			body:        `{"message":"quota exhausted"}`,
			wantKind:    diagnostics.QuotaExhausted,
			wantSummary: "Token quota is exhausted",
			wantFix:     "Increase the Token quota for Account dmx in the provider console",
		},
		{
			name:        "token disabled",
			status:      http.StatusForbidden,
			body:        `{"message":"token disabled"}`,
			wantKind:    diagnostics.TokenDisabled,
			wantSummary: "Token is disabled",
			wantFix:     "Enable the Token for Account dmx in the provider console",
		},
		{
			name:        "token restricted",
			status:      http.StatusForbidden,
			body:        `{"message":"forbidden"}`,
			wantKind:    diagnostics.TokenRestricted,
			wantSummary: "Token or Account is restricted",
			wantFix:     "Review access restrictions for Account dmx in the provider console",
		},
		{
			name:        "missing protocol path",
			status:      http.StatusNotFound,
			body:        `{}`,
			wantKind:    diagnostics.EndpointMismatch,
			wantSummary: "API URL or path does not match",
			wantFix:     "Check whether the configured protocol endpoint requires /v1 and whether its host is correct",
		},
		{
			name:        "upstream failure",
			status:      http.StatusInternalServerError,
			body:        `{}`,
			wantKind:    diagnostics.UpstreamFailure,
			wantSummary: "Endpoint provider failed",
			wantFix:     "Try again later; if it persists, contact the endpoint operator with the HTTP status code",
		},
		{
			name:        "unexpected response",
			status:      http.StatusTeapot,
			body:        `{}`,
			wantKind:    diagnostics.Unexpected,
			wantSummary: "Endpoint returned unexpected HTTP status 418",
			wantFix:     "Run `aigw doctor` for detailed status",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
				return response(test.status, test.body), nil
			}), runtime(), "secret", diagnostics.ScopeEndpoint)
			if result.Kind != test.wantKind || result.Summary != test.wantSummary || result.Fix != test.wantFix {
				t.Fatalf("Probe() = %#v", result)
			}
			if strings.Contains(strings.ToLower(result.Summary+result.Fix), "gateway") {
				t.Fatalf("endpoint-neutral result mentions gateway: %#v", result)
			}
		})
	}
}

func TestProbeClassifiesNetworkFailure(t *testing.T) {
	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: network unreachable")
	}), runtime(), "secret", diagnostics.ScopeEndpoint)
	if result.Kind != diagnostics.NetworkFailure || !result.Retryable {
		t.Fatalf("result = %#v", result)
	}
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
	missingModel.Model = ""
	for _, test := range []struct {
		name    string
		runtime configuration.Runtime
		scope   diagnostics.Scope
		want    diagnostics.Kind
	}{
		{name: "missing endpoint", runtime: configuration.Runtime{Client: configuration.ClientCodex}, scope: diagnostics.ScopeEndpoint, want: diagnostics.EndpointMismatch},
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
	if strings.Contains(result.Summary+result.Detail+result.Fix, secret) {
		t.Fatalf("credential leaked: %#v", result)
	}
}

type countingReadCloser struct {
	io.Reader
	closed *int
}

func (c *countingReadCloser) Close() error {
	*c.closed++
	return nil
}
