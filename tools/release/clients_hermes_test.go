//go:build client_acceptance

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

type hermesSessionRecorder struct {
	http.Handler
	model  string
	mu     sync.Mutex
	inputs []int
}

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
	var input struct {
		Model  string          `json:"model"`
		Stream bool            `json:"stream"`
		Input  json.RawMessage `json:"input"`
	}
	if json.Unmarshal(body, &input) != nil || input.Model != recorder.model || len(input.Input) == 0 {
		http.Error(response, "configured model input required", http.StatusBadRequest)
		return
	}
	if !input.Stream {
		recorder.Handler.ServeHTTP(response, request)
		return
	}
	var items []json.RawMessage
	if json.Unmarshal(input.Input, &items) != nil || len(items) == 0 {
		http.Error(response, "configured streaming input required", http.StatusBadRequest)
		return
	}
	recorder.mu.Lock()
	recorder.inputs = append(recorder.inputs, len(items))
	recorder.mu.Unlock()
	recorder.Handler.ServeHTTP(response, request)
}

func (recorder *hermesSessionRecorder) inputCounts() []int {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return slices.Clone(recorder.inputs)
}

func (j *journeyFixture) requireHermesTwoTurn(executable string, recorder *hermesSessionRecorder, completions *atomic.Int64) {
	j.testing.Helper()
	before := len(recorder.inputCounts())
	completed := completions.Load()
	args := []string{
		"chat", "--quiet", "--query-file", "-", "--max-turns", "1", "--run-budget", "45",
		"--ignore-rules", "--source", "tool", "--continue", "aigw-native-two-turn",
	}
	for turn := range 2 {
		invocation := slices.Clone(args)
		if turn == 0 {
			invocation = append(invocation, "--create-if-missing")
		}
		output := j.runWithInput(executable, "Reply with exactly: AIGW_OK\n", invocation...)
		if !strings.Contains(string(output), "AIGW_OK") {
			j.testing.Fatalf("Hermes turn %d did not return the model marker", turn+1)
		}
	}
	inputs := recorder.inputCounts()
	if len(inputs) != before+2 || inputs[before] < 1 || inputs[before+1] <= inputs[before] || completions.Load() != completed+2 {
		j.testing.Fatalf("Hermes did not retain a two-turn model session: input items=%v completions=%d", inputs[before:], completions.Load()-completed)
	}
}

func (j *journeyFixture) requireHermesContinuedTurn(executable string, recorder *hermesSessionRecorder, completions *atomic.Int64, previousItems int, first bool) int {
	j.testing.Helper()
	before := len(recorder.inputCounts())
	completed := completions.Load()
	args := []string{
		"chat", "--quiet", "--query-file", "-", "--max-turns", "1", "--run-budget", "45",
		"--ignore-rules", "--source", "tool", "--continue", "aigw-native-lifecycle-continuity",
	}
	if first {
		args = append(args, "--create-if-missing")
	}
	output := j.runWithInput(executable, "Reply with exactly: AIGW_OK\n", args...)
	if !strings.Contains(string(output), "AIGW_OK") {
		j.testing.Fatal("Hermes lifecycle turn did not return the model marker")
	}
	inputs := recorder.inputCounts()
	if len(inputs) != before+1 || inputs[before] <= previousItems || completions.Load() != completed+1 {
		j.testing.Fatalf("Hermes did not continue one session across lifecycle: input items=%v previous=%d completions=%d", inputs[before:], previousItems, completions.Load()-completed)
	}
	return inputs[before]
}
