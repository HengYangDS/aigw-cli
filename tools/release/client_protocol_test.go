//go:build client_acceptance

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/redaction"
	"aigw-cli/internal/secrets"
)

func newNativeClientServer(t *testing.T, client string, protocol configuration.EndpointProtocol, model, token string, completions *atomic.Int64) (*httptest.Server, *hermesSessionRecorder) {
	t.Helper()
	requiredEffort := "high"
	if client == configuration.ClientHermes {
		requiredEffort = ""
	}
	handler := clientResponseHandler(protocol, map[string]*atomic.Int64{model: completions}, token, requiredEffort)
	var hermesSession *hermesSessionRecorder
	if client == configuration.ClientHermes {
		hermesSession = &hermesSessionRecorder{Handler: handler, model: model}
		handler = hermesSession
	}
	requests := map[string]int{}
	var requestsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
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
				clientResponseHandler(protocol, expected, "synthetic", "high").ServeHTTP(response, request)
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
			clientResponseHandler(protocol, map[string]*atomic.Int64{"configured-model": &completions}, "synthetic", "high").ServeHTTP(response, request)
			if response.Code != http.StatusOK || completions.Load() != 1 || response.Header().Get("Content-Type") != "application/json" || !json.Valid(response.Body.Bytes()) || !strings.Contains(response.Body.String(), `"pong"`) {
				t.Fatalf("non-stream inference response: status=%d completions=%d content-type=%q body=%s", response.Code, completions.Load(), response.Header().Get("Content-Type"), response.Body.String())
			}
		})
	}
}

func (p nativeClientJourneyPlan) runCodexGeneralRoutes(t *testing.T) {
	t.Helper()
	const (
		account = "aihubmix"
		token   = "native-general-route-token"
	)
	routeIDs := make([]string, 0, len(p.manifest.Routes))
	completions := map[string]*atomic.Int64{}
	for routeID, route := range p.manifest.Routes {
		if route.Account != account || !slices.Contains(route.AdmittedProtocols(), configuration.ProtocolOpenAIResponses) {
			continue
		}
		if _, duplicate := completions[route.UpstreamModel]; duplicate {
			t.Fatalf("AIHubMix Routes reuse upstream model %q", route.UpstreamModel)
		}
		routeIDs = append(routeIDs, routeID)
		completions[route.UpstreamModel] = &atomic.Int64{}
	}
	if len(routeIDs) == 0 {
		t.Fatal("team manifest has no AIHubMix OpenAI Responses Routes")
	}
	slices.Sort(routeIDs)
	server := httptest.NewServer(clientResponseHandler(configuration.ProtocolOpenAIResponses, completions, token, "high"))
	t.Cleanup(server.Close)
	executable, err := requiredClientInput("AIGW_ACCEPTANCE_CODEX", false)
	if err != nil {
		t.Fatal(err)
	}
	journey := newNativeJourney(t, p.candidate, server.URL+"/v1", false)
	journey.prepareNativeClient(configuration.ClientCodex, executable, p.team)
	journey.isolateNativeClientManifest(configuration.ClientCodex)
	journey.setEnvironment(secrets.EnvironmentKey(account), token)
	journey.run("setup", "--from", journey.manifest)
	journey.run("use", "--for", configuration.ClientCodex, account+"-gpt-6-astra")
	journey.enableNativeClient(configuration.ClientCodex, executable)
	selected := readFile(t, journey.config)
	for _, routeID := range routeIDs {
		route := p.manifest.Routes[routeID]
		t.Run(routeID, func(t *testing.T) {
			before := completions[route.UpstreamModel].Load()
			journey.testing = t
			journey.run("verify", "--for", configuration.ClientCodex, "--route", routeID)
			if completions[route.UpstreamModel].Load() != before+1 {
				t.Fatalf("Codex did not complete exactly one request for upstream model %q", route.UpstreamModel)
			}
			if !slices.Equal(readFile(t, journey.config), selected) {
				t.Fatal("explicit Route verification changed the selected client binding")
			}
		})
	}
	journey.testing = t
	journey.uninstallWithAndRequireInstallationRemoved(p.candidate)
}

func (p nativeClientJourneyPlan) runCodexToolLoop(t *testing.T) {
	t.Helper()
	const (
		account = "aihubmix"
		routeID = "aihubmix-grok-4.7"
		token   = "native-tool-loop-token"
	)
	executable, err := requiredClientInput("AIGW_ACCEPTANCE_CODEX", false)
	if err != nil {
		t.Fatal(err)
	}
	var toolOutput atomic.Bool
	var probe codexToolLoopProbe
	server := httptest.NewServer(codexToolLoopHandler(token, &toolOutput, &probe))
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Codex tool-loop requests=%d calls=%d results=%d rejected=%d",
				probe.requests.Load(), probe.toolCalls.Load(), probe.toolResults.Load(), probe.rejected.Load())
			if first := probe.firstResult.Load(); first != nil {
				t.Logf("first bounded tool result: %q", *first)
			}
		}
	})
	t.Cleanup(server.Close)
	journey := newNativeJourney(t, p.candidate, server.URL+"/v1", false)
	journey.prepareNativeClient(configuration.ClientCodex, executable, p.team)
	journey.isolateNativeClientManifest(configuration.ClientCodex)
	journey.setEnvironment(secrets.EnvironmentKey(account), token)
	journey.run("setup", "--from", journey.manifest)
	journey.run("use", "--for", configuration.ClientCodex, routeID)
	journey.enableNativeClient(configuration.ClientCodex, executable)
	cfg, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(journey.root, "tool-response.txt")
	plan, err := codex.VerificationPlan(executable, filepath.Join(journey.root, "home", ".codex", "config.toml"), output, runtime)
	if err != nil {
		t.Fatal(err)
	}
	plan.Args[len(plan.Args)-1] = "Use a shell tool to run echo AIGW_TOOL_OK, then reply exactly AIGW_OK."
	journey.runWith(executable, plan.Args...)
	if !toolOutput.Load() {
		t.Fatal("Codex completed without a successful exec_command result")
	}
	if got, err := os.ReadFile(output); err != nil || strings.TrimSpace(string(got)) != "AIGW_OK" {
		t.Fatalf("Codex final tool-loop response = %q, %v", got, err)
	}
	journey.uninstallWithAndRequireInstallationRemoved(p.candidate)
}

type codexToolLoopRequest struct {
	Model     string `json:"model"`
	Stream    bool   `json:"stream"`
	Reasoning struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	Tools []struct {
		Name string `json:"name"`
	} `json:"tools"`
	Input []struct {
		Type   string          `json:"type"`
		Output json.RawMessage `json:"output"`
	} `json:"input"`
}

type codexToolLoopProbe struct {
	requests, toolCalls, toolResults, rejected atomic.Int64
	firstResult                                atomic.Pointer[string]
}

func codexToolLoopHandler(token string, toolOutput *atomic.Bool, probe *codexToolLoopProbe) http.Handler {
	var completions atomic.Int64
	base := clientResponseHandler(configuration.ProtocolOpenAIResponses, map[string]*atomic.Int64{"grok-4.7": &completions}, token, "high")
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		probe.requests.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/v1/responses" {
			base.ServeHTTP(response, request)
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			probe.rejected.Add(1)
			http.Error(response, "read request", http.StatusBadRequest)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		var input codexToolLoopRequest
		if err := json.Unmarshal(body, &input); err != nil {
			probe.rejected.Add(1)
			http.Error(response, "decode request", http.StatusBadRequest)
			return
		}
		if input.Model != "grok-4.7" || !input.Stream || input.Reasoning.Effort != "high" {
			probe.rejected.Add(1)
			http.Error(response, "configured model and effort required", http.StatusBadRequest)
			return
		}
		sawResult := false
		for _, item := range input.Input {
			result := string(item.Output)
			if item.Type == "function_call_output" {
				sawResult = true
				probe.toolResults.Add(1)
				preview := redaction.Text(result, token)
				preview = preview[:min(len(preview), 512)]
				probe.firstResult.CompareAndSwap(nil, &preview)
			}
			if item.Type == "function_call_output" && strings.Contains(result, "AIGW_TOOL_OK") &&
				(strings.Contains(result, "Process exited with code 0") || strings.Contains(result, `"exit_code":0`)) {
				toolOutput.Store(true)
			}
		}
		hasExec := false
		for _, tool := range input.Tools {
			hasExec = hasExec || tool.Name == "exec_command"
		}
		if !toolOutput.Load() && hasExec && !sawResult {
			if request.Header.Get("Authorization") != "Bearer "+token && request.Header.Get("X-Api-Key") != token {
				probe.rejected.Add(1)
				base.ServeHTTP(response, request)
				return
			}
			probe.toolCalls.Add(1)
			writeCodexToolCall(response)
			return
		}
		base.ServeHTTP(response, request)
	})
}

func TestCodexToolLoopStopsAfterUnsuccessfulToolResult(t *testing.T) {
	const token = "fixture-token"
	var toolOutput atomic.Bool
	var probe codexToolLoopProbe
	handler := codexToolLoopHandler(token, &toolOutput, &probe)
	for _, body := range []string{
		`{"model":"grok-4.7","stream":true,"reasoning":{"effort":"high"},"tools":[{"name":"exec_command"}],"input":[]}`,
		`{"model":"grok-4.7","stream":true,"reasoning":{"effort":"high"},"tools":[{"name":"exec_command"}],"input":[{"type":"function_call_output","output":"permission denied fixture-token"}]}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("tool-loop fixture response status = %d", response.Code)
		}
	}
	if probe.requests.Load() != 2 || probe.toolCalls.Load() != 1 || probe.toolResults.Load() != 1 || toolOutput.Load() {
		t.Fatal("unsuccessful tool result caused another tool call or false acceptance")
	}
	if first := probe.firstResult.Load(); first == nil || strings.Contains(*first, token) {
		t.Fatal("bounded tool diagnostic was absent or exposed its Token")
	}
}

func writeCodexToolCall(response http.ResponseWriter) {
	const arguments = `{"cmd":"echo AIGW_TOOL_OK"}`
	added := `{"id":"fc_aigw","type":"function_call","call_id":"call_aigw","name":"exec_command","arguments":"","status":"in_progress"}`
	completed := fmt.Sprintf(`{"id":"fc_aigw","type":"function_call","call_id":"call_aigw","name":"exec_command","arguments":%q,"status":"completed"}`, arguments)
	events := []struct{ name, data string }{
		{"response.created", `{"type":"response.created","response":{"id":"resp_aigw_tool","object":"response","status":"in_progress","output":[]}}`},
		{"response.output_item.added", fmt.Sprintf(`{"type":"response.output_item.added","output_index":0,"item":%s}`, added)},
		{"response.function_call_arguments.delta", fmt.Sprintf(`{"type":"response.function_call_arguments.delta","item_id":"fc_aigw","output_index":0,"delta":%q}`, arguments)},
		{"response.function_call_arguments.done", fmt.Sprintf(`{"type":"response.function_call_arguments.done","item_id":"fc_aigw","output_index":0,"arguments":%q}`, arguments)},
		{"response.output_item.done", fmt.Sprintf(`{"type":"response.output_item.done","output_index":0,"item":%s}`, completed)},
		{"response.completed", fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_aigw_tool","object":"response","status":"completed","output":[%s],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`, completed)},
	}
	response.Header().Set("Content-Type", "text/event-stream")
	for _, event := range events {
		if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event.name, event.data); err != nil {
			return
		}
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

func clientResponseHandler(protocol configuration.EndpointProtocol, completions map[string]*atomic.Int64, token, requiredEffort string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":[],"models":[],"has_more":false}`)
	})
	path, _ := streamRequest(protocol, "")
	mux.HandleFunc("POST "+path, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
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
			ReasoningEffort string `json:"reasoning_effort"`
		}
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
		response.Header().Set("Content-Type", "text/event-stream")
		for _, data := range clientResponseEvents(protocol, input.Model) {
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
		completion.Add(1)
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
		return "/v1/responses", fmt.Sprintf(`{"model":%q,"stream":true,"reasoning":{"effort":"high"},"input":[{"role":"user","content":[{"type":"input_text","text":"ping"}]}]}`, model)
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
