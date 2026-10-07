//go:build client_acceptance

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
)

type responsesToolDefinition struct {
	Type  string                    `json:"type"`
	Name  string                    `json:"name"`
	Tools []responsesToolDefinition `json:"tools"`
}

type responsesToolLoopItem struct {
	Type   string          `json:"type"`
	CallID string          `json:"call_id"`
	Output json.RawMessage `json:"output"`
}

type clientInferenceRequest struct {
	Model           string          `json:"model"`
	Stream          bool            `json:"stream"`
	Input           json.RawMessage `json:"input"`
	MaxOutputTokens int             `json:"max_output_tokens"`
	MaxTokens       int             `json:"max_tokens"`
	Store           bool            `json:"store"`
	Messages        json.RawMessage `json:"messages"`
	Reasoning       struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	OutputConfig struct {
		Effort string `json:"effort"`
	} `json:"output_config"`
	ReasoningEffort string                    `json:"reasoning_effort"`
	Tools           []responsesToolDefinition `json:"tools"`
}

func responsesToolLoopInputCount(input json.RawMessage) int {
	var items []json.RawMessage
	if json.Unmarshal(input, &items) == nil {
		return len(items)
	}
	var text string
	if json.Unmarshal(input, &text) == nil && text == "" {
		return 0
	}
	return 1
}

func responsesToolLoopOutputs(input json.RawMessage) ([]responsesToolLoopItem, error) {
	var items []responsesToolLoopItem
	if err := json.Unmarshal(input, &items); err != nil {
		var text string
		if json.Unmarshal(input, &text) == nil {
			return nil, nil
		}
		return nil, err
	}
	return slices.DeleteFunc(items, func(item responsesToolLoopItem) bool {
		return item.Type != "function_call_output" && item.Type != "custom_tool_call_output"
	}), nil
}

func codexChallengeReply(input clientInferenceRequest) (string, error) {
	outputs, err := responsesToolLoopOutputs(input.Input)
	if err != nil {
		return "", fmt.Errorf("invalid native challenge input")
	}
	for _, item := range outputs {
		if item.CallID != "call_aigw_challenge" {
			continue
		}
		var output string
		if json.Unmarshal(item.Output, &output) != nil {
			var content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(item.Output, &content); err != nil {
				return "", fmt.Errorf("native challenge tool output is malformed")
			}
			for _, part := range content {
				if part.Type != "input_text" {
					return "", fmt.Errorf("native challenge tool output is not text")
				}
				output += part.Text
			}
		}
		if !strings.Contains(output, "Process exited with code 0") {
			return "", fmt.Errorf("native challenge tool did not complete successfully")
		}
		fields := strings.Fields(output)
		marker := fields[len(fields)-1]
		if decoded, err := hex.DecodeString(marker); err != nil || len(decoded) != 24 {
			return "", fmt.Errorf("native challenge tool did not return the file contents")
		}
		return marker, nil
	}
	return "", nil
}

func (input clientInferenceRequest) codexCommandTool() (custom bool, err error) {
	for _, tool := range input.Tools {
		if tool.Type == "function" && tool.Name == "exec_command" {
			return false, nil
		}
	}
	var items []struct {
		Type  string                    `json:"type"`
		Role  string                    `json:"role"`
		Tools []responsesToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(input.Input, &items); err != nil {
		return false, err
	}
	for _, item := range items {
		if item.Type != "additional_tools" || item.Role != "developer" {
			continue
		}
		for _, namespace := range item.Tools {
			if namespace.Type != "namespace" || namespace.Name != "functions" {
				continue
			}
			for _, tool := range namespace.Tools {
				if tool.Type == "custom" && tool.Name == "exec" {
					return true, nil
				}
			}
		}
	}
	return false, fmt.Errorf("native challenge requires the client's declared command tool")
}

func newNativeClientServer(t *testing.T, client string, protocol configuration.EndpointProtocol, model, token string, completions *atomic.Int64) (*httptest.Server, *hermesSessionRecorder) {
	t.Helper()
	requiredEffort := "high"
	if client == configuration.ClientHermes {
		requiredEffort = ""
	}
	shell := ""
	if client == configuration.ClientCodex {
		shell = nativeCodexToolShell(t)
	}
	handler := clientResponseHandler(protocol, map[string]*atomic.Int64{model: completions}, token, requiredEffort, shell)
	var hermesSession *hermesSessionRecorder
	if client == configuration.ClientHermes {
		hermesSession = &hermesSessionRecorder{Handler: handler, model: model, token: token}
		handler = hermesSession
	}
	requests := map[string]int{}
	var requestsMu sync.Mutex
	started := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		t.Logf("client request after %s: %s %s", time.Since(started).Round(time.Millisecond), request.Method, request.URL.Path)
		requestsMu.Lock()
		requests[request.Method+" "+request.URL.Path]++
		requestsMu.Unlock()
		handler.ServeHTTP(response, request)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		if t.Failed() {
			requestsMu.Lock()
			defer requestsMu.Unlock()
			t.Logf("client request paths: %v", requests)
		}
	})
	return server, hermesSession
}

func TestNativeClientStreamEnvelope(t *testing.T) {
	for _, protocol := range []configuration.EndpointProtocol{
		configuration.ProtocolAnthropic,
		configuration.ProtocolOpenAIResponses,
		configuration.ProtocolOpenAIChatCompletions,
	} {
		t.Run(string(protocol), func(t *testing.T) {
			var completions atomic.Int64
			expected := map[string]*atomic.Int64{"configured-model": &completions}
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
				{http.MethodPost, path, "synthetic", `{"model":"configured-model","stream":false}`, http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", `{"stream":true}`, http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", strings.ReplaceAll(body, "high", "low"), http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", strings.ReplaceAll(body, "configured-model", "different-model"), http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", body, http.StatusOK, 1},
			} {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Authorization", "Bearer "+test.credential)
				response = httptest.NewRecorder()
				clientResponseHandler(protocol, expected, "synthetic", "high", "").ServeHTTP(response, request)
				if response.Code != test.status || completions.Load() != test.completed {
					t.Fatalf("%s %s: status=%d completions=%d", test.method, test.path, response.Code, completions.Load())
				}
			}
			assertStreamEvents(t, protocol, response.Body.String())
		})
	}
}

func TestNativeClientInferenceEnvelope(t *testing.T) {
	for _, protocol := range []configuration.EndpointProtocol{
		configuration.ProtocolAnthropic,
		configuration.ProtocolOpenAIResponses,
		configuration.ProtocolOpenAIChatCompletions,
	} {
		t.Run(string(protocol), func(t *testing.T) {
			request, err := credential.ModelInferenceRequest(t.Context(), "https://fixture.test/v1", protocol, "synthetic", "configured-model")
			if err != nil {
				t.Fatal(err)
			}
			var completions atomic.Int64
			response := httptest.NewRecorder()
			recorder := hermesSessionRecorder{
				Handler: clientResponseHandler(protocol, map[string]*atomic.Int64{"configured-model": &completions}, "synthetic", "high", ""),
				model:   "configured-model",
			}
			recorder.ServeHTTP(response, request)
			if response.Code != http.StatusOK || completions.Load() != 1 || response.Header().Get("Content-Type") != "application/json" || !json.Valid(response.Body.Bytes()) || !strings.Contains(response.Body.String(), `"pong"`) {
				t.Fatalf("non-stream inference response: status=%d completions=%d content-type=%q body=%s", response.Code, completions.Load(), response.Header().Get("Content-Type"), response.Body.String())
			}
			if len(recorder.inputCounts()) != 0 {
				t.Fatal("non-stream inference was recorded as a native client session turn")
			}
		})
	}
}

func assertStreamEvents(t *testing.T, protocol configuration.EndpointProtocol, body string) {
	t.Helper()
	for _, data := range clientResponseEvents(protocol, "configured-model", "AIGW_OK") {
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

func clientResponseHandler(protocol configuration.EndpointProtocol, completions map[string]*atomic.Int64, token, requiredEffort, shell string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":[],"models":[],"has_more":false}`)
	})
	path, _ := streamRequest(protocol, "")
	mux.HandleFunc("POST "+path, func(response http.ResponseWriter, request *http.Request) {
		var input clientInferenceRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			http.Error(response, "invalid model request", http.StatusBadRequest)
			return
		}
		completion, expectedModel := completions[input.Model]
		if !expectedModel {
			http.Error(response, "configured model required", http.StatusBadRequest)
			return
		}
		if !input.Stream {
			var body string
			switch protocol {
			case configuration.ProtocolAnthropic:
				body = `{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`
			case configuration.ProtocolOpenAIResponses:
				body = `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`
			case configuration.ProtocolOpenAIChatCompletions:
				body = `{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`
			}
			valid := protocol == configuration.ProtocolOpenAIResponses && len(input.Input) > 0 && input.MaxOutputTokens > 0 && !input.Store ||
				protocol != configuration.ProtocolOpenAIResponses && len(input.Messages) > 0 && input.MaxTokens > 0
			if !valid || body == "" {
				http.Error(response, "configured inference request required", http.StatusBadRequest)
				return
			}
			response.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(response, body); err == nil {
				completion.Add(1)
			}
			return
		}
		if requiredEffort != "" && streamEffort(protocol, input.Reasoning.Effort, input.OutputConfig.Effort, input.ReasoningEffort) != requiredEffort {
			http.Error(response, "configured effort required", http.StatusBadRequest)
			return
		}
		if err := input.writeStream(response, protocol, shell); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		completion.Add(1)
	})
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !clientFixtureAuthorized(request, token) {
			http.Error(response, "credential mismatch", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(response, request)
	})
}

func (input clientInferenceRequest) writeStream(response http.ResponseWriter, protocol configuration.EndpointProtocol, shell string) error {
	text := "AIGW_OK"
	if protocol == configuration.ProtocolOpenAIResponses && strings.Contains(string(input.Input), "challenge.txt") {
		var err error
		text, err = codexChallengeReply(input)
		if err != nil {
			return err
		}
	}
	if text == "" {
		custom, err := input.codexCommandTool()
		if err != nil {
			return err
		}
		command := map[string]any{"cmd": "cat challenge.txt"}
		if shell != "" {
			command["cmd"], command["shell"], command["login"] = "type challenge.txt", shell, false
		}
		arguments, err := json.Marshal(command)
		if err != nil {
			return err
		}
		return writeResponsesToolCall(response, "resp_aigw_challenge", "fc_aigw_challenge", "call_aigw_challenge", "exec_command", string(arguments), custom)
	}
	return writeNativeResponseEvents(response, protocol, clientResponseEvents(protocol, input.Model, text))
}

func writeNativeResponseEvents(response http.ResponseWriter, protocol configuration.EndpointProtocol, events []string) error {
	response.Header().Set("Content-Type", "text/event-stream")
	for _, data := range events {
		if protocol == configuration.ProtocolOpenAIChatCompletions {
			if _, err := fmt.Fprintf(response, "data: %s\n\n", data); err != nil {
				return err
			}
			continue
		}
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("invalid fixture event")
		}
		if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
			return err
		}
	}
	return nil
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
		return "/v1/responses", fmt.Sprintf(`{"model":%q,"stream":true,"reasoning":{"effort":"high"},"input":[{"role":"user","content":[{"type":"input_text","text":"ping"}]}]}`, model)
	case configuration.ProtocolOpenAIChatCompletions:
		return "/v1/chat/completions", fmt.Sprintf(`{"model":%q,"stream":true,"reasoning_effort":"high"}`, model)
	default:
		return "", ""
	}
}

func clientResponseEvents(protocol configuration.EndpointProtocol, model, text string) []string {
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
		fmt.Sprintf(`{"type":"response.output_text.delta","item_id":"msg_fixture","output_index":0,"content_index":0,"delta":%q}`, text),
		fmt.Sprintf(`{"type":"response.output_text.done","item_id":"msg_fixture","output_index":0,"content_index":0,"text":%q}`, text),
		fmt.Sprintf(`{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":%q,"annotations":[]}]}}`, text),
		fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_fixture","object":"response","status":"completed","output":[{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":%q,"annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`, text),
	}
}

func clientFixtureAuthorized(request *http.Request, token string) bool {
	return request.Header.Get("Authorization") == "Bearer "+token || request.Header.Get("X-Api-Key") == token
}

func TestResponsesToolLoopOutputsAcceptsTextInput(t *testing.T) {
	outputs, err := responsesToolLoopOutputs(json.RawMessage(`"reply briefly"`))
	if err != nil {
		t.Fatalf("valid Responses text input rejected: %v", err)
	}
	if len(outputs) != 0 {
		t.Fatalf("Responses text input produced tool outputs: %v", outputs)
	}
}

func TestNativeCodexChallengeStreamRequiresToolOutputAndRecall(t *testing.T) {
	const marker = "0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, client := range []struct{ name, tools, declaration, call, outputType string }{
		{"function", `[{"type":"function","name":"exec_command"}]`, "", `"name":"exec_command"`, "function_call_output"},
		{"native custom", `[]`, `{"type":"additional_tools","role":"developer","tools":[{"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec"}]}]},`, `"namespace":"functions"`, "custom_tool_call_output"},
	} {
		t.Run(client.name, func(t *testing.T) {
			var completed atomic.Int64
			handler := clientResponseHandler(configuration.ProtocolOpenAIResponses, map[string]*atomic.Int64{"configured-model": &completed}, "synthetic", "high", "")
			challenge := client.declaration + `{"role":"user","content":"Use a shell tool to read challenge.txt"}`
			result := `"Process exited with code 0\nFinal output:\n` + marker + `"`
			if client.outputType == "custom_tool_call_output" {
				result = `[{"type":"input_text","text":"Script completed\nWall time 0.1 seconds\nOutput:\n"},{"type":"input_text","text":` + result + `}]`
			}
			output := `,{"type":"` + client.outputType + `","call_id":"call_aigw_challenge","output":` + result + `}`
			for index, input := range []string{
				`[` + challenge + `]`,
				`[` + challenge + output + `]`,
				`[` + challenge + output + `,{"role":"assistant","content":"` + marker + `"},{"role":"user","content":"Recall the previous turn without using tools"}]`,
			} {
				body := `{"model":"configured-model","stream":true,"reasoning":{"effort":"high"},"tools":` + client.tools + `,"input":` + input + `}`
				request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
				request.Header.Set("Authorization", "Bearer synthetic")
				response := httptest.NewRecorder()
				if index == 0 {
					handler.ServeHTTP(failedNativeResponseWriter{httptest.NewRecorder()}, request)
					if completed.Load() != 0 {
						t.Fatal("failed native stream counted as a completed tool call")
					}
					request.Body = io.NopCloser(strings.NewReader(body))
				}
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					t.Fatalf("native challenge request %d failed: %d", index, response.Code)
				}
				if index == 0 {
					if !strings.Contains(response.Body.String(), client.call) || strings.Contains(response.Body.String(), marker) {
						t.Fatal("the first turn did not require an actual challenge file tool read")
					}
				} else if !strings.Contains(response.Body.String(), marker) || strings.Contains(response.Body.String(), `"type":"function_call"`) || strings.Contains(response.Body.String(), `"type":"custom_tool_call"`) {
					t.Fatal("native tool output or its same-session recall was not preserved")
				}
			}
			if completed.Load() != 3 {
				t.Fatalf("native challenge completion count = %d, want tool call, final text and recalled text", completed.Load())
			}
		})
	}
}

type failedNativeResponseWriter struct{ *httptest.ResponseRecorder }

func (failedNativeResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func writeResponsesToolCall(response http.ResponseWriter, responseID, itemID, callID, name, input string, custom bool) error {
	kind, field, deltaEvent := "function_call", "arguments", "response.function_call_arguments"
	namespace := ""
	if custom {
		kind, field, deltaEvent = "custom_tool_call", "input", "response.custom_tool_call_input"
		name = "exec"
		input = `const result = await tools.exec_command(` + input + `); if (result.exit_code !== 0) throw new Error("Native command failed"); text("Process exited with code 0\nFinal output:\n" + result.output);`
		namespace = `,"namespace":"functions"`
	}
	added := fmt.Sprintf(`{"id":%q,"type":%q,"call_id":%q,"name":%q%s,%q:"","status":"in_progress"}`, itemID, kind, callID, name, namespace, field)
	completed := fmt.Sprintf(`{"id":%q,"type":%q,"call_id":%q,"name":%q%s,%q:%q,"status":"completed"}`, itemID, kind, callID, name, namespace, field, input)
	events := []string{
		fmt.Sprintf(`{"type":"response.created","response":{"id":%q,"object":"response","status":"in_progress","output":[]}}`, responseID),
		fmt.Sprintf(`{"type":"response.output_item.added","output_index":0,"item":%s}`, added),
		fmt.Sprintf(`{"type":%q,"item_id":%q,"output_index":0,"delta":%q}`, deltaEvent+".delta", itemID, input),
		fmt.Sprintf(`{"type":%q,"item_id":%q,"output_index":0,%q:%q}`, deltaEvent+".done", itemID, field, input),
		fmt.Sprintf(`{"type":"response.output_item.done","output_index":0,"item":%s}`, completed),
		fmt.Sprintf(`{"type":"response.completed","response":{"id":%q,"object":"response","status":"completed","output":[%s],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`, responseID, completed),
	}
	return writeNativeResponseEvents(response, configuration.ProtocolOpenAIResponses, events)
}
