package diagnostics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/diagnostics"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestProbeStableUsesRealHTTPRecoveryBoundary(t *testing.T) {
	t.Run("immediate success", func(t *testing.T) {
		const secret = "immediate-secret"
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			calls.Add(1)
			if request.Method != http.MethodGet || request.URL.Path != "/v1/models" || request.Header.Get("Authorization") != "Bearer "+secret {
				t.Errorf("request = %s %s, Authorization %q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
			}
			writer.WriteHeader(http.StatusOK)
			if _, err := writer.Write([]byte(`{"status":"ok"}`)); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		defer server.Close()

		result := diagnostics.ProbeStable(context.Background(), server.Client(), domain.Runtime{Endpoint: server.URL + "/v1"}, secret, diagnostics.StabilityPolicy{
			RecoveryDelays: []time.Duration{time.Millisecond},
			AttemptTimeout: time.Second,
		})
		if result.Kind != diagnostics.Healthy || result.Attempts != 1 || result.RecoveredTransient || calls.Load() != 1 {
			t.Fatalf("result = %#v, calls = %d", result, calls.Load())
		}
	})

	t.Run("bounded timed recovery", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 1 {
				writer.WriteHeader(http.StatusUnauthorized)
				if _, err := writer.Write([]byte(`{"message":"temporary authentication failure"}`)); err != nil {
					t.Errorf("write response: %v", err)
				}
				return
			}
			writer.WriteHeader(http.StatusOK)
			if _, err := writer.Write([]byte(`{"status":"ok"}`)); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		defer server.Close()

		result := diagnostics.ProbeStable(context.Background(), server.Client(), domain.Runtime{Endpoint: server.URL + "/v1"}, "secret", diagnostics.StabilityPolicy{
			RecoveryDelays: []time.Duration{time.Millisecond, 2 * time.Millisecond},
			AttemptTimeout: time.Second,
		})
		if result.Kind != diagnostics.Healthy || result.Attempts != 3 || !result.RecoveredTransient || calls.Load() != 3 {
			t.Fatalf("result = %#v, calls = %d", result, calls.Load())
		}
	})
}

func TestProbeClassifiesRealHTTPResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		kind   diagnostics.Kind
	}{
		{name: "quota exhausted", status: http.StatusForbidden, body: "quota exhausted", kind: diagnostics.QuotaExhausted},
		{name: "token disabled", status: http.StatusForbidden, body: "token disabled", kind: diagnostics.TokenDisabled},
		{name: "token restricted", status: http.StatusForbidden, body: "forbidden", kind: diagnostics.TokenRestricted},
		{name: "rate limited", status: http.StatusTooManyRequests, body: "rate limited", kind: diagnostics.RateLimited},
		{name: "endpoint mismatch", status: http.StatusNotFound, body: "not found", kind: diagnostics.EndpointMismatch},
		{name: "model unavailable", status: http.StatusServiceUnavailable, body: "model unavailable", kind: diagnostics.ModelUnavailable},
		{name: "gateway failure", status: http.StatusInternalServerError, body: "gateway failed", kind: diagnostics.GatewayFailure},
		{name: "unexpected", status: http.StatusTeapot, body: "teapot", kind: diagnostics.Unexpected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(tt.status)
				if _, err := writer.Write([]byte(tt.body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			result := diagnostics.Probe(context.Background(), server.Client(), domain.Runtime{Endpoint: server.URL}, "secret")
			if result.Kind != tt.kind || result.HTTPStatus != tt.status || result.Summary == "" || result.Fix == "" {
				t.Fatalf("Probe() = %#v", result)
			}
		})
	}
}

func TestProbeRejectsEmptyEndpointBeforeHTTP(t *testing.T) {
	result := diagnostics.Probe(context.Background(), http.DefaultClient, domain.Runtime{}, "secret")
	if result.Kind != diagnostics.EndpointMismatch || result.HTTPStatus != 0 || result.Summary != "Invalid API URL" || result.Fix == "" {
		t.Fatalf("Probe() = %#v", result)
	}
}

func TestProbeReportsTruncatedHTTPResponse(t *testing.T) {
	const secret = "truncated-response-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			t.Error("test server does not support connection hijacking")
			return
		}
		connection, stream, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack connection: %v", err)
			return
		}
		defer func() { _ = connection.Close() }()
		if _, err := stream.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100\r\nContent-Type: text/plain\r\n\r\npartial"); err != nil {
			t.Errorf("write truncated response: %v", err)
			return
		}
		if err := stream.Flush(); err != nil {
			t.Errorf("flush truncated response: %v", err)
		}
	}))
	defer server.Close()

	result := diagnostics.Probe(context.Background(), server.Client(), domain.Runtime{Endpoint: server.URL}, secret)
	if result.Kind != diagnostics.NetworkFailure || result.HTTPStatus != http.StatusOK || !result.Retryable || !strings.Contains(result.Detail, "unexpected EOF") {
		t.Fatalf("Probe() = %#v", result)
	}
	if strings.Contains(result.Detail, secret) {
		t.Fatalf("read error leaked credential: %#v", result)
	}
}
