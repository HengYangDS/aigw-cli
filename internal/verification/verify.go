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
	if _, err := captureRunner.RunCapture(ctx, plan); err != nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex minimal verification request failed: %w", err)
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
