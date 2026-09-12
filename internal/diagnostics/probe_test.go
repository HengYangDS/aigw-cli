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
	return configuration.Runtime{ProfileID: "dmx", ProfileLabel: "DMXAPI", AccountID: "dmx", AccountLabel: "DMXAPI", Client: configuration.ClientCodex, Endpoint: "https://service.test/v1"}
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
		}), runtime(), "secret")
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
	}), runtime(), secret)
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

func TestProbeRedactsAnAccountTokenEchoedByTheService(t *testing.T) {
	secret := "aigw-test-account-token-never-leaks"
	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, `{"message":"rejected token aigw-test-account-token-never-leaks"}`), nil
	}), runtime(), secret)
	if strings.Contains(result.Detail, secret) {
		t.Fatalf("service response leaked Account Token: %#v", result)
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
			wantSummary: "Account Token is invalid or belongs to a different service",
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
			wantSummary: "Upstream service failed",
			wantFix:     "Try again later; if it persists, contact the service operator with the HTTP status code",
		},
		{
			name:        "unexpected response",
			status:      http.StatusTeapot,
			body:        `{}`,
			wantKind:    diagnostics.Unexpected,
			wantSummary: "Service endpoint returned unexpected HTTP status 418",
			wantFix:     "Run `aigw doctor` for detailed status",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
				return response(test.status, test.body), nil
			}), runtime(), "secret")
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
	}), runtime(), "secret")
	if result.Kind != diagnostics.NetworkFailure || !result.Retryable {
		t.Fatalf("result = %#v", result)
	}
}

func TestDefaultStabilityPolicyUsesBoundedRecovery(t *testing.T) {
	policy := diagnostics.DefaultStabilityPolicy()
	wantDelays := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second}
	if len(policy.RecoveryDelays) != len(wantDelays) {
		t.Fatalf("RecoveryDelays = %v, want %v", policy.RecoveryDelays, wantDelays)
	}
	for index, want := range wantDelays {
		if policy.RecoveryDelays[index] != want {
			t.Fatalf("RecoveryDelays[%d] = %s, want %s", index, policy.RecoveryDelays[index], want)
		}
	}
	if policy.AttemptTimeout != 5*time.Second {
		t.Fatalf("AttemptTimeout = %s, want 5s", policy.AttemptTimeout)
	}
}

func TestProbeStableRecoversOnlyAfterThreeHealthyObservations(t *testing.T) {
	client := sequenceClient(t, []probeObservation{
		{status: http.StatusUnauthorized, body: `{"message":"temporary authentication failure"}`},
		{status: http.StatusOK, body: `{"data":[]}`},
		{status: http.StatusOK, body: `{"data":[]}`},
		{status: http.StatusOK, body: `{"data":[]}`},
	})

	result := diagnostics.ProbeStable(context.Background(), client, runtime(), "secret", immediateStabilityPolicy())

	if result.Kind != diagnostics.Healthy || result.Attempts != 4 || !result.RecoveredTransient {
		t.Fatalf("result = %#v, want recovered healthy result after four attempts", result)
	}
}

func TestProbeStableConfirmsPersistentInvalidToken(t *testing.T) {
	client := sequenceClient(t, []probeObservation{
		{status: http.StatusUnauthorized, body: `{"message":"invalid api key"}`},
		{status: http.StatusUnauthorized, body: `{"message":"invalid api key"}`},
		{status: http.StatusUnauthorized, body: `{"message":"invalid api key"}`},
		{status: http.StatusUnauthorized, body: `{"message":"invalid api key"}`},
	})

	result := diagnostics.ProbeStable(context.Background(), client, runtime(), "secret", immediateStabilityPolicy())

	if result.Kind != diagnostics.InvalidToken || result.Attempts != 4 || result.RecoveredTransient {
		t.Fatalf("result = %#v, want persistent invalid token after four attempts", result)
	}
	if !strings.Contains(result.Fix, "aigw rotate") {
		t.Fatalf("persistent invalid token fix = %q, want manual rotate guidance", result.Fix)
	}
}

func TestProbeStableClassifiesMixedAuthenticationOutcomesAsUnstable(t *testing.T) {
	client := sequenceClient(t, []probeObservation{
		{status: http.StatusUnauthorized, body: `{"message":"temporary authentication failure"}`},
		{status: http.StatusOK, body: `{"data":[]}`},
		{status: http.StatusUnauthorized, body: `{"message":"temporary authentication failure"}`},
		{status: http.StatusOK, body: `{"data":[]}`},
	})

	result := diagnostics.ProbeStable(context.Background(), client, runtime(), "secret", immediateStabilityPolicy())

	if result.Kind != diagnostics.AuthenticationUnstable || result.Attempts != 4 || !result.Retryable {
		t.Fatalf("result = %#v, want retryable authentication instability", result)
	}
	if strings.Contains(result.Fix, "rotate") {
		t.Fatalf("unstable authentication must not recommend rotation: %#v", result)
	}
}

func TestProbeStableCancellationStopsRecoveryPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	client := clientFunc(func(*http.Request) (*http.Response, error) {
		calls++
		cancel()
		return response(http.StatusUnauthorized, `{"message":"temporary authentication failure"}`), nil
	})
	policy := diagnostics.StabilityPolicy{
		RecoveryDelays: []time.Duration{time.Hour, time.Hour, time.Hour},
		AttemptTimeout: time.Second,
	}
	started := time.Now()

	result := diagnostics.ProbeStable(ctx, client, runtime(), "secret", policy)

	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("ProbeStable took %s after cancellation", elapsed)
	}
	if calls != 1 || result.Attempts != 1 {
		t.Fatalf("calls=%d result=%#v, want one completed observation", calls, result)
	}
	if result.Kind != diagnostics.AuthenticationUnstable || !result.Retryable || strings.Contains(result.Fix, "rotate") {
		t.Fatalf("canceled recovery result = %#v, want retry-only authentication instability", result)
	}
}

func TestProbeStableTreatsRecoveryBodyReadTimeoutsAsAuthenticationUnstable(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			closed := 0
			client := clientFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return response(http.StatusUnauthorized, `{"message":"temporary authentication failure"}`), nil
				}
				return &http.Response{
					StatusCode: status,
					Body: &contextErrorBody{
						ctx:    req.Context(),
						closed: &closed,
					},
				}, nil
			})
			policy := diagnostics.StabilityPolicy{
				RecoveryDelays: []time.Duration{0, 0, 0},
				AttemptTimeout: 10 * time.Millisecond,
			}

			result := diagnostics.ProbeStable(context.Background(), client, runtime(), "secret", policy)

			if calls != 4 || closed != 3 {
				t.Fatalf("calls=%d closed=%d, want four calls and three closed recovery bodies", calls, closed)
			}
			if result.Kind != diagnostics.AuthenticationUnstable || result.Attempts != 4 || result.RecoveredTransient || !result.Retryable {
				t.Fatalf("result = %#v, want retryable authentication instability", result)
			}
			if strings.Contains(result.Fix, "rotate") {
				t.Fatalf("body-read timeout must not recommend rotation: %#v", result)
			}
		})
	}
}

func TestProbeStableClosesEveryResponse(t *testing.T) {
	closed := 0
	statuses := []int{http.StatusUnauthorized, http.StatusOK, http.StatusOK, http.StatusOK}
	calls := 0
	client := clientFunc(func(*http.Request) (*http.Response, error) {
		status := statuses[calls]
		calls++
		return &http.Response{
			StatusCode: status,
			Body: &countingReadCloser{
				Reader: strings.NewReader(`{"data":[]}`),
				closed: &closed,
			},
		}, nil
	})

	result := diagnostics.ProbeStable(context.Background(), client, runtime(), "secret", immediateStabilityPolicy())

	if result.Kind != diagnostics.Healthy || closed != 4 {
		t.Fatalf("result=%#v closed=%d, want four closed responses", result, closed)
	}
}

func TestProbeStableNeverReturnsCredential(t *testing.T) {
	secret := "aigw-stability-token-never-leaks"
	client := sequenceClient(t, []probeObservation{
		{status: http.StatusUnauthorized, body: `{"message":"rejected aigw-stability-token-never-leaks"}`},
		{status: http.StatusOK, body: `{"message":"accepted aigw-stability-token-never-leaks"}`},
		{status: http.StatusUnauthorized, body: `{"message":"rejected aigw-stability-token-never-leaks"}`},
		{status: http.StatusOK, body: `{"message":"accepted aigw-stability-token-never-leaks"}`},
	})

	result := diagnostics.ProbeStable(context.Background(), client, runtime(), secret, immediateStabilityPolicy())

	if strings.Contains(result.Summary+result.Detail+result.Fix, secret) {
		t.Fatalf("credential leaked: %#v", result)
	}
}

type probeObservation struct {
	status int
	body   string
}

func sequenceClient(t *testing.T, observations []probeObservation) clientFunc {
	t.Helper()
	calls := 0
	return func(*http.Request) (*http.Response, error) {
		if calls >= len(observations) {
			t.Fatalf("unexpected probe call %d", calls+1)
		}
		observation := observations[calls]
		calls++
		return response(observation.status, observation.body), nil
	}
}

func immediateStabilityPolicy() diagnostics.StabilityPolicy {
	return diagnostics.StabilityPolicy{
		RecoveryDelays: []time.Duration{0, 0, 0},
		AttemptTimeout: time.Second,
	}
}

type countingReadCloser struct {
	io.Reader
	closed *int
}

type contextErrorBody struct {
	ctx    context.Context
	closed *int
}

func (body *contextErrorBody) Read([]byte) (int, error) {
	<-body.ctx.Done()
	return 0, body.ctx.Err()
}

func (body *contextErrorBody) Close() error {
	*body.closed++
	return nil
}

func (c *countingReadCloser) Close() error {
	*c.closed++
	return nil
}
