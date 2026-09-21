package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"aigw-cli/internal/claude"
	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	surfaceidentity "aigw-cli/internal/surface"

	"github.com/rogpeppe/go-internal/robustio"
)

// externalCredentialRunner suppresses unknown external-helper credentials on
// failure while retaining the existing verifier's response-marker semantics.
type externalCredentialRunner struct{ runner process.CaptureRunner }

func (runner externalCredentialRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	output, err := runner.runner.RunCapture(ctx, plan)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("external credential client failed; diagnostics suppressed")
	}
	return output, nil
}

func (codexAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime, explicitProfile string) (_ Verification, result error) {
	if deps.AIGWExecutable != "" {
		runtime.CredentialCommand = runtime.CredentialExecutable(deps.AIGWExecutable)
	}
	adapter := cfg.Clients[configuration.ClientCodex]
	if explicitProfile != "" && adapter.Enabled && adapter.Executable != "" && len(adapter.Targets) > 0 {
		isolated, workspace, err := isolateCodexProjection(cfg, runtime, adapter)
		if err != nil {
			return Verification{}, err
		}
		defer func() {
			if err := robustio.RemoveAll(workspace); err != nil {
				result = errors.Join(result, fmt.Errorf("remove isolated Codex verification projection %s: %w", workspace, err))
			}
		}()
		cfg = isolated
	}
	verifyCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	runner := deps.Runner
	if cfg.Clients[configuration.ClientCodex].CredentialCommand != "" && runner != nil {
		runner = externalCredentialRunner{runner: runner}
	}
	identity, err := clientverification.VerifyCodexInvocation(verifyCtx, runner, cfg, runtime)
	return Verification{Version: identity.Version, SHA256: identity.SHA256}, err
}

func (claudeAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime, explicitProfile string) (_ Verification, result error) {
	adapter := cfg.Clients[configuration.ClientClaude]
	token := ""
	if adapter.CredentialCommand == "" {
		if deps.Secrets == nil {
			return Verification{}, fmt.Errorf("token for account %q is unavailable: secret store is unavailable", runtime.AccountID)
		}
		var err error
		token, err = deps.Secrets.Get(runtime.AccountID)
		if err != nil {
			instruction, _ := credential.TokenRecovery(deps.Secrets, runtime.AccountID)
			return Verification{}, fmt.Errorf("token for account %q is unavailable: %w; %s", runtime.AccountID, err, instruction)
		}
	}
	if !adapter.Enabled || adapter.Executable == "" {
		return Verification{}, fmt.Errorf("claude adapter is disabled; run `aigw repair`")
	}
	ready, err := discovery.ExecutableAvailable(adapter.Executable)
	if err != nil {
		return Verification{}, fmt.Errorf("inspect Claude executable: %w", err)
	}
	if !ready {
		return Verification{}, fmt.Errorf("claude executable is unavailable; run `aigw repair`")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	runtime.CredentialCommand = runtime.CredentialExecutable(deps.AIGWExecutable)
	settingsPath := deps.ClaudeSettingsPath
	if explicitProfile != "" {
		isolated, workspace, err := isolateClaudeProjection(cfg, runtime, deps)
		if err != nil {
			return Verification{}, err
		}
		defer func() {
			if err := robustio.RemoveAll(workspace); err != nil {
				result = errors.Join(result, fmt.Errorf("remove isolated Claude verification projection %s: %w", workspace, err))
			}
		}()
		settingsPath = isolated
	}
	runner := deps.Runner
	if adapter.CredentialCommand != "" && runner != nil {
		runner = externalCredentialRunner{runner: runner}
	}
	return Verification{}, clientverification.VerifyClaudeRuntime(verifyCtx, runner, adapter.Executable, settingsPath, runtime, token)
}

func isolateCodexProjection(cfg configuration.Config, runtime configuration.Runtime, adapter configuration.ClientBinding) (configuration.Config, string, error) {
	workspace, err := os.MkdirTemp("", "aigw-codex-profile-verification-")
	if err != nil {
		return configuration.Config{}, "", fmt.Errorf("create isolated Codex verification projection: %w", err)
	}
	selectedTargets := append([]string(nil), adapter.Targets...)
	sort.Strings(selectedTargets)
	target := filepath.Join(workspace, "config.toml")
	if err := codex.CopyProjection(selectedTargets[0], target); err != nil {
		return configuration.Config{}, workspace, fmt.Errorf("copy Codex verification input: %w", err)
	}
	before := codex.TargetRef{SurfaceID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ProjectionMode: codex.ProjectionFullSelection, Path: target}
	after := before
	after.Executable = adapter.Executable
	after.CreateIfAbsent = true
	if _, err := codex.ReconcileConfigs([]codex.TargetRef{before}, []codex.TargetRef{after}, runtime); err != nil {
		return configuration.Config{}, workspace, fmt.Errorf("prepare isolated Codex verification projection: %w", err)
	}
	isolated := cfg.Clone()
	adapter.Targets = []string{target}
	isolated.Clients[configuration.ClientCodex] = adapter
	return isolated, workspace, nil
}

func isolateClaudeProjection(cfg configuration.Config, runtime configuration.Runtime, deps Dependencies) (string, string, error) {
	workspace, err := os.MkdirTemp("", "aigw-claude-profile-verification-")
	if err != nil {
		return "", "", fmt.Errorf("create isolated Claude verification projection: %w", err)
	}
	settingsPath := filepath.Join(workspace, "settings.json")
	if err := claude.CopyProjection(deps.ClaudeSettingsPath, settingsPath); err != nil {
		return "", workspace, fmt.Errorf("copy Claude verification projection: %w", err)
	}
	selected, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return "", workspace, fmt.Errorf("resolve selected Claude projection: %w", err)
	}
	if _, err := claude.ReconcileSettings(settingsPath, false, runtime, runtime.CredentialCommand, selected.Model); err != nil {
		return "", workspace, fmt.Errorf("prepare isolated Claude verification projection: %w", err)
	}
	return settingsPath, workspace, nil
}
