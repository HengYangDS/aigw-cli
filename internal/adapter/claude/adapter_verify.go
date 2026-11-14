package claude

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"

	"github.com/rogpeppe/go-internal/robustio"
)

func retainedExecutable(path string, cfg domain.Config, runtime domain.Runtime, currentReader string) (string, error) {
	if !runtime.RequiresProviderToken() {
		return runtime.CredentialExecutable(currentReader), nil
	}
	reader, err := adapter.RetainedDefaultReader(currentReader, cfg.Clients[ID].CredentialCommand != "", func() (string, error) {
		return ObservedCredentialExecutable(path, runtime)
	})
	if err != nil {
		return "", fmt.Errorf("inspect retained Claude credential reader: %w", err)
	}
	return runtime.CredentialExecutable(reader), nil
}

func (claudeAdapter) Verify(ctx context.Context, deps adapter.Dependencies, cfg domain.Config, runtime domain.Runtime, explicitRoute string) (_ adapter.Verification, result error) {
	clientAdapter := cfg.Clients[ID]
	token := ""
	if clientAdapter.CredentialCommand == "" {
		if deps.Secrets == nil {
			return adapter.Verification{}, fmt.Errorf("Token for provider %q is unavailable: secret store is unavailable", runtime.ProviderID)
		}
		var err error
		token, err = deps.Secrets.Get(runtime.ProviderID)
		if err != nil {
			instruction, _ := secret.TokenRecovery(deps.Secrets, runtime.ProviderID)
			return adapter.Verification{}, fmt.Errorf("Token for provider %q is unavailable: %w; %s", runtime.ProviderID, err, instruction)
		}
	}
	if !clientAdapter.Enabled || clientAdapter.Executable == "" {
		return adapter.Verification{}, fmt.Errorf("claude adapter is disabled; run `aigw sync`")
	}
	ready, err := discovery.ExecutableAvailable(clientAdapter.Executable)
	if err != nil {
		return adapter.Verification{}, fmt.Errorf("inspect Claude executable: %w", err)
	}
	if !ready {
		return adapter.Verification{}, fmt.Errorf("claude executable is unavailable; run `aigw sync`")
	}
	settings, err := SettingsPath(deps.Host)
	if err != nil {
		return adapter.Verification{}, err
	}
	verifyCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	reader := deps.AIGWExecutable
	if !clientAdapter.Model.IsZero() {
		selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
		if err != nil {
			return adapter.Verification{}, err
		}
		reader, err = retainedExecutable(settings, cfg, selected, reader)
		if err != nil {
			return adapter.Verification{}, err
		}
	}
	runtime.CredentialCommand = runtime.CredentialExecutable(reader)
	settingsPath := settings
	if explicitRoute != "" {
		isolated, workspace, err := isolateProjection(cfg, runtime, settings)
		if err != nil {
			return adapter.Verification{}, err
		}
		defer func() {
			if err := robustio.RemoveAll(workspace); err != nil {
				result = errors.Join(result, fmt.Errorf("remove isolated Claude verification files %s: %w", workspace, err))
			}
		}()
		settingsPath = isolated
	}
	runner, external := deps.Runner, clientAdapter.CredentialCommand != ""
	if external && runner != nil {
		runner = adapter.SuppressExternalCredentialDiagnostics(runner)
	}
	err = verifyRuntime(verifyCtx, runner, clientAdapter.Executable, settingsPath, runtime, token, explicitRoute)
	return adapter.Verification{Scope: "Native text response; tools and continuation are not probed"}, err
}

func isolateProjection(cfg domain.Config, runtime domain.Runtime, source string) (settingsPath, workspace string, result error) {
	workspace, err := os.MkdirTemp("", "aigw-claude-route-verification-")
	if err != nil {
		return "", "", fmt.Errorf("create isolated Claude verification files: %w", err)
	}
	defer func() {
		if result == nil {
			return
		}
		if err := robustio.RemoveAll(workspace); err != nil {
			result = errors.Join(result, fmt.Errorf("remove failed Claude verification files %s: %w", workspace, err))
			return
		}
		workspace = ""
	}()
	settingsPath = filepath.Join(workspace, "settings.json")
	if err := CopyProjection(source, settingsPath); err != nil {
		return "", workspace, fmt.Errorf("copy Claude verification files: %w", err)
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return "", workspace, fmt.Errorf("resolve selected Claude client files: %w", err)
	}
	if _, err := ReconcileSettings(settingsPath, false, runtime, runtime.CredentialCommand, selected.UpstreamModel); err != nil {
		return "", workspace, fmt.Errorf("prepare isolated Claude verification files: %w", err)
	}
	return settingsPath, workspace, nil
}
