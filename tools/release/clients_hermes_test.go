//go:build client_acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aigw-cli/internal/client/hermes"
	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"

	"aigw-cli/internal/clients"

	"go.yaml.in/yaml/v3"
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

func (j *journeyFixture) verifyNativeHermesSelection(executable string) {
	j.testing.Helper()
	store := configuration.NewStore(j.config, clients.Admission())
	before, err := store.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	selected, err := before.ResolveRuntime(hermes.ID, "")
	if err != nil {
		j.testing.Fatal(err)
	}
	var alternative domain.Runtime
	for _, id := range before.RouteIDs() {
		candidate, err := before.ResolveRuntime(hermes.ID, id)
		if err == nil && candidate.AccountID == selected.AccountID && candidate.Protocol == selected.Protocol && candidate.Model != selected.Model {
			alternative = candidate
			break
		}
	}
	if alternative.RouteID == "" {
		j.testing.Fatal("native Hermes selection requires another owned model on the same protocol")
	}
	target := j.clientProjectionPaths(hermes.ID)[0]
	j.runWith(executable, "config", "set", "model.default", alternative.Model)
	nativeBefore, stateBefore := readFile(j.testing, target), readFile(j.testing, target+".aigw-state.json")
	var adoption struct {
		AdoptedSelections map[string]string `json:"adopted_selections"`
	}
	if err := json.Unmarshal(j.run("sync", "--dry-run", "--json"), &adoption); err != nil || adoption.AdoptedSelections[hermes.ID] != alternative.RouteID {
		j.testing.Fatalf("synchronization preview did not adopt the native selection: %v, %v", adoption.AdoptedSelections, err)
	}
	args := []string{"use", "--for", hermes.ID, "--protocol", string(alternative.Protocol), alternative.RouteID}
	j.run(append(slices.Clone(args), "--dry-run", "--json")...)
	if !bytes.Equal(readFile(j.testing, target), nativeBefore) || !bytes.Equal(readFile(j.testing, target+".aigw-state.json"), stateBefore) {
		j.testing.Fatal("selection preview changed the native preimages")
	}
	j.run(args...)
	after, err := store.Load()
	if err != nil || after.SelectedRoute(hermes.ID) != alternative.RouteID {
		j.testing.Fatalf("native selection adoption: %v", err)
	}
	var original, adopted map[string]any
	if err := yaml.Unmarshal(nativeBefore, &original); err != nil {
		j.testing.Fatal(err)
	}
	if err := yaml.Unmarshal(readFile(j.testing, target), &adopted); err != nil {
		j.testing.Fatal(err)
	}
	if !reflect.DeepEqual(original, adopted) || !reflect.DeepEqual(before.Accounts, after.Accounts) {
		j.testing.Fatal("selection adoption changed native settings or Account metadata")
	}
	var preview struct {
		Targets []struct {
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(j.run("sync", "--dry-run", "--json"), &preview); err != nil {
		j.testing.Fatal(err)
	}
	for _, target := range preview.Targets {
		if target.Action != "unchanged" && target.Action != "already-converged" {
			j.testing.Fatalf("native selection did not converge: %s", target.Action)
		}
	}
	j.run("use", "--for", hermes.ID, "--protocol", string(selected.Protocol), selected.RouteID)
	j.run("check")
	j.requireNativePreferences(hermes.ID)
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
	var input clientInferenceRequest
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

func (recorder *hermesSessionRecorder) offerReadFileCall(response http.ResponseWriter, request *http.Request, input clientInferenceRequest, state hermesSessionSnapshot, hasReadFileSession bool) bool {
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
	arguments, err := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: state.readFilePath})
	if err != nil {
		http.Error(response, "invalid read_file arguments", http.StatusInternalServerError)
		return true
	}
	if err := writeResponsesToolCall(response, "resp_hermes_read_file", "fc_hermes_read_file", hermesReadFileCallID, "read_file", string(arguments), false); err != nil {
		return true
	}
	recorder.mu.Lock()
	recorder.readFileCallIssued = true
	recorder.mu.Unlock()
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

// requireHermesContinuedTurn creates the journey's Hermes session when session
// is empty and otherwise resumes it by its stored identifier. Hermes freezes a
// session's creation options and refuses them on a resume. The session is
// created with a title because Hermes otherwise asks the model for one in the
// background, a request that is not part of the turn; a titled creation is
// itself a resume of the new session, so no turn passes a frozen option.
func (j *journeyFixture) requireHermesContinuedTurn(executable string, recorder *hermesSessionRecorder, completions *atomic.Int64, previousItems int, session string) (int, string) {
	j.testing.Helper()
	before := len(recorder.inputCounts())
	completed := completions.Load()
	first := session == ""
	prompt := "Reply with exactly: AIGW_OK\n"
	args := []string{"chat", "--quiet", "--query-file", "-", "--source", "tool", "--resume", session}
	if first {
		filePath := filepath.Join(j.root, "home", "hermes-read-file.txt")
		fileMarker := fmt.Sprintf("AIGW_HERMES_READ_FILE_%s", filepath.Base(j.root))
		sessionMarker := fmt.Sprintf("AIGW_HERMES_TOOL_SESSION_%s", filepath.Base(j.root))
		if err := os.WriteFile(filePath, []byte(fileMarker+"\n"), 0o600); err != nil {
			j.testing.Fatal(err)
		}
		recorder.expectReadFile(filePath, fileMarker, sessionMarker)
		prompt = fmt.Sprintf("Session marker %q. Use read_file to read %q, then reply with exactly: AIGW_OK\n", sessionMarker, filePath)
		args = []string{
			"chat", "--quiet", "--query-file", "-", "--oneshot", "--source", "tool",
			"--continue", "aigw-native-lifecycle-continuity", "--create-if-missing",
		}
	}
	recorder.requireToolHistory(true)
	defer recorder.requireToolHistory(false)
	output, diagnostics := j.runStreamsWithContext(j.testing.Context(), executable, prompt, args...)
	if !strings.Contains(string(output), "AIGW_OK") {
		j.testing.Fatal("Hermes lifecycle turn did not return the model marker")
	}
	reported := reportedHermesSession(diagnostics)
	if reported == "" || !first && reported != session {
		j.testing.Fatalf("Hermes lifecycle turn reported session %q, want the stored session", reported)
	}
	session = reported
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
	return inputs[finalInput], session
}

// reportedHermesSession reads the identifier on the session_id: line that
// quiet Hermes chats write to their diagnostics stream.
func reportedHermesSession(diagnostics []byte) string {
	session := ""
	for line := range strings.Lines(string(diagnostics)) {
		if id, found := strings.CutPrefix(strings.TrimSpace(line), "session_id:"); found {
			session = strings.TrimSpace(id)
		}
	}
	return session
}

// stopHermesGateway stops the gateway that the journey's chats started in its
// Hermes home through the native command, which refuses an active turn, and
// waits until the gateway releases the home's lock.
func (j *journeyFixture) stopHermesGateway(executable, home string) {
	if hermes.AwaitGatewayAbsence(home, 0, 0) {
		return
	}
	environment := environmentWith(j.environment, map[string]string{
		"HERMES_HOME": home, "HERMES_GATEWAY_LOCK_DIR": hermes.GatewayLockDirectory(home),
	})
	ctx, cancel := context.WithTimeout(context.Background(), clientverification.ProtocolTimeout)
	defer cancel()
	_, _, _ = (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: executable, Directory: home, Env: environment, Args: []string{"gateway", "stop"}})
	hermes.AwaitGatewayAbsence(home, 15*time.Second, 250*time.Millisecond)
}

func testHermesSessionIdentifier(t *testing.T) {
	for _, test := range []struct{ diagnostics, want string }{
		{"\nsession_id: local-0123abc\n", "local-0123abc"},
		{"Hermes ready\n", ""},
	} {
		if got := reportedHermesSession([]byte(test.diagnostics)); got != test.want {
			t.Fatalf("session from %q = %q, want %q", test.diagnostics, got, test.want)
		}
	}
}

func testHermesNativeClientInputs(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "program")
	if err := os.WriteFile(file, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	installation := t.TempDir()
	lock := filepath.Join(installation, "installs", "native", ".install.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_ACCEPTANCE_HERMES_HOME", installation)
	journey := &journeyFixture{
		testing: t, root: root, manifest: filepath.Join(root, "team.toml"),
		endpoint: "http://127.0.0.1:1/v1",
	}
	err := journey.prepareNativeClient(hermes.ID, file, readFile(t, filepath.Join("..", "..", "manifests", "team.toml")))
	home := environmentValues(journey.environment)["HERMES_HOME"]
	if runtime.GOOS == "windows" {
		if err == nil || home != "" || !strings.Contains(err.Error(), "no client process was started") {
			t.Fatalf("unsupported Windows launcher isolation was not refused before client setup: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if lazy := environmentValues(journey.environment)["HERMES_DISABLE_LAZY_INSTALLS"]; lazy != "1" {
		t.Fatalf("Hermes acceptance may install dependencies on demand: HERMES_DISABLE_LAZY_INSTALLS=%q", lazy)
	}
	physicalRoot, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	physicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	if relative, err := filepath.Rel(physicalRoot, physicalHome); err != nil || relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		t.Fatalf("Hermes acceptance home %q is not physically isolated from %q", home, installation)
	}
	if borrowed, err := os.Readlink(filepath.Join(home, "installs", "native", ".install.lock")); err != nil || borrowed != lock {
		t.Fatalf("Hermes acceptance did not retain its native dependency lock: %q, %v", borrowed, err)
	}
	requireFileContains(t, filepath.Join(home, "config.yaml"), "updates:", "security:", "cli:", "expose_on_path: false", "allow_lazy_installs: false", "check: false")
}
