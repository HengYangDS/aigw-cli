// Package verification owns explicit, quota-consuming live model probes.
package verification

import (
	"context"
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
)

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
const responseLimit int64 = int64(len(responseSentinel) + 2)

// VerifyCodexInvocation validates one synchronized Codex target, measures the
// configured executable, and makes exactly one non-persistent client request.
func VerifyCodexInvocation(ctx context.Context, runner Runner, cfg configuration.Config, clientRuntime configuration.Runtime) (codex.ExecutableIdentity, error) {
	adapter := cfg.Adapters[configuration.ClientCodex]
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
		return codex.ExecutableIdentity{}, fmt.Errorf("Profile %q has no Codex model", clientRuntime.ProfileID)
	}
	targets := append([]string(nil), adapter.Targets...)
	sort.Strings(targets)
	target := targets[0]
	if err := codex.ValidateConfig(target, clientRuntime); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex configuration target is not synchronized: %w; run `aigw sync`", err)
	}
	captureRunner, ok := runner.(process.CaptureRunner)
	if !ok {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex verification capture runner is unavailable")
	}
	identity, err := codex.IdentifyExecutable(ctx, captureRunner, adapter.Executable, filepath.Dir(target))
	if err != nil {
		return codex.ExecutableIdentity{}, err
	}
	output, err := os.CreateTemp("", "aigw-codex-verification-*.txt")
	if err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("create Codex verification output: %w", err)
	}
	outputPath := output.Name()
	defer func() { _ = os.Remove(outputPath) }()
	if err := output.Close(); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("close Codex verification output: %w", err)
	}
	plan, err := codex.VerificationPlan(adapter.Executable, target, outputPath, clientRuntime)
	if err != nil {
		return codex.ExecutableIdentity{}, err
	}
	diagnostic, err := captureRunner.RunCapture(ctx, plan)
	if err != nil {
		return codex.ExecutableIdentity{}, verificationFailure("Codex", configuration.ClientCodex, diagnostic, err)
	}
	finalMessage, err := readBoundedFile(outputPath, responseLimit)
	if err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("read Codex final response: %w", err)
	}
	if strings.TrimSpace(string(finalMessage)) != responseSentinel {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex model response did not return the expected AIGW_OK verification marker")
	}
	return identity, nil
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
	output, err := captureRunner.RunCapture(ctx, plan)
	if err != nil {
		return verificationFailure("Claude", configuration.ClientClaude, output, err, token)
	}
	if strings.TrimSpace(string(output)) != responseSentinel {
		return fmt.Errorf("Claude model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}

func verificationFailure(label, client string, diagnostic []byte, cause error, secrets ...string) error {
	detail := strings.Join(strings.Fields(redaction.Text(string(diagnostic), secrets...)), " ")
	next := "aigw verify --for " + client
	if detail == "" {
		return fmt.Errorf("%s minimal verification request failed: %w; inspect the client error, then run `%s`", label, cause, next)
	}
	return fmt.Errorf("%s minimal verification request failed: %s; correct the reported client error, then run `%s`: %w", label, detail, next, cause)
}
