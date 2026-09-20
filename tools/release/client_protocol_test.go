//go:build client_acceptance

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/configuration"
)

func TestNativeClientStreamEnvelope(t *testing.T) {
	for _, protocol := range []configuration.EndpointProtocol{
		configuration.ProtocolAnthropic,
		configuration.ProtocolOpenAIResponses,
		configuration.ProtocolOpenAIChatCompletions,
	} {
		t.Run(string(protocol), func(t *testing.T) {
			var completions atomic.Int64
			path, body := streamRequest(protocol, "configured-model")
			var response *httptest.ResponseRecorder
			for _, test := range []struct {
				method, path, credential, body string
				status                         int
				completed                      int64
			}{
				{http.MethodPost, path, "", `{"stream":true}`, http.StatusUnauthorized, 0},
				{http.MethodPost, "/wrong", "synthetic", `{"stream":true}`, http.StatusNotFound, 0},
				{http.MethodGet, path, "synthetic", `{"stream":true}`, http.StatusMethodNotAllowed, 0},
				{http.MethodPost, path, "synthetic", `{"stream":false}`, http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", `{"stream":true}`, http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", strings.ReplaceAll(body, "high", "low"), http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", strings.ReplaceAll(body, "configured-model", "different-model"), http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", body, http.StatusOK, 1},
			} {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Authorization", "Bearer "+test.credential)
				response = httptest.NewRecorder()
				clientResponseHandler(protocol, "configured-model", "synthetic", &completions).ServeHTTP(response, request)
				if response.Code != test.status || completions.Load() != test.completed {
					t.Fatalf("%s %s: status=%d completions=%d", test.method, test.path, response.Code, completions.Load())
				}
			}
			assertStreamEvents(t, protocol, response.Body.String())
		})
	}
}

func assertStreamEvents(t *testing.T, protocol configuration.EndpointProtocol, body string) {
	t.Helper()
	for _, data := range clientResponseEvents(protocol, "configured-model") {
		if protocol == configuration.ProtocolOpenAIChatCompletions {
			if !strings.Contains(body, "data: "+data+"\n\n") {
				t.Fatalf("missing Chat Completions event: %s", body)
			}
			continue
		}
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body, "event: "+event.Type+"\ndata: "+data+"\n\n") {
			t.Fatalf("missing named %s event: %s", event.Type, body)
		}
	}
}

func clientResponseHandler(protocol configuration.EndpointProtocol, model, token string, completions *atomic.Int64) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":[],"models":[],"has_more":false}`)
	})
	path, _ := streamRequest(protocol, model)
	mux.HandleFunc("POST "+path, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
			OutputConfig struct {
				Effort string `json:"effort"`
			} `json:"output_config"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || !input.Stream || input.Model != model {
			http.Error(response, "configured model and stream required", http.StatusBadRequest)
			return
		}
		if streamEffort(protocol, input.Reasoning.Effort, input.OutputConfig.Effort, input.ReasoningEffort) != "high" {
			http.Error(response, "configured high effort required", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		for _, data := range clientResponseEvents(protocol, model) {
			if protocol == configuration.ProtocolOpenAIChatCompletions {
				if _, err := fmt.Fprintf(response, "data: %s\n\n", data); err != nil {
					return
				}
				continue
			}
			var event struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				http.Error(response, "invalid fixture event", http.StatusInternalServerError)
				return
			}
			if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
				return
			}
		}
		completions.Add(1)
	})
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token && request.Header.Get("X-Api-Key") != token {
			http.Error(response, "credential mismatch", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(response, request)
	})
}

func streamEffort(protocol configuration.EndpointProtocol, responses, messages, chat string) string {
	switch protocol {
	case configuration.ProtocolAnthropic:
		return messages
	case configuration.ProtocolOpenAIResponses:
		return responses
	case configuration.ProtocolOpenAIChatCompletions:
		return chat
	default:
		return ""
	}
}

func streamRequest(protocol configuration.EndpointProtocol, model string) (string, string) {
	switch protocol {
	case configuration.ProtocolAnthropic:
		return "/v1/messages", fmt.Sprintf(`{"model":%q,"stream":true,"output_config":{"effort":"high"}}`, model)
	case configuration.ProtocolOpenAIResponses:
		return "/v1/responses", fmt.Sprintf(`{"model":%q,"stream":true,"reasoning":{"effort":"high"}}`, model)
	case configuration.ProtocolOpenAIChatCompletions:
		return "/v1/chat/completions", fmt.Sprintf(`{"model":%q,"stream":true,"reasoning_effort":"high"}`, model)
	default:
		return "", ""
	}
}

func clientResponseEvents(protocol configuration.EndpointProtocol, model string) []string {
	if protocol == configuration.ProtocolAnthropic {
		return []string{
			fmt.Sprintf(`{"type":"message_start","message":{"id":"msg_fixture","type":"message","role":"assistant","model":%q,"content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}`, model),
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"AIGW_OK"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}`,
			`{"type":"message_stop"}`,
		}
	}
	if protocol == configuration.ProtocolOpenAIChatCompletions {
		return []string{
			fmt.Sprintf(`{"id":"chatcmpl_fixture","object":"chat.completion.chunk","model":%q,"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`, model),
			fmt.Sprintf(`{"id":"chatcmpl_fixture","object":"chat.completion.chunk","model":%q,"choices":[{"index":0,"delta":{"content":"AIGW_OK"},"finish_reason":null}]}`, model),
			fmt.Sprintf(`{"id":"chatcmpl_fixture","object":"chat.completion.chunk","model":%q,"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`, model),
			"[DONE]",
		}
	}
	return []string{
		`{"type":"response.created","response":{"id":"resp_fixture","object":"response","status":"in_progress","output":[]}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_fixture","type":"message","role":"assistant","status":"in_progress","content":[]}}`,
		`{"type":"response.content_part.added","item_id":"msg_fixture","output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`,
		`{"type":"response.output_text.delta","item_id":"msg_fixture","output_index":0,"content_index":0,"delta":"AIGW_OK"}`,
		`{"type":"response.output_text.done","item_id":"msg_fixture","output_index":0,"content_index":0,"text":"AIGW_OK"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"AIGW_OK","annotations":[]}]}}`,
		`{"type":"response.completed","response":{"id":"resp_fixture","object":"response","status":"completed","output":[{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"AIGW_OK","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
	}
}
