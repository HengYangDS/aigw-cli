//go:build client_acceptance

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/configuration"
)

func newHermesSessionRecorderForTest(model, token string, completions *atomic.Int64) *hermesSessionRecorder {
	return &hermesSessionRecorder{
		Handler: clientResponseHandler(
			configuration.ProtocolOpenAIResponses,
			map[string]*atomic.Int64{model: completions},
			token,
			"",
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
