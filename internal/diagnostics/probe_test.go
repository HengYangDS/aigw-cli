package diagnostics_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/diagnostics"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type clientFunc func(*http.Request) (*http.Response, error)

func (f clientFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func runtime() domain.Runtime {
	return domain.Runtime{ProfileID: "dmx", ProfileLabel: "DMXAPI", AccountID: "dmx", AccountLabel: "DMXAPI", Endpoint: "https://gateway.test/v1"}
}

func TestProbeClassifiesUsefulFailureCauses(t *testing.T) {
	tests := []struct {
		status int
		body   string
		kind   diagnostics.Kind
	}{
		{401, `{"message":"invalid api key"}`, diagnostics.InvalidToken},
		{403, `{"message":"令牌额度不足"}`, diagnostics.QuotaExhausted},
		{403, `{"message":"token disabled"}`, diagnostics.TokenDisabled},
		{403, `{"message":"forbidden"}`, diagnostics.TokenRestricted},
		{429, `{"message":"too many requests"}`, diagnostics.RateLimited},
		{503, `{"message":"no available channel for model"}`, diagnostics.ModelUnavailable},
		{500, `{"message":"internal error"}`, diagnostics.GatewayFailure},
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
	if result.Kind != diagnostics.Healthy || requestURL != "https://gateway.test/v1/models" || authorization != "Bearer "+secret {
		t.Fatalf("result=%#v url=%q auth=%q", result, requestURL, authorization)
	}
	if strings.Contains(result.Summary+result.Detail+result.Fix, secret) {
		t.Fatalf("credential leaked: %#v", result)
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
