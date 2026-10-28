package client

import (
	"context"
	"errors"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

// NewClaudeAdapter returns the Claude Code adapter for the composition root to register.
func NewClaudeAdapter() Adapter { return claudeAdapter{} }

// NewClaudeDesktopAdapter returns the Claude Desktop adapter for the composition root to register.
func NewClaudeDesktopAdapter() Adapter { return claudeDesktopAdapter{} }

// NewHermesAdapter returns the Hermes adapter for the composition root to register.
func NewHermesAdapter() Adapter { return hermesAdapter{} }

// NewDiscoverer binds an adapter registry to one host observation source.
func NewDiscoverer(registry Registry, source DiscoverySource) discovery.Discoverer {
	return registeredDiscoverer{registry: registry, source: source}
}

type registeredDiscoverer struct {
	registry Registry
	source   DiscoverySource
}

func (discoverer registeredDiscoverer) Discover() discovery.Result {
	return discoverer.registry.Discover(discoverer.source)
}

type claudeAdapter struct{}

func (claudeAdapter) Spec() configuration.ClientSpec {
	return AdmittedSpec(configuration.ClientClaude)
}

func (claudeAdapter) Discover(source DiscoverySource) discovery.Result {
	return discovery.Result{Executables: map[string]string{configuration.ClientClaude: source.Executable(configuration.ClientClaude)}}
}

func (claudeAdapter) Converge(deps Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		if _, unselected := errors.AsType[*configuration.RuntimeBindingUnselectedError](err); unselected {
			return nil
		}
		return err
	}
	adapter, explicitlyConfigured := cfg.Clients[configuration.ClientClaude]
	if !explicitlyConfigured || !adapter.Enabled {
		return nil
	}
	executable, err := ResolveExecutable(configuration.ClientClaude, adapter.Executable, discovered.Executable(configuration.ClientClaude))
	if err != nil {
		return err
	}
	available := !runtime.UsesAIGWCredentialStore()
	if runtime.UsesAIGWCredentialStore() {
		available, err = SecretAvailable(deps.Secrets, runtime.AccountID)
		if err != nil {
			return err
		}
	}
	if executable != "" && available {
		cfg.SetClientActivation(configuration.ClientClaude, true, executable, adapter.Targets)
	}
	return nil
}

func (claudeAdapter) Plan(deps Dependencies, before, after configuration.Config) ([]ProjectionPlan, error) {
	if !claudeProjectionRequired(before, after) {
		return nil, nil
	}
	disabled, runtime, err := claudeProjectionInput(after)
	if err != nil {
		return nil, err
	}
	previous, _ := before.ResolveRuntime(configuration.ClientClaude, "")
	plan, err := claude.PlanSettings(deps.ClaudeSettingsPath, disabled, runtime, runtime.CredentialExecutable(deps.AIGWExecutable), previous.Model)
	if err != nil {
		return nil, err
	}
	return []ProjectionPlan{{Client: configuration.ClientClaude, Target: plan.Target, Action: string(plan.Action), Surface: string(surfaceidentity.ClaudeSettings), ChangesState: plan.ChangesState()}}, nil
}

func (claudeAdapter) Apply(_ context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	if !claudeProjectionRequired(before, after) {
		return nil, nil
	}
	disabled, runtime, err := claudeProjectionInput(after)
	if err != nil {
		return nil, err
	}
	previous, _ := before.ResolveRuntime(configuration.ClientClaude, "")
	return claude.ReconcileSettings(deps.ClaudeSettingsPath, disabled, runtime, runtime.CredentialExecutable(deps.AIGWExecutable), previous.Model)
}

func (claudeAdapter) ProjectionChanged(before, after configuration.Config) bool {
	beforeAdapter := before.Clients[configuration.ClientClaude]
	afterAdapter := after.Clients[configuration.ClientClaude]
	if beforeAdapter.Enabled != afterAdapter.Enabled || beforeAdapter.CredentialCommand != afterAdapter.CredentialCommand {
		return true
	}
	if !afterAdapter.Enabled {
		return false
	}
	beforeRuntime, beforeErr := before.ResolveRuntime(configuration.ClientClaude, "")
	afterRuntime, afterErr := after.ResolveRuntime(configuration.ClientClaude, "")
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return !beforeRuntime.SameUpstream(afterRuntime) || beforeRuntime.Endpoint != afterRuntime.Endpoint || beforeRuntime.Model != afterRuntime.Model
}

func (claudeAdapter) Inspect(_ context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime) Status {
	adapter := cfg.Clients[configuration.ClientClaude]
	if !adapter.Enabled {
		return Status{Issue: "Claude adapter is disabled", RepairAction: "aigw sync"}
	}
	if adapter.Executable == "" {
		return Status{Issue: "Claude executable is not configured", RepairAction: "aigw repair"}
	}
	ready, err := discovery.ExecutableAvailable(adapter.Executable)
	if err != nil {
		return Status{Issue: "Cannot inspect Claude executable", RepairAction: "aigw repair"}
	}
	if !ready {
		return Status{Issue: "Claude executable is unavailable", RepairAction: "aigw repair"}
	}
	reader, err := claudeRetainedExecutable(deps.ClaudeSettingsPath, cfg, runtime, deps.AIGWExecutable)
	if err != nil {
		return Status{Issue: "Claude credential reader is not synchronized with the selected Route", RepairAction: "aigw sync"}
	}
	inspection, err := claude.InspectSettings(deps.ClaudeSettingsPath, runtime, reader)
	if err != nil {
		return Status{Issue: "Claude settings are not synchronized with the selected Route", RepairAction: "aigw sync"}
	}
	return Status{Ready: true, NativeModelOverride: inspection.NativeModelOverride}
}

func (claudeAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, configuration.ClientClaude)
}

func claudeProjectionRequired(before, after configuration.Config) bool {
	previous := before.Clients[configuration.ClientClaude]
	current := after.Clients[configuration.ClientClaude]
	return previous.Enabled && previous.Executable != "" || current.Enabled && current.Executable != ""
}

func claudeProjectionInput(cfg configuration.Config) (bool, configuration.Runtime, error) {
	disabled := !cfg.Clients[configuration.ClientClaude].Enabled
	if disabled {
		return true, configuration.Runtime{}, nil
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	return false, runtime, err
}
