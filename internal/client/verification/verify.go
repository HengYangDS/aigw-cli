// Package verification owns explicit, quota-consuming live model probes.
package verification

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"

	"github.com/rogpeppe/go-internal/robustio"
)

// ProtocolTimeout bounds each native turn, including cold client startup.
const ProtocolTimeout = time.Minute

const responseSentinel = "AIGW_OK"

var removeCodexWorkspace = robustio.RemoveAll

// VerifyCodexInvocation proves a native file-reading tool and exact-session
// recall with the selected projection in a disposable, operation-owned home.
func VerifyCodexInvocation(ctx context.Context, runner process.VerificationRunner, cfg configuration.Config, clientRuntime configuration.Runtime) (_ codex.ExecutableIdentity, result error) {
	adapter := cfg.Clients[configuration.ClientCodex]
	if !adapter.Enabled {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex adapter is disabled; run `aigw repair`")
	}
	if adapter.Executable == "" {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex executable is not configured; run `aigw repair`")
	}
	if len(adapter.Targets) == 0 {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex configuration target is missing; run `aigw repair`")
	}
	if clientRuntime.Model == "" {
		return codex.ExecutableIdentity{}, fmt.Errorf("Route %q has no Codex model", clientRuntime.RouteID)
	}
	targets := append([]string(nil), adapter.Targets...)
	sort.Strings(targets)
	target := targets[0]
	if err := codex.ValidateConfig(target, clientRuntime); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex configuration target is not synchronized: %w; run `aigw sync`", err)
	}
	if runner == nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex verification capture runner is unavailable")
	}
	workspace, err := os.MkdirTemp("", "aigw-codex-verification-")
	if err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("create Codex verification workspace: %w", err)
	}
	defer func() {
		if err := removeCodexWorkspace(workspace); err != nil {
			result = errors.Join(result, fmt.Errorf("remove Codex verification workspace %s: %w", workspace, err))
		}
	}()
	isolated := filepath.Join(workspace, "home", "config.toml")
	if err := codex.CopyProjection(target, isolated); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("copy Codex verification projection: %w", err)
	}
	identityCtx, cancel := context.WithTimeout(ctx, ProtocolTimeout)
	identity, err := codex.IdentifyExecutable(identityCtx, runner, adapter.Executable, filepath.Dir(isolated), filepath.Join(filepath.Dir(isolated), "tmp"))
	cancel()
	if err != nil {
		return codex.ExecutableIdentity{}, err
	}
	challenge := filepath.Join(workspace, "challenge.txt")
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("create Codex verification challenge: %w", err)
	}
	marker := hex.EncodeToString(value)
	if err := os.WriteFile(challenge, []byte(marker+"\n"), 0o600); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("write Codex verification challenge: %w", err)
	}
	session := ""
	for index, prompt := range []string{
		"Use a shell tool to read challenge.txt in the current directory. Reply with exactly its contents.",
		"Without reading files or using tools again, reply with exactly the contents of the file you read in the previous turn.",
	} {
		if index == 1 {
			if err := os.Remove(challenge); err != nil {
				return codex.ExecutableIdentity{}, fmt.Errorf("remove Codex verification challenge before recall: %w", err)
			}
		}
		outputPath := filepath.Join(workspace, "response.txt")
		plan, err := codex.VerificationPlan(adapter.Executable, isolated, outputPath, clientRuntime, prompt, session)
		if err != nil {
			return codex.ExecutableIdentity{}, err
		}
		session, err = verifyCodexTurn(ctx, runner, plan, outputPath, session, marker)
		if err != nil {
			return codex.ExecutableIdentity{}, err
		}
	}
	return identity, nil
}

func verifyCodexTurn(ctx context.Context, runner process.VerificationRunner, plan process.Plan, outputPath, sessionID, marker string) (string, error) {
	if err := os.Remove(outputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove preceding Codex final response: %w", err)
	}
	turnCtx, cancel := context.WithTimeout(ctx, ProtocolTimeout)
	defer cancel()
	output, diagnostic, err := runner.RunCaptureStreams(turnCtx, plan)
	if err != nil {
		decoder := json.NewDecoder(bytes.NewReader(output))
		for {
			var event codexNativeEvent
			if decoder.Decode(&event) != nil {
				break
			}
			if event.Type == "error" || event.Type == "turn.failed" {
				diagnostic = fmt.Appendf(diagnostic, "\n%s\n%s", event.Message, event.Error.Message)
			}
		}
		return "", verificationFailure("Codex", configuration.ClientCodex, diagnostic, err)
	}
	if process.DiagnosticFailure(diagnostic) {
		return "", verificationDiagnostic("Codex", diagnostic)
	}
	finalMessage, err := readBoundedFile(outputPath, int64(len(marker)+2))
	if err != nil {
		return "", fmt.Errorf("read Codex final response: %w", err)
	}
	if strings.TrimSpace(string(finalMessage)) != marker {
		return "", fmt.Errorf("Codex final response did not return the private verification challenge")
	}
	return codexTurnEvidence(output, sessionID, marker)
}

type codexNativeEvent struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Message  string `json:"message"`
	Error    struct {
		Message string `json:"message"`
	} `json:"error"`
	Item struct {
		Type     string `json:"type"`
		Message  string `json:"message"`
		Status   string `json:"status"`
		Command  string `json:"command"`
		Output   string `json:"aggregated_output"`
		Text     string `json:"text"`
		ExitCode *int   `json:"exit_code"`
	} `json:"item"`
}

func (event codexNativeEvent) challengeEvidence(marker, expectedSession string) (tool, message bool, err error) {
	switch event.Item.Type {
	case "error":
		return false, false, verificationDiagnostic("Codex", []byte(event.Item.Message))
	case "agent_message", "reasoning":
	default:
		if expectedSession != "" {
			return false, false, fmt.Errorf("Codex continuation used a tool instead of recalling the previous turn")
		}
	}
	if event.Type != "item.completed" {
		return false, false, nil
	}
	switch event.Item.Type {
	case "command_execution":
		return event.Item.Status == "completed" && event.Item.ExitCode != nil && *event.Item.ExitCode == 0 &&
			strings.Contains(event.Item.Command, "challenge.txt") && strings.TrimSpace(event.Item.Output) == marker, false, nil
	case "agent_message":
		return false, strings.TrimSpace(event.Item.Text) == marker, nil
	default:
		return false, false, nil
	}
}

func codexTurnEvidence(output []byte, expectedSession, marker string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	var session string
	var finalMessage, completed bool
	toolPending := expectedSession == ""
	for {
		var event codexNativeEvent
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return "", fmt.Errorf("Codex native event evidence is malformed")
		}
		if completed || session == "" && event.Type != "thread.started" {
			return "", fmt.Errorf("Codex native evidence is outside its owned session or completed turn")
		}
		switch event.Type {
		case "thread.started":
			if session != "" || event.ThreadID == "" || expectedSession != "" && event.ThreadID != expectedSession {
				return "", fmt.Errorf("Codex native session identity is missing or changed")
			}
			session = event.ThreadID
		case "item.started", "item.updated", "item.completed":
			tool, message, err := event.challengeEvidence(marker, expectedSession)
			if err != nil {
				return "", err
			}
			if message && toolPending {
				return "", fmt.Errorf("Codex native reply preceded its challenge tool read")
			}
			toolPending, finalMessage = toolPending && !tool, finalMessage || message
		case "turn.completed":
			completed = true
		case "error", "turn.failed":
			return "", fmt.Errorf("Codex native turn failed; diagnostics suppressed")
		}
	}
	if !completed || !finalMessage || toolPending {
		return "", fmt.Errorf("Codex native tool/continuation evidence is incomplete")
	}
	return session, nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

// VerifyClaudeRuntime performs one bounded Claude CLI request.
func VerifyClaudeRuntime(ctx context.Context, runner process.VerificationRunner, executable, settingsPath string, clientRuntime configuration.Runtime, token string) error {
	if clientRuntime.Model == "" {
		return fmt.Errorf("Route %q has no Claude model", clientRuntime.RouteID)
	}
	plan, err := claude.VerificationPlan(executable, settingsPath, "Reply with exactly: AIGW_OK", os.Environ(), clientRuntime)
	if err != nil {
		return err
	}
	if runner == nil {
		return fmt.Errorf("Claude verification runner is unavailable")
	}
	output, diagnostic, err := runner.RunCaptureStreams(ctx, plan)
	if err != nil {
		return verificationFailure("Claude", configuration.ClientClaude, bytes.Join([][]byte{diagnostic, output}, []byte("\n")), err, token)
	}
	if process.DiagnosticFailure(diagnostic) {
		return verificationDiagnostic("Claude", diagnostic)
	}
	if strings.TrimSpace(string(output)) != responseSentinel {
		return fmt.Errorf("Claude model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}

func verificationDiagnostic(label string, diagnostic []byte) error {
	text := strings.ToLower(string(diagnostic))
	if strings.Contains(text, "model metadata") && strings.Contains(text, "fallback metadata") {
		return fmt.Errorf("%s emitted a model-metadata warning; native metadata is incomplete and verification is unqualified; select a model supported by the client or update the client", label)
	}
	return fmt.Errorf("%s emitted a native-client warning or error; verification is incomplete; inspect the client with the selected Route", label)
}

type requestFailureError struct {
	message string
	cause   error
}

func (failure requestFailureError) Error() string { return failure.message }

func (failure requestFailureError) Unwrap() error { return failure.cause }

func verificationFailure(label, client string, diagnostic []byte, cause error, secrets ...string) error {
	detail := verificationFailureSummary(client, diagnostic, cause, secrets...)
	next := "aigw verify --for " + client
	return requestFailureError{
		message: fmt.Sprintf("%s minimal verification request failed: %s; run `%s`", label, detail, next),
		cause:   cause,
	}
}

func verificationFailureSummary(client string, diagnostic []byte, cause error, secrets ...string) string {
	text := strings.ToLower(redaction.Text(string(diagnostic), secrets...))
	switch {
	case errors.Is(cause, context.DeadlineExceeded), strings.Contains(text, "deadline exceeded"), strings.Contains(text, "timed out"):
		return "client verification timed out"
	case client == configuration.ClientClaude && strings.Contains(text, "context_management") && strings.Contains(text, "extra inputs are not permitted"):
		return "endpoint rejects pre-release context management; set CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1 in native Claude settings env and verify again"
	case strings.Contains(text, "unrecognized_model"),
		strings.Contains(text, "not support for model"),
		strings.Contains(text, "model id") && strings.Contains(text, "incorrect"),
		strings.Contains(text, "model") && strings.Contains(text, "unavailable"),
		strings.Contains(text, "model") && strings.Contains(text, "status 503"):
		return "selected model is unavailable through the client or endpoint"
	case strings.Contains(text, "provider auth command"),
		strings.Contains(text, "unauthorized"),
		strings.Contains(text, "authentication"),
		strings.Contains(text, "status 401"):
		return "client or endpoint rejected authentication"
	default:
		return "client process rejected the request; diagnostics suppressed"
	}
}
