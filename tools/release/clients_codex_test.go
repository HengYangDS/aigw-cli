//go:build client_acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
	"aigw-cli/internal/secrets"
)

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
	journey.run("use", "--for", configuration.ClientCodex, "aihubmix-gpt-6.1-sol")
	configured, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := configured.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(journey.root, "base-model-response.txt")
	plan, err := codex.VerificationPlan(executable, filepath.Join(journey.root, "home", ".codex", "config.toml"), output, runtime)
	if err != nil {
		t.Fatal(err)
	}
	plan.Env = journey.environment
	ctx, cancel := context.WithTimeout(t.Context(), clientverification.ProtocolTimeout)
	defer cancel()
	_, stderr, err := (process.Runner{}).RunCaptureStreams(ctx, plan)
	if err != nil || strings.Contains(string(stderr), "codex_models_manager") {
		t.Fatalf("Codex base-model metadata request failed: %v; stderr: %s", err, redaction.Text(string(stderr), token))
	}
	if got := strings.TrimSpace(string(readFile(t, output))); got != "AIGW_OK" {
		t.Fatalf("Codex base-model response = %q", got)
	}
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
		var input responsesToolLoopRequest
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
		outputs, err := responsesToolLoopOutputs(input.Input)
		if err != nil {
			probe.rejected.Add(1)
			http.Error(response, "invalid Responses tool input", http.StatusBadRequest)
			return
		}
		sawResult := false
		for _, item := range outputs {
			result := string(item.Output)
			sawResult = true
			probe.toolResults.Add(1)
			preview := redaction.Text(result, token)
			preview = preview[:min(len(preview), 512)]
			probe.firstResult.CompareAndSwap(nil, &preview)
			if strings.Contains(result, "AIGW_TOOL_OK") &&
				(strings.Contains(result, "Process exited with code 0") || strings.Contains(result, `"exit_code":0`)) {
				toolOutput.Store(true)
			}
		}
		hasExec := false
		for _, tool := range input.Tools {
			hasExec = hasExec || tool.Name == "exec_command"
		}
		if !toolOutput.Load() && hasExec && !sawResult {
			if !clientFixtureAuthorized(request, token) {
				probe.rejected.Add(1)
				base.ServeHTTP(response, request)
				return
			}
			probe.toolCalls.Add(1)
			writeResponsesFunctionCall(
				response,
				"resp_aigw_tool",
				"fc_aigw",
				"call_aigw",
				"exec_command",
				`{"cmd":"echo AIGW_TOOL_OK"}`,
			)
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
