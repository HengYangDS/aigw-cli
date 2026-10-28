package claude

import (
	"context"
	"errors"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

// SettingsSurface identifies Claude Code's user settings file.
const SettingsSurface surfaceidentity.ID = "claude-settings"

// claudeAdapter is the Claude Code client adapter behind the client seam.
type claudeAdapter struct{}

// New returns the Claude Code adapter for the composition root to register.
func New() client.Adapter { return claudeAdapter{} }

func (claudeAdapter) Spec() configuration.ClientSpec {
	return client.AdmittedSpec(configuration.ClientClaude)
}

func (claudeAdapter) Discover(source client.DiscoverySource) discovery.Result {
	return discovery.Result{Executables: map[string]string{configuration.ClientClaude: source.Executable(configuration.ClientClaude)}}
}

func (claudeAdapter) Converge(deps client.Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
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
	executable, err := client.ResolveExecutable(configuration.ClientClaude, adapter.Executable, discovered.Executable(configuration.ClientClaude))
	if err != nil {
		return err
	}
	available := !runtime.UsesAIGWCredentialStore()
	if runtime.UsesAIGWCredentialStore() {
		available, err = client.SecretAvailable(deps.Secrets, runtime.AccountID)
		if err != nil {
			return err
		}
	}
	if executable != "" && available {
		cfg.SetClientActivation(configuration.ClientClaude, true, executable, adapter.Targets)
	}
	return nil
}

func (claudeAdapter) Plan(deps client.Dependencies, before, after configuration.Config) ([]client.ProjectionPlan, error) {
	if !projectionRequired(before, after) {
		return nil, nil
	}
	disabled, runtime, err := projectionInput(after)
	if err != nil {
		return nil, err
	}
	previous, _ := before.ResolveRuntime(configuration.ClientClaude, "")
	settings, err := SettingsPath(deps.Host)
	if err != nil {
		return nil, err
	}
	plan, err := PlanSettings(settings, disabled, runtime, runtime.CredentialExecutable(deps.AIGWExecutable), previous.Model)
	if err != nil {
		return nil, err
	}
	return []client.ProjectionPlan{{Client: configuration.ClientClaude, Target: plan.Target, Action: string(plan.Action), Surface: string(SettingsSurface), Effects: plan.Effects(), ChangesState: plan.ChangesState()}}, nil
}

func (claudeAdapter) Apply(_ context.Context, deps client.Dependencies, before, after configuration.Config) (client.ProjectionReceipt, error) {
	if !projectionRequired(before, after) {
		return nil, nil
	}
	disabled, runtime, err := projectionInput(after)
	if err != nil {
		return nil, err
	}
	previous, _ := before.ResolveRuntime(configuration.ClientClaude, "")
	settings, err := SettingsPath(deps.Host)
	if err != nil {
		return nil, err
	}
	return ReconcileSettings(settings, disabled, runtime, runtime.CredentialExecutable(deps.AIGWExecutable), previous.Model)
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

func (claudeAdapter) Inspect(_ context.Context, deps client.Dependencies, cfg configuration.Config, runtime configuration.Runtime) client.Status {
	adapter := cfg.Clients[configuration.ClientClaude]
	if !adapter.Enabled {
		return client.Status{Issue: "Claude adapter is disabled", RepairAction: "aigw sync"}
	}
	if adapter.Executable == "" {
		return client.Status{Issue: "Claude executable is not configured", RepairAction: "aigw repair"}
	}
	ready, err := discovery.ExecutableAvailable(adapter.Executable)
	if err != nil {
		return client.Status{Issue: "Cannot inspect Claude executable", RepairAction: "aigw repair"}
	}
	if !ready {
		return client.Status{Issue: "Claude executable is unavailable", RepairAction: "aigw repair"}
	}
	settings, err := SettingsPath(deps.Host)
	if err != nil {
		return client.Status{Issue: "Claude settings are not synchronized with the selected Route", RepairAction: "aigw sync"}
	}
	reader, err := retainedExecutable(settings, cfg, runtime, deps.AIGWExecutable)
	if err != nil {
		return client.Status{Issue: "Claude credential reader is not synchronized with the selected Route", RepairAction: "aigw sync"}
	}
	inspection, err := InspectSettings(settings, runtime, reader)
	if err != nil {
		return client.Status{Issue: "Claude settings are not synchronized with the selected Route", RepairAction: "aigw sync"}
	}
	return client.Status{Ready: true, NativeModelOverride: inspection.NativeModelOverride}
}

// AdoptSelection adopts nothing: Claude Code has no native selection that AIGW
// records as its Client Binding.
func (claudeAdapter) AdoptSelection(client.Dependencies, *configuration.Config) (string, error) {
	return "", nil
}

func (claudeAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, configuration.ClientClaude)
}

func projectionRequired(before, after configuration.Config) bool {
	previous := before.Clients[configuration.ClientClaude]
	current := after.Clients[configuration.ClientClaude]
	return previous.Enabled && previous.Executable != "" || current.Enabled && current.Executable != ""
}

func projectionInput(cfg configuration.Config) (bool, configuration.Runtime, error) {
	disabled := !cfg.Clients[configuration.ClientClaude].Enabled
	if disabled {
		return true, configuration.Runtime{}, nil
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	return false, runtime, err
}
