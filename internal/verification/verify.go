// Package verification owns explicit, quota-consuming live model probes.
package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

// HTTPDoer executes one HTTP request.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Runner is the ordinary process capability carried by a CLI invocation. Live
// verification additionally requires that the concrete runner implement
// process.CaptureRunner.
type Runner interface {
	Run(context.Context, process.Plan) error
}

// ProtocolTimeout allows a cold Claude CLI process to initialize and complete
// one bounded upstream request.
const ProtocolTimeout = time.Minute

const responseSentinel = "AIGW_OK"
const responseLimit = 256 * 1024

type response struct {
	Status     string `json:"status"`
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

// ValidateFullReadiness checks the local preconditions for verifying both
// supported clients without performing a model request.
func ValidateFullReadiness(cfg configuration.Config) error {
	claudeAdapter := cfg.Adapters[configuration.ClientClaude]
	if !claudeAdapter.Enabled || claudeAdapter.Executable == "" {
		return fmt.Errorf("Full verification requires an enabled Claude adapter; run `aigw repair`")
	}
	ready, err := claude.Ready(claudeAdapter.Executable)
	if err != nil {
		return fmt.Errorf("Failed to inspect Claude executable: %w", err)
	}
	if !ready {
		return fmt.Errorf("Full verification requires an available Claude executable; run `aigw repair`")
	}
	codexAdapter := cfg.Adapters[configuration.ClientCodex]
	if !codexAdapter.Enabled || codexAdapter.Executable == "" || len(codexAdapter.Targets) == 0 {
		return fmt.Errorf("Full verification requires an enabled Codex adapter with at least one configuration target; run `aigw repair`")
	}
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return fmt.Errorf("Failed to resolve the Codex route required for full verification: %w", err)
	}
	for _, target := range codexAdapter.Targets {
		if err := codex.ValidateConfig(target, clientRuntime); err != nil {
			return fmt.Errorf("Full verification requires a synchronized Codex configuration target %s: %w; run `aigw sync`", target, err)
		}
	}
	return nil
}

// VerifyCodexResponse performs one bounded OpenAI Responses request.
func VerifyCodexResponse(ctx context.Context, client HTTPDoer, clientRuntime configuration.Runtime, token string) error {
	if clientRuntime.Model == "" {
		return fmt.Errorf("Profile %q has no Codex model", clientRuntime.ProfileID)
	}
	body, _ := json.Marshal(map[string]any{
		"model":             clientRuntime.Model,
		"input":             "Reply with exactly: AIGW_OK",
		"max_output_tokens": 16,
		"store":             false,
	})
	requestURL := strings.TrimRight(clientRuntime.Endpoint, "/")
	if !strings.HasSuffix(requestURL, "/responses") {
		requestURL += "/responses"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Codex model request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if err != nil {
		return fmt.Errorf("Failed to read Codex verification response: %w", err)
	}
	if len(responseBody) > responseLimit {
		return fmt.Errorf("Codex verification response exceeds %d bytes", responseLimit)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("Codex model authentication was rejected (HTTP %d); run `aigw rotate %s`", resp.StatusCode, clientRuntime.AccountID)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Codex model request returned HTTP %d", resp.StatusCode)
	}
	if !HasResponseSentinel(responseBody) {
		return fmt.Errorf("Codex model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}

// HasResponseSentinel checks supported Responses projections for the exact
// bounded verification marker.
func HasResponseSentinel(data []byte) bool {
	var decoded response
	if err := json.Unmarshal(data, &decoded); err != nil {
		return false
	}
	if decoded.Status != "" && decoded.Status != "completed" {
		return false
	}
	if strings.TrimSpace(decoded.OutputText) == responseSentinel {
		return true
	}
	for _, output := range decoded.Output {
		for _, content := range output.Content {
			if (content.Type == "output_text" || content.Type == "text") && strings.TrimSpace(content.Text) == responseSentinel {
				return true
			}
		}
	}
	return false
}

// VerifyClaudeInvocation checks native executable admission before executing the
// configured Claude adapter.
func VerifyClaudeInvocation(ctx context.Context, runner Runner, cfg configuration.Config, clientRuntime configuration.Runtime, token string) error {
	adapter := cfg.Adapters[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is disabled; run `aigw repair`")
	}
	ready, err := claude.Ready(adapter.Executable)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("Claude executable is unavailable; run `aigw repair`")
	}
	return VerifyClaudeRuntime(ctx, runner, adapter.Executable, clientRuntime, token)
}

// VerifyClaudeRuntime performs one bounded Claude CLI request.
func VerifyClaudeRuntime(ctx context.Context, runner Runner, executable string, clientRuntime configuration.Runtime, token string) error {
	if clientRuntime.Model == "" {
		return fmt.Errorf("Profile %q has no Claude model", clientRuntime.ProfileID)
	}
	plan, err := claude.Plan(executable, []string{"--safe-mode", "--disable-slash-commands", "--no-session-persistence", "--print", "--model", clientRuntime.Model, "Reply with exactly: AIGW_OK"}, os.Environ(), clientRuntime, token)
	if err != nil {
		return err
	}
	plan.Replace = false
	captureRunner, ok := runner.(process.CaptureRunner)
	if !ok {
		return fmt.Errorf("Claude verification runner is unavailable")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, ProtocolTimeout)
	defer cancel()
	output, err := captureRunner.RunCapture(verifyCtx, plan)
	if err != nil {
		return fmt.Errorf("Claude minimal verification request failed: %w", err)
	}
	if strings.TrimSpace(string(output)) != responseSentinel {
		return fmt.Errorf("Claude model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}
