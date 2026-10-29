package claude

import (
	"context"
	"errors"

	"aigw-cli/internal/client"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	surfaceidentity "aigw-cli/internal/surface"
)

// ID identifies the admitted Claude Code client.
const ID = "claude"

// SettingsSurface identifies Claude Code's user settings file.
const SettingsSurface surfaceidentity.ID = "claude-settings"

// claudeAdapter is the Claude Code client adapter behind the client seam.
type claudeAdapter struct{}

// New returns the Claude Code adapter for the composition root to register.
func New() client.Adapter { return claudeAdapter{} }

func (claudeAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{
		ID: ID, Label: "Claude", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolAnthropic},
		ProductName: "Claude Code", ModelDiscoveryOptional: true, Doctor: domain.DoctorWording{AdapterStates: true},
		PublishedProfileProtocol: domain.ProtocolAnthropic,
	}
}

func (claudeAdapter) Discover(source client.DiscoverySource) discovery.Result {
	return discovery.Result{Executables: map[string]string{ID: source.Executable(ID)}}
}

func (claudeAdapter) Converge(deps client.Dependencies, cfg *domain.Config, discovered discovery.Result) error {
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		if _, unselected := errors.AsType[*domain.RuntimeBindingUnselectedError](err); unselected {
			return nil
		}
		return err
	}
	adapter, explicitlyConfigured := cfg.Clients[ID]
	if !explicitlyConfigured || !adapter.Enabled {
		return nil
	}
	executable, err := client.ResolveExecutable(ID, adapter.Executable, discovered.Executable(ID))
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
		cfg.SetClientActivation(ID, true, executable, adapter.Targets)
	}
	return nil
}

func (claudeAdapter) Plan(deps client.Dependencies, before, after domain.Config) ([]client.ProjectionPlan, error) {
	if !projectionRequired(before, after) {
		return nil, nil
	}
	disabled, runtime, err := projectionInput(after)
	if err != nil {
		return nil, err
	}
	previous, _ := before.ResolveRuntime(ID, "")
	settings, err := SettingsPath(deps.Host)
	if err != nil {
		return nil, err
	}
	plan, err := PlanSettings(settings, disabled, runtime, runtime.CredentialExecutable(deps.AIGWExecutable), previous.Model)
	if err != nil {
		return nil, err
	}
	return []client.ProjectionPlan{{Client: ID, Target: plan.Target, Action: string(plan.Action), Surface: string(SettingsSurface), Effects: plan.Effects(), ChangesState: plan.ChangesState()}}, nil
}

func (claudeAdapter) Apply(_ context.Context, deps client.Dependencies, before, after domain.Config) (client.ProjectionReceipt, error) {
	if !projectionRequired(before, after) {
		return nil, nil
	}
	disabled, runtime, err := projectionInput(after)
	if err != nil {
		return nil, err
	}
	previous, _ := before.ResolveRuntime(ID, "")
	settings, err := SettingsPath(deps.Host)
	if err != nil {
		return nil, err
	}
	return ReconcileSettings(settings, disabled, runtime, runtime.CredentialExecutable(deps.AIGWExecutable), previous.Model)
}

func (claudeAdapter) ProjectionChanged(before, after domain.Config) bool {
	beforeAdapter := before.Clients[ID]
	afterAdapter := after.Clients[ID]
	if beforeAdapter.Enabled != afterAdapter.Enabled || beforeAdapter.CredentialCommand != afterAdapter.CredentialCommand {
		return true
	}
	if !afterAdapter.Enabled {
		return false
	}
	beforeRuntime, beforeErr := before.ResolveRuntime(ID, "")
	afterRuntime, afterErr := after.ResolveRuntime(ID, "")
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return !beforeRuntime.SameUpstream(afterRuntime) || beforeRuntime.Endpoint != afterRuntime.Endpoint || beforeRuntime.Model != afterRuntime.Model
}

func (claudeAdapter) Inspect(_ context.Context, deps client.Dependencies, cfg domain.Config, runtime domain.Runtime) client.Status {
	adapter := cfg.Clients[ID]
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
func (claudeAdapter) AdoptSelection(client.Dependencies, *domain.Config) (string, error) {
	return "", nil
}

func (claudeAdapter) Withdraw(cfg *domain.Config) {
	delete(cfg.Clients, ID)
}

func projectionRequired(before, after domain.Config) bool {
	previous := before.Clients[ID]
	current := after.Clients[ID]
	return previous.Enabled && previous.Executable != "" || current.Enabled && current.Executable != ""
}

func projectionInput(cfg domain.Config) (bool, domain.Runtime, error) {
	disabled := !cfg.Clients[ID].Enabled
	if disabled {
		return true, domain.Runtime{}, nil
	}
	runtime, err := cfg.ResolveRuntime(ID, "")
	return false, runtime, err
}
