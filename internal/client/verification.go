package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aigw-cli/internal/claude"
	clientverification "aigw-cli/internal/client/verification"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"

	"github.com/rogpeppe/go-internal/robustio"
)

func claudeRetainedExecutable(path string, cfg configuration.Config, runtime configuration.Runtime, currentReader string) (string, error) {
	if !runtime.RequiresAccountToken() {
		return runtime.CredentialExecutable(currentReader), nil
	}
	reader, err := RetainedDefaultReader(currentReader, cfg.Clients[configuration.ClientClaude].CredentialCommand != "", func() (string, error) {
		return claude.ObservedCredentialExecutable(path, runtime)
	})
	if err != nil {
		return "", fmt.Errorf("inspect retained Claude credential reader: %w", err)
	}
	return runtime.CredentialExecutable(reader), nil
}

func (claudeAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime, explicitRoute string) (_ Verification, result error) {
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
	reader := deps.AIGWExecutable
	if adapter.Route != "" {
		selected, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
		if err != nil {
			return Verification{}, err
		}
		reader, err = claudeRetainedExecutable(deps.ClaudeSettingsPath, cfg, selected, reader)
		if err != nil {
			return Verification{}, err
		}
	}
	runtime.CredentialCommand = runtime.CredentialExecutable(reader)
	settingsPath := deps.ClaudeSettingsPath
	if explicitRoute != "" {
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
		runner = SuppressExternalCredentialDiagnostics(runner)
	}
	return Verification{Scope: "Native text response; tools and continuation are not probed"}, clientverification.VerifyClaudeRuntime(verifyCtx, runner, adapter.Executable, settingsPath, runtime, token, explicitRoute)
}

func isolateClaudeProjection(cfg configuration.Config, runtime configuration.Runtime, deps Dependencies) (settingsPath, workspace string, result error) {
	workspace, err := os.MkdirTemp("", "aigw-claude-route-verification-")
	if err != nil {
		return "", "", fmt.Errorf("create isolated Claude verification projection: %w", err)
	}
	defer func() {
		if result == nil {
			return
		}
		if err := robustio.RemoveAll(workspace); err != nil {
			result = errors.Join(result, fmt.Errorf("remove failed Claude verification projection %s: %w", workspace, err))
			return
		}
		workspace = ""
	}()
	settingsPath = filepath.Join(workspace, "settings.json")
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
