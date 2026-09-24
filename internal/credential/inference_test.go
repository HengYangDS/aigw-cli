package credential

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
)

func TestModelInferenceRequestUsesExactWireModelAndProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		endpoint   string
		protocol   configuration.EndpointProtocol
		model      string
		wantURL    string
		wantHeader string
		wantValue  string
		wantBody   map[string]any
	}{
		{
			name:       "anthropic messages",
			endpoint:   "https://anthropic.example/v1/",
			protocol:   configuration.ProtocolAnthropic,
			model:      "claude-opus-5-5[1m]",
			wantURL:    "https://anthropic.example/v1/messages",
			wantHeader: "X-Api-Key",
			wantValue:  "fixture-token",
			wantBody: map[string]any{
				"model":      "claude-opus-5-5[1m]",
				"max_tokens": float64(1),
				"messages":   []any{map[string]any{"role": "user", "content": "ping"}},
			},
		},
		{
			name:       "openai responses",
			endpoint:   "https://responses.example/v1",
			protocol:   configuration.ProtocolOpenAIResponses,
			model:      "gpt-6-sol",
			wantURL:    "https://responses.example/v1/responses",
			wantHeader: "Authorization",
			wantValue:  "Bearer fixture-token",
			wantBody: map[string]any{
				"model":             "gpt-6-sol",
				"input":             "ping",
				"max_output_tokens": float64(16),
				"store":             false,
			},
		},
		{
			name:       "openai chat completions",
			endpoint:   "https://chat.example/v1/",
			protocol:   configuration.ProtocolOpenAIChatCompletions,
			model:      "chat-model",
			wantURL:    "https://chat.example/v1/chat/completions",
			wantHeader: "Authorization",
			wantValue:  "Bearer fixture-token",
			wantBody: map[string]any{
				"model":      "chat-model",
				"max_tokens": float64(1),
				"messages":   []any{map[string]any{"role": "user", "content": "ping"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, err := ModelInferenceRequest(context.Background(), test.endpoint, test.protocol, "fixture-token", test.model)
			if err != nil {
				t.Fatal(err)
			}
			if request.Method != http.MethodPost || request.URL.String() != test.wantURL {
				t.Fatalf("method/url = %s %s, want POST %s", request.Method, request.URL, test.wantURL)
			}
			if request.Header.Get(test.wantHeader) != test.wantValue {
				t.Fatalf("missing %s authentication header", test.wantHeader)
			}
			if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("content negotiation headers are missing")
			}
			if test.protocol == configuration.ProtocolAnthropic && request.Header.Get("Anthropic-Version") != "2023-06-01" {
				t.Fatal("Anthropic version header is missing")
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body, test.wantBody) {
				t.Fatalf("request body = %#v, want %#v", body, test.wantBody)
			}
		})
	}
}

func TestModelInferenceRequestRejectsMissingModelAndUnknownProtocol(t *testing.T) {
	t.Parallel()

	_, err := ModelInferenceRequest(context.Background(), "https://example.test/v1", configuration.ProtocolOpenAIResponses, "fixture-token", "  ")
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("empty model error = %v", err)
	}
	_, err = ModelInferenceRequest(context.Background(), "https://example.test/v1", configuration.EndpointProtocol("unknown"), "fixture-token", "model")
	if err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("unknown protocol error = %v", err)
	}
}
