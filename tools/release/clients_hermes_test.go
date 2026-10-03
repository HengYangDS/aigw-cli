//go:build client_acceptance

package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

	"aigw-cli/internal/configuration"
)

type hermesSessionRecorder struct {
	http.Handler
	model, token                                        string
	mu                                                  sync.Mutex
	inputs                                              []int
	readFilePath, readFileMarker, readFileSessionMarker string
	readFileCallIssued, readFileOutputAccepted          bool
	toolHistoryRequired                                 bool
}

type hermesSessionSnapshot struct {
	readFilePath, readFileMarker, readFileSessionMarker string
	readFileCallIssued, readFileOutputAccepted          bool
	toolHistoryRequired                                 bool
}

const hermesReadFileCallID = "call_hermes_read_file"

func (recorder *hermesSessionRecorder) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/v1/responses" {
		recorder.Handler.ServeHTTP(response, request)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, 1<<20))
	if err != nil {
		http.Error(response, "invalid model request", http.StatusBadRequest)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	var input responsesToolLoopRequest
	if json.Unmarshal(body, &input) != nil || input.Model != recorder.model || responsesToolLoopInputCount(input.Input) == 0 {
		http.Error(response, "configured model input required", http.StatusBadRequest)
		return
	}
	if !input.Stream {
		recorder.Handler.ServeHTTP(response, request)
		return
	}
	state := recorder.recordInput(input.Input)
	hasReadFileSession, hasReadFileOutput, err := recorder.validateReadFileHistory(input.Input, state)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	if recorder.offerReadFileCall(response, request, input, state, hasReadFileSession) {
		return
	}
	if err := recorder.requireRetainedReadFileOutput(state, hasReadFileSession, hasReadFileOutput); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	recorder.Handler.ServeHTTP(response, request)
}

func (recorder *hermesSessionRecorder) recordInput(input json.RawMessage) hermesSessionSnapshot {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.inputs = append(recorder.inputs, responsesToolLoopInputCount(input))
	return hermesSessionSnapshot{
		readFilePath:           recorder.readFilePath,
		readFileMarker:         recorder.readFileMarker,
		readFileSessionMarker:  recorder.readFileSessionMarker,
		readFileCallIssued:     recorder.readFileCallIssued,
		readFileOutputAccepted: recorder.readFileOutputAccepted,
		toolHistoryRequired:    recorder.toolHistoryRequired,
	}
}

func (recorder *hermesSessionRecorder) validateReadFileHistory(input json.RawMessage, state hermesSessionSnapshot) (bool, bool, error) {
	hasReadFileSession := inputContainsHermesMarker(input, state.readFileSessionMarker)
	if state.toolHistoryRequired && !hasReadFileSession {
		return false, false, errors.New("read_file session history missing")
	}
	outputs, err := responsesToolLoopOutputs(input)
	if err != nil && (hasReadFileSession || state.toolHistoryRequired) {
		return hasReadFileSession, false, errors.New("invalid read_file session input")
	}
	hasReadFileOutput := false
	for _, item := range outputs {
		if !hasReadFileSession && state.readFilePath != "" && !state.toolHistoryRequired {
			continue
		}
		hasReadFileOutput = true
		if state.readFilePath == "" || !hasReadFileSession || !state.readFileCallIssued ||
			item.CallID != hermesReadFileCallID || !hermesReadFileOutputMatches(item.Output, state.readFileMarker) {
			return hasReadFileSession, hasReadFileOutput, errors.New("matching read_file output required")
		}
	}
	return hasReadFileSession, hasReadFileOutput, nil
}

func (recorder *hermesSessionRecorder) offerReadFileCall(response http.ResponseWriter, request *http.Request, input responsesToolLoopRequest, state hermesSessionSnapshot, hasReadFileSession bool) bool {
	if state.readFilePath == "" || state.readFileCallIssued {
		return false
	}
	if !hasReadFileSession {
		http.Error(response, "read_file session marker required", http.StatusBadRequest)
		return true
	}
	if !clientFixtureAuthorized(request, recorder.token) {
		http.Error(response, "credential mismatch", http.StatusUnauthorized)
		return true
	}
	if !slices.ContainsFunc(input.Tools, func(tool responsesToolDefinition) bool {
		return tool.Name == "read_file"
	}) {
		http.Error(response, "read_file tool declaration required", http.StatusBadRequest)
		return true
	}
	recorder.mu.Lock()
	recorder.readFileCallIssued = true
	recorder.mu.Unlock()
	arguments, err := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: state.readFilePath})
	if err != nil {
		http.Error(response, "invalid read_file arguments", http.StatusInternalServerError)
		return true
	}
	writeResponsesFunctionCall(
		response,
		"resp_hermes_read_file",
		"fc_hermes_read_file",
		hermesReadFileCallID,
		"read_file",
		string(arguments),
	)
	return true
}

func (recorder *hermesSessionRecorder) requireRetainedReadFileOutput(state hermesSessionSnapshot, hasReadFileSession, hasReadFileOutput bool) error {
	if state.readFilePath == "" || !hasReadFileSession {
		return nil
	}
	if state.readFileCallIssued && !state.readFileOutputAccepted && !hasReadFileOutput {
		return errors.New("read_file output missing from continued input")
	}
	if state.readFileOutputAccepted && !hasReadFileOutput {
		return errors.New("read_file output was not retained in the session")
	}
	if state.readFileCallIssued && !state.readFileOutputAccepted && hasReadFileOutput {
		recorder.mu.Lock()
		recorder.readFileOutputAccepted = true
		recorder.mu.Unlock()
	}
	return nil
}

func (recorder *hermesSessionRecorder) inputCounts() []int {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return slices.Clone(recorder.inputs)
}

func (recorder *hermesSessionRecorder) expectReadFile(path, marker, sessionMarker string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.readFilePath = path
	recorder.readFileMarker = marker
	recorder.readFileSessionMarker = sessionMarker
}

func (recorder *hermesSessionRecorder) requireToolHistory(required bool) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.toolHistoryRequired = required
}

func inputContainsHermesMarker(input json.RawMessage, marker string) bool {
	return marker != "" && bytes.Contains(input, []byte(marker))
}

func hermesReadFileOutputMatches(output json.RawMessage, marker string) bool {
	var serialized string
	if marker == "" || json.Unmarshal(output, &serialized) != nil {
		return false
	}
	var result struct {
		Content  json.RawMessage `json:"content"`
		Error    json.RawMessage `json:"error"`
		IsError  *bool           `json:"is_error"`
		IsBinary *bool           `json:"is_binary"`
	}
	if json.Unmarshal([]byte(serialized), &result) != nil || len(result.Content) == 0 ||
		(result.IsError != nil && *result.IsError) || result.IsBinary == nil || *result.IsBinary ||
		!hermesToolErrorIsEmpty(result.Error) {
		return false
	}
	return bytes.Contains(result.Content, []byte(marker))
}

func hermesToolErrorIsEmpty(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return true
	}
	var message string
	return json.Unmarshal(trimmed, &message) == nil && strings.TrimSpace(message) == ""
}

func (recorder *hermesSessionRecorder) fileReadVerified() bool {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.readFileOutputAccepted
}

func newHermesSessionRecorderForTest(model, token string, completions *atomic.Int64) *hermesSessionRecorder {
	return &hermesSessionRecorder{
		Handler: clientResponseHandler(
			configuration.ProtocolOpenAIResponses,
			map[string]*atomic.Int64{model: completions},
			token,
			"",
		),
		model: model,
		token: token,
	}
}

func serveHermesSessionRequest(t *testing.T, recorder *hermesSessionRecorder, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+recorder.token)
	response := httptest.NewRecorder()
	recorder.ServeHTTP(response, request)
	return response
}

func hermesSerializedReadFileOutput(t *testing.T, content, errorMessage string, isBinary bool) string {
	t.Helper()
	serialized, err := json.Marshal(struct {
		Content  string `json:"content"`
		Error    string `json:"error,omitempty"`
		IsBinary bool   `json:"is_binary"`
	}{Content: content, Error: errorMessage, IsBinary: isBinary})
	if err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(string(serialized))
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func TestHermesSessionRecorderRejectsUnsolicitedToolOutput(t *testing.T) {
	const (
		model = "hermes-tool-fixture"
		token = "fixture-token"
	)
	var completions atomic.Int64
	recorder := newHermesSessionRecorderForTest(model, token, &completions)
	response := serveHermesSessionRequest(t, recorder,
		`{"model":"hermes-tool-fixture","stream":true,"max_output_tokens":64,"store":false,"tools":[{"type":"function","name":"read_file"}],"input":[{"type":"function_call_output","call_id":"call_unrequested","output":"AIGW_OK"}]}`,
	)
	if response.Code != http.StatusBadRequest || completions.Load() != 0 {
		t.Fatalf("unsolicited Hermes tool output accepted: status=%d completions=%d", response.Code, completions.Load())
	}
}

func TestHermesSessionRecorderRejectsMismatchedReadFileOutput(t *testing.T) {
	const (
		model         = "hermes-tool-fixture"
		token         = "fixture-token"
		marker        = "AIGW_HERMES_EXPECTED_FILE_MARKER"
		sessionMarker = "AIGW_HERMES_TOOL_SESSION_FIXTURE"
	)
	for _, test := range []struct {
		name, callID, outputMarker, errorMessage string
		isBinary                                 bool
		missing                                  bool
	}{
		{name: "wrong call ID", callID: "call_other", outputMarker: marker},
		{name: "wrong file content", callID: hermesReadFileCallID, outputMarker: "unrelated content"},
		{name: "tool error containing marker", callID: hermesReadFileCallID, outputMarker: marker, errorMessage: "File not found: " + marker},
		{name: "binary result", callID: hermesReadFileCallID, outputMarker: marker, isBinary: true},
		{name: "missing tool output", missing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var completions atomic.Int64
			recorder := newHermesSessionRecorderForTest(model, token, &completions)
			recorder.expectReadFile(filepath.Join(t.TempDir(), "read-file.txt"), marker, sessionMarker)
			toolRequest := fmt.Sprintf(`{"model":%q,"stream":true,"max_output_tokens":64,"store":false,"tools":[{"type":"function","name":"read_file"}],"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}]}`, model, sessionMarker)
			toolCall := serveHermesSessionRequest(t, recorder, toolRequest)
			if toolCall.Code != http.StatusOK || !strings.Contains(toolCall.Body.String(), `"name":"read_file"`) {
				t.Fatalf("read_file call was not offered: status=%d body=%s", toolCall.Code, toolCall.Body.String())
			}

			input := fmt.Sprintf(`[{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"AIGW_OK"}]}]`, sessionMarker)
			if !test.missing {
				output := hermesSerializedReadFileOutput(t, test.outputMarker, test.errorMessage, test.isBinary)
				input = fmt.Sprintf(`[{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]},{"type":"function_call_output","call_id":%q,"output":%s}]`, sessionMarker, test.callID, output)
			}
			outputRequest := fmt.Sprintf(`{"model":%q,"stream":true,"max_output_tokens":64,"store":false,"input":%s}`, model, input)
			response := serveHermesSessionRequest(t, recorder, outputRequest)
			if response.Code != http.StatusBadRequest || completions.Load() != 0 || recorder.fileReadVerified() {
				t.Fatalf("mismatched Hermes tool output accepted: status=%d completions=%d", response.Code, completions.Load())
			}
		})
	}
}

func TestHermesSessionRecorderRequiresReadFileOutputInRetainedInput(t *testing.T) {
	const (
		model         = "hermes-tool-fixture"
		token         = "fixture-token"
		marker        = "AIGW_HERMES_EXPECTED_FILE_MARKER"
		sessionMarker = "AIGW_HERMES_TOOL_SESSION_FIXTURE"
	)
	var completions atomic.Int64
	recorder := newHermesSessionRecorderForTest(model, token, &completions)
	recorder.expectReadFile(filepath.Join(t.TempDir(), "read-file.txt"), marker, sessionMarker)
	toolRequest := fmt.Sprintf(`{"model":%q,"stream":true,"max_output_tokens":64,"store":false,"tools":[{"type":"function","name":"read_file"}],"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}]}`, model, sessionMarker)
	toolCall := serveHermesSessionRequest(t, recorder, toolRequest)
	if toolCall.Code != http.StatusOK || !strings.Contains(toolCall.Body.String(), `"name":"read_file"`) {
		t.Fatalf("read_file call was not offered: status=%d body=%s", toolCall.Code, toolCall.Body.String())
	}
	toolOutput := fmt.Sprintf(`{"type":"function_call_output","call_id":%q,"output":%s}`, hermesReadFileCallID, hermesSerializedReadFileOutput(t, marker, "", false))
	userMessage := fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}`, sessionMarker)
	continuedInput := fmt.Sprintf(`[%s,%s,{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]`, userMessage, toolOutput)
	for _, input := range []string{
		fmt.Sprintf(`[%s,%s]`, userMessage, toolOutput),
		continuedInput,
	} {
		request := fmt.Sprintf(`{"model":%q,"stream":true,"max_output_tokens":64,"store":false,"input":%s}`, model, input)
		response := serveHermesSessionRequest(t, recorder, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "AIGW_OK") {
			t.Fatalf("matching read_file output was not accepted: status=%d body=%s", response.Code, response.Body.String())
		}
	}
	if completions.Load() != 2 || !recorder.fileReadVerified() {
		t.Fatalf("read_file output evidence = completions:%d accepted:%t", completions.Load(), recorder.fileReadVerified())
	}

	lostHistory := fmt.Sprintf(`{"model":%q,"stream":true,"max_output_tokens":64,"store":false,"input":[%s]}`, model, userMessage)
	response := serveHermesSessionRequest(t, recorder, lostHistory)
	if response.Code != http.StatusBadRequest || completions.Load() != 2 {
		t.Fatalf("Hermes continued after its read_file output was removed: status=%d completions=%d", response.Code, completions.Load())
	}

	freshSession := fmt.Sprintf(`{"model":%q,"stream":true,"max_output_tokens":64,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"fresh session"}]}]}`, model)
	response = serveHermesSessionRequest(t, recorder, freshSession)
	if response.Code != http.StatusOK || completions.Load() != 3 {
		t.Fatalf("fresh Hermes session was rejected: status=%d completions=%d", response.Code, completions.Load())
	}
}

func (j *journeyFixture) requireHermesContinuedTurn(executable string, recorder *hermesSessionRecorder, completions *atomic.Int64, previousItems int, first bool) int {
	j.testing.Helper()
	before := len(recorder.inputCounts())
	completed := completions.Load()
	maxTurns := "1"
	prompt := "Reply with exactly: AIGW_OK\n"
	if first {
		filePath := filepath.Join(j.root, "home", "hermes-read-file.txt")
		fileMarker := fmt.Sprintf("AIGW_HERMES_READ_FILE_%s", filepath.Base(j.root))
		sessionMarker := fmt.Sprintf("AIGW_HERMES_TOOL_SESSION_%s", filepath.Base(j.root))
		if err := os.WriteFile(filePath, []byte(fileMarker+"\n"), 0o600); err != nil {
			j.testing.Fatal(err)
		}
		recorder.expectReadFile(filePath, fileMarker, sessionMarker)
		maxTurns = "2"
		prompt = fmt.Sprintf("Session marker %q. Use read_file to read %q, then reply with exactly: AIGW_OK\n", sessionMarker, filePath)
	}
	recorder.requireToolHistory(true)
	defer recorder.requireToolHistory(false)
	args := []string{
		"chat", "--quiet", "--query-file", "-", "--max-turns", maxTurns, "--run-budget", "45",
		"--ignore-rules", "--source", "tool", "--continue", "aigw-native-lifecycle-continuity",
	}
	if first {
		args = append(args, "--create-if-missing")
	}
	output := j.runWithInput(executable, prompt, args...)
	if !strings.Contains(string(output), "AIGW_OK") {
		j.testing.Fatal("Hermes lifecycle turn did not return the model marker")
	}
	inputs := recorder.inputCounts()
	requestCount := 1
	finalInput := before
	if first {
		requestCount = 2
		finalInput++
	}
	if len(inputs) != before+requestCount {
		j.testing.Fatalf("Hermes request count across lifecycle = %d, want %d", len(inputs)-before, requestCount)
	}
	if first {
		if inputs[before+1] <= inputs[before] {
			j.testing.Fatalf("Hermes read_file output did not continue the same request: input items=%v", inputs[before:])
		}
	}
	if inputs[finalInput] <= previousItems || completions.Load() != completed+1 {
		j.testing.Fatalf("Hermes did not continue one session across lifecycle: input items=%v previous=%d completions=%d", inputs[before:], previousItems, completions.Load()-completed)
	}
	if first && !recorder.fileReadVerified() {
		j.testing.Fatal("Hermes lifecycle session did not return the matching read_file result")
	}
	return inputs[finalInput]
}
