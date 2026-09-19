// Package verification owns explicit, quota-consuming live model probes.
package verification

import (
	"context"
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

// ProtocolTimeout allows a cold Claude CLI process to initialize and complete
// one bounded upstream request.
const ProtocolTimeout = time.Minute

const responseSentinel = "AIGW_OK"
const responseLimit int64 = int64(len(responseSentinel) + 2)

var removeCodexWorkspace = robustio.RemoveAll

// VerifyCodexInvocation validates one synchronized Codex target, measures the
// configured executable, and makes exactly one non-persistent client request.
func VerifyCodexInvocation(ctx context.Context, runner process.CaptureRunner, cfg configuration.Config, clientRuntime configuration.Runtime) (_ codex.ExecutableIdentity, result error) {
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
	if runner == nil {
		return codex.ExecutableIdentity{}, fmt.Errorf("Codex verification capture runner is unavailable")
	}
	identity, err := codex.IdentifyExecutable(ctx, runner, adapter.Executable, filepath.Dir(target))
	if err != nil {
		return codex.ExecutableIdentity{}, err
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
	outputPath := filepath.Join(workspace, "response.txt")
	plan, err := codex.VerificationPlan(adapter.Executable, target, outputPath, clientRuntime)
	if err != nil {
		return codex.ExecutableIdentity{}, err
	}
	diagnostic, err := runner.RunCapture(ctx, plan)
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
func VerifyClaudeRuntime(ctx context.Context, runner process.CaptureRunner, executable, settingsPath string, clientRuntime configuration.Runtime, token string) error {
	if clientRuntime.Model == "" {
		return fmt.Errorf("Profile %q has no Claude model", clientRuntime.ProfileID)
	}
	plan, err := claude.VerificationPlan(executable, settingsPath, "Reply with exactly: AIGW_OK", os.Environ(), clientRuntime)
	if err != nil {
		return err
	}
	if runner == nil {
		return fmt.Errorf("Claude verification runner is unavailable")
	}
	output, err := runner.RunCapture(ctx, plan)
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
