package codex

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

	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"

	"github.com/rogpeppe/go-internal/robustio"
)

var removeVerificationWorkspace = robustio.RemoveAll

// VerifyInvocation proves a native file-reading tool and exact-session recall
// with the selected projection in a disposable, operation-owned home.
// explicitRoute is the invocation's --route, which its retry advice repeats.
func VerifyInvocation(ctx context.Context, runner process.VerificationRunner, cfg domain.Config, clientRuntime domain.Runtime, explicitRoute string) (_ ExecutableIdentity, result error) {
	binding := cfg.Clients[ID]
	if !binding.Enabled {
		return ExecutableIdentity{}, fmt.Errorf("Codex adapter is disabled; run `aigw repair`")
	}
	if binding.Executable == "" {
		return ExecutableIdentity{}, fmt.Errorf("Codex executable is not configured; run `aigw repair`")
	}
	if len(binding.Targets) == 0 {
		return ExecutableIdentity{}, fmt.Errorf("Codex configuration target is missing; run `aigw repair`")
	}
	if clientRuntime.Model == "" {
		return ExecutableIdentity{}, fmt.Errorf("Route %q has no Codex model", clientRuntime.RouteID)
	}
	targets := append([]string(nil), binding.Targets...)
	sort.Strings(targets)
	target := targets[0]
	if err := ValidateConfig(target, clientRuntime); err != nil {
		return ExecutableIdentity{}, fmt.Errorf("Codex configuration target is not synchronized: %w; run `aigw sync`", err)
	}
	if runner == nil {
		return ExecutableIdentity{}, fmt.Errorf("Codex verification capture runner is unavailable")
	}
	workspace, err := os.MkdirTemp("", "aigw-codex-verification-")
	if err != nil {
		return ExecutableIdentity{}, fmt.Errorf("create Codex verification workspace: %w", err)
	}
	defer func() {
		if err := removeVerificationWorkspace(workspace); err != nil {
			result = errors.Join(result, fmt.Errorf("remove Codex verification workspace %s: %w", workspace, err))
		}
	}()
	isolated := filepath.Join(workspace, "home", "config.toml")
	if err := CopyProjection(target, isolated); err != nil {
		return ExecutableIdentity{}, fmt.Errorf("copy Codex verification projection: %w", err)
	}
	identityCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	identity, err := IdentifyExecutable(identityCtx, runner, binding.Executable, filepath.Dir(isolated), filepath.Join(filepath.Dir(isolated), "tmp"))
	cancel()
	if err != nil {
		return ExecutableIdentity{}, err
	}
	challenge := filepath.Join(workspace, "challenge.txt")
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return ExecutableIdentity{}, fmt.Errorf("create Codex verification challenge: %w", err)
	}
	marker := hex.EncodeToString(value)
	if err := os.WriteFile(challenge, []byte(marker+"\n"), 0o600); err != nil {
		return ExecutableIdentity{}, fmt.Errorf("write Codex verification challenge: %w", err)
	}
	session := ""
	for index, prompt := range []string{
		"Use a shell tool to read challenge.txt in the current directory. Reply with exactly its contents.",
		"Without reading files or using tools again, reply with exactly the contents of the file you read in the previous turn.",
	} {
		if index == 1 {
			if err := os.Remove(challenge); err != nil {
				return ExecutableIdentity{}, fmt.Errorf("remove Codex verification challenge before recall: %w", err)
			}
		}
		outputPath := filepath.Join(workspace, "response.txt")
		plan, err := VerificationPlan(binding.Executable, isolated, outputPath, clientRuntime, prompt, session)
		if err != nil {
			return ExecutableIdentity{}, err
		}
		session, err = verifyTurn(ctx, runner, plan, outputPath, session, marker, explicitRoute)
		if err != nil {
			return ExecutableIdentity{}, err
		}
	}
	return identity, nil
}

func verifyTurn(ctx context.Context, runner process.VerificationRunner, plan process.Plan, outputPath, sessionID, marker, explicitRoute string) (string, error) {
	if err := os.Remove(outputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove preceding Codex final response: %w", err)
	}
	turnCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	output, diagnostic, err := runner.RunCaptureStreams(turnCtx, plan)
	if err != nil {
		decoder := json.NewDecoder(bytes.NewReader(output))
		for {
			var event nativeEvent
			if decoder.Decode(&event) != nil {
				break
			}
			if event.Type == "error" || event.Type == "turn.failed" {
				diagnostic = fmt.Appendf(diagnostic, "\n%s\n%s", event.Message, event.Error.Message)
			}
		}
		return "", clientverification.Failure("Codex", ID, explicitRoute, diagnostic, err, nil)
	}
	if process.DiagnosticFailure(diagnostic) {
		return "", clientverification.Diagnostic("Codex", diagnostic)
	}
	finalMessage, err := clientverification.ReadBoundedFile(outputPath, int64(len(marker)+2))
	if err != nil {
		return "", fmt.Errorf("read Codex final response: %w", err)
	}
	if strings.TrimSpace(string(finalMessage)) != marker {
		return "", fmt.Errorf("Codex final response did not return the private verification challenge")
	}
	return turnEvidence(output, sessionID, marker)
}

type nativeEvent struct {
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

func (event nativeEvent) challengeEvidence(marker, expectedSession string) (tool, message bool, err error) {
	switch event.Item.Type {
	case "error":
		return false, false, clientverification.Diagnostic("Codex", []byte(event.Item.Message))
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

func turnEvidence(output []byte, expectedSession, marker string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	var session string
	var finalMessage, completed bool
	toolPending := expectedSession == ""
	for {
		var event nativeEvent
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
