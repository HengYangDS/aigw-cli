package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"aigw-cli/internal/client"
	clientverification "aigw-cli/internal/client/verification"
	configuration "aigw-cli/internal/configuration"
	surfaceidentity "aigw-cli/internal/surface"

	"github.com/rogpeppe/go-internal/robustio"
)

func (codexAdapter) Verify(ctx context.Context, deps client.Dependencies, cfg configuration.Config, runtime configuration.Runtime, explicitRoute string) (_ client.Verification, result error) {
	binding := cfg.Clients[configuration.ClientCodex]
	reader := deps.AIGWExecutable
	if binding.Enabled && binding.Executable != "" && len(binding.Targets) > 0 && binding.Route != "" {
		targets := append([]string(nil), binding.Targets...)
		sort.Strings(targets)
		selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
		if err != nil {
			return client.Verification{}, err
		}
		selected, err = retainedRuntime(targets[0], cfg, selected, reader)
		if err != nil {
			return client.Verification{}, err
		}
		if selected.CredentialCommand != "" {
			reader = selected.CredentialCommand
		}
	}
	runtime.CredentialCommand = runtime.CredentialExecutable(reader)
	if explicitRoute != "" && binding.Enabled && binding.Executable != "" && len(binding.Targets) > 0 {
		isolated, workspace, err := isolateProjection(cfg, runtime, binding)
		if err != nil {
			return client.Verification{}, err
		}
		defer func() {
			if err := robustio.RemoveAll(workspace); err != nil {
				result = errors.Join(result, fmt.Errorf("remove isolated Codex verification projection %s: %w", workspace, err))
			}
		}()
		cfg = isolated
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 3*clientverification.ProtocolTimeout)
	defer cancel()
	runner := deps.Runner
	if cfg.Clients[configuration.ClientCodex].CredentialCommand != "" && runner != nil {
		runner = client.SuppressExternalCredentialDiagnostics(runner)
	}
	identity, err := VerifyInvocation(verifyCtx, runner, cfg, runtime, explicitRoute)
	return client.Verification{Version: identity.Version, SHA256: identity.SHA256, Scope: "Native file-tool execution and same-session recall"}, err
}

func retainedRuntime(target string, cfg configuration.Config, runtime configuration.Runtime, currentReader string) (configuration.Runtime, error) {
	if !runtime.RequiresAccountToken() || currentReader == "" {
		return runtime, nil
	}
	if cfg.Clients[configuration.ClientCodex].CredentialCommand != "" {
		runtime.CredentialCommand = runtime.CredentialExecutable(currentReader)
		return runtime, nil
	}
	runtime.CredentialCommand = currentReader
	reader, err := client.RetainedDefaultReader(currentReader, false, func() (string, error) {
		return ObservedCredentialCommand(target, runtime)
	})
	if err != nil {
		return runtime, fmt.Errorf("inspect retained Codex credential reader: %w", err)
	}
	runtime.CredentialCommand = reader
	return runtime, nil
}

func isolateProjection(cfg configuration.Config, runtime configuration.Runtime, binding configuration.ClientBinding) (isolated configuration.Config, workspace string, result error) {
	workspace, err := os.MkdirTemp("", "aigw-codex-route-verification-")
	if err != nil {
		return configuration.Config{}, "", fmt.Errorf("create isolated Codex verification projection: %w", err)
	}
	defer func() {
		if result == nil {
			return
		}
		if err := robustio.RemoveAll(workspace); err != nil {
			result = errors.Join(result, fmt.Errorf("remove failed Codex verification projection %s: %w", workspace, err))
			return
		}
		workspace = ""
	}()
	selectedTargets := append([]string(nil), binding.Targets...)
	sort.Strings(selectedTargets)
	target := filepath.Join(workspace, "config.toml")
	if err := CopyProjection(selectedTargets[0], target); err != nil {
		return configuration.Config{}, workspace, fmt.Errorf("copy Codex verification input: %w", err)
	}
	before := TargetRef{SurfaceID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ProjectionMode: ProjectionFullSelection, Path: target}
	after := before
	after.Executable = binding.Executable
	after.CreateIfAbsent = true
	selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return configuration.Config{}, workspace, fmt.Errorf("resolve selected Codex projection: %w", err)
	}
	if selected.RequiresAccountToken() {
		selected.CredentialCommand = selected.CredentialExecutable(runtime.CredentialCommand)
	}
	if _, err := ReconcileConfigsAuthorizedTransition([]TargetRef{before}, []TargetRef{after}, selected, runtime); err != nil {
		return configuration.Config{}, workspace, fmt.Errorf("prepare isolated Codex verification projection: %w", err)
	}
	isolated = cfg.Clone()
	binding.Targets = []string{target}
	isolated.Clients[configuration.ClientCodex] = binding
	return isolated, workspace, nil
}
