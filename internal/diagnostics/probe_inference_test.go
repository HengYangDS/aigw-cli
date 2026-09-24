package diagnostics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/diagnostics"
)

func inferenceRuntime() configuration.Runtime {
	selected := runtime()
	selected.Protocol = configuration.ProtocolOpenAIResponses
	selected.Model = "gpt-6-sol"
	return selected
}

func TestInferenceScopeCarriesExactModelAndClassifiesDistributorRefusal(t *testing.T) {
	t.Parallel()

	calls := 0
	doer := clientFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodPost || request.URL.String() != "https://service.test/v1/responses" {
			t.Fatalf("inference request = %s %s", request.Method, request.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "gpt-6-sol" {
			t.Fatalf("model = %v", body["model"])
		}
		return response(http.StatusServiceUnavailable, `{"message":"分组 default 下模型 gpt-6-sol 无可用渠道（distributor）"}`), nil
	})
	result := diagnostics.ProbeStable(context.Background(), doer, inferenceRuntime(), "fixture-token", diagnostics.ScopeInference, immediateStabilityPolicy())
	if calls != 1 || result.Kind != diagnostics.ModelUnavailable || result.Scope != diagnostics.ScopeInference ||
		result.Attempts != 1 || !result.Retryable {
		t.Fatalf("calls=%d result=%#v", calls, result)
	}
	if strings.Contains(result.Detail, "fixture-token") {
		t.Fatal("result disclosed the account token")
	}
}

func TestInferenceScopeNeverRetriesAuthenticationFailure(t *testing.T) {
	t.Parallel()

	calls := 0
	result := diagnostics.ProbeStable(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return response(http.StatusUnauthorized, `{"message":"invalid token"}`), nil
	}), inferenceRuntime(), "fixture-token", diagnostics.ScopeInference, immediateStabilityPolicy())
	if calls != 1 || result.Kind != diagnostics.InvalidToken || result.Scope != diagnostics.ScopeInference || result.Attempts != 1 {
		t.Fatalf("calls=%d result=%#v", calls, result)
	}
}

func TestDefaultInferenceScopeHasABoundedLongerAttempt(t *testing.T) {
	t.Parallel()

	var remaining time.Duration
	result := diagnostics.ProbeStable(context.Background(), clientFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("inference request has no deadline")
		}
		remaining = time.Until(deadline)
		return response(http.StatusOK, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}]}`), nil
	}), inferenceRuntime(), "fixture-token", diagnostics.ScopeInference, diagnostics.DefaultStabilityPolicy())
	if result.Kind != diagnostics.Healthy || remaining < 55*time.Second || remaining > 61*time.Second {
		t.Fatalf("inference result=%#v remaining=%s, want a bounded 60s attempt", result, remaining)
	}
}

func TestInferenceScopeSuccessAndMissingModelHaveDistinctEvidence(t *testing.T) {
	t.Parallel()

	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`), nil
	}), inferenceRuntime(), "fixture-token", diagnostics.ScopeInference)
	if result.Kind != diagnostics.Healthy || result.Scope != diagnostics.ScopeInference || !strings.Contains(result.Summary, "Inference") {
		t.Fatalf("successful inference result = %#v", result)
	}

	missing := inferenceRuntime()
	missing.Model = ""
	calls := 0
	result = diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return response(http.StatusOK, `{"data":[]}`), nil
	}), missing, "fixture-token", diagnostics.ScopeInference)
	if calls != 0 || result.Kind != diagnostics.ModelUnresolved || result.Scope != diagnostics.ScopeInference {
		t.Fatalf("calls=%d missing-model result=%#v", calls, result)
	}
}

func TestInferenceScopeRejectsSuccessfulHTTPWithoutCompletedOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		protocol configuration.EndpointProtocol
		body     string
	}{
		{"responses incomplete", configuration.ProtocolOpenAIResponses, `{"status":"incomplete","output":[],"incomplete_details":{"reason":"max_output_tokens"}}`},
		{"responses empty", configuration.ProtocolOpenAIResponses, `{"status":"completed","output":[]}`},
		{"responses error", configuration.ProtocolOpenAIResponses, `{"error":{"message":"provider failed"}}`},
		{"anthropic empty", configuration.ProtocolAnthropic, `{"type":"message","role":"assistant","content":[]}`},
		{"chat empty", configuration.ProtocolOpenAIChatCompletions, `{"choices":[]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected := inferenceRuntime()
			selected.Protocol = test.protocol
			calls := 0
			result := diagnostics.ProbeStable(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return response(http.StatusOK, test.body), nil
			}), selected, "fixture-token", diagnostics.ScopeInference, immediateStabilityPolicy())
			if calls != 1 || result.Kind == diagnostics.Healthy || result.Scope != diagnostics.ScopeInference || result.Attempts != 1 {
				t.Fatalf("calls=%d result=%#v", calls, result)
			}
		})
	}
}

func TestInferenceScopeAcceptsTextOutputForEachProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		protocol configuration.EndpointProtocol
		body     string
	}{
		{"responses", configuration.ProtocolOpenAIResponses, `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`},
		{"anthropic", configuration.ProtocolAnthropic, `{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`},
		{"chat", configuration.ProtocolOpenAIChatCompletions, `{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected := inferenceRuntime()
			selected.Protocol = test.protocol
			result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, test.body), nil
			}), selected, "fixture-token", diagnostics.ScopeInference)
			if result.Kind != diagnostics.Healthy || result.Scope != diagnostics.ScopeInference {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}
