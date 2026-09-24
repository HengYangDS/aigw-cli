package diagnostics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

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
		return response(http.StatusServiceUnavailable, `{"message":"group default has no available channel (distributor) for model gpt-6-sol"}`), nil
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

func TestInferenceScopeSuccessAndMissingModelHaveDistinctEvidence(t *testing.T) {
	t.Parallel()

	result := diagnostics.Probe(context.Background(), clientFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"id":"response"}`), nil
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
