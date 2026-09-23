package client

import (
	"context"
	"errors"
	"slices"

	claudedesktop "aigw-cli/internal/claude/desktop"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

type claudeDesktopAdapter struct{}

func (claudeDesktopAdapter) Spec() configuration.ClientSpec {
	return mustClientSpec(configuration.ClientClaudeDesktop)
}

func (claudeDesktopAdapter) Discover(source DiscoverySource) discovery.Result {
	executable := source.ClaudeDesktopExecutable()
	library := source.ClaudeDesktopLibraryDirectory()
	return discovery.Result{
		Executables: map[string]string{configuration.ClientClaudeDesktop: executable},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.ClaudeDesktopLibrary),
			Product:     "Claude Desktop",
			Authority:   string(surfaceidentity.AuthorityAIGW),
			Executable:  executable,
			ConfigPath:  library,
			Present:     executable != "",
			AutoManaged: true,
		}},
	}
}

func (claudeDesktopAdapter) Converge(deps Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaudeDesktop, "")
	if err != nil {
		if _, unselected := errors.AsType[*configuration.RuntimeBindingUnselectedError](err); unselected {
			return nil
		}
		return err
	}
	binding, explicitlyConfigured := cfg.Clients[configuration.ClientClaudeDesktop]
	if !explicitlyConfigured || !binding.Enabled {
		return nil
	}
	executable, err := resolveExecutable(configuration.ClientClaudeDesktop, binding.Executable, discovered.Executable(configuration.ClientClaudeDesktop))
	if err != nil {
		return err
	}
	available := !runtime.UsesAIGWCredentialStore()
	if runtime.UsesAIGWCredentialStore() {
		available, err = secretAvailable(deps.Secrets, runtime.AccountID)
		if err != nil {
			return err
		}
	}
	target := claudeDesktopTarget(binding.Targets, discovered)
	if executable != "" && target != "" && available {
		cfg.SetClientActivation(configuration.ClientClaudeDesktop, true, executable, []string{target})
	}
	return nil
}

func (claudeDesktopAdapter) Plan(deps Dependencies, before, after configuration.Config) ([]ProjectionPlan, error) {
	plans, targets, err := claudeDesktopPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectionPlan, 0, len(plans))
	for index, plan := range plans {
		result = append(result, ProjectionPlan{Client: configuration.ClientClaudeDesktop, Target: targets[index], Action: string(plan.Action), ChangesState: plan.ChangesState()})
	}
	return result, nil
}

type claudeDesktopReceipt []claudedesktop.Receipt

func (receipt claudeDesktopReceipt) Rollback() error {
	var result error
	for _, item := range slices.Backward(receipt) {
		result = errors.Join(result, item.Rollback())
	}
	return result
}

func (claudeDesktopAdapter) Apply(_ context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	plans, _, err := claudeDesktopPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	receipts := claudeDesktopReceipt{}
	for _, plan := range plans {
		receipt, err := plan.Apply()
		if err != nil {
			return nil, errors.Join(err, receipts.Rollback())
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (claudeDesktopAdapter) ProjectionChanged(before, after configuration.Config) bool {
	previous, current := before.Clients[configuration.ClientClaudeDesktop], after.Clients[configuration.ClientClaudeDesktop]
	if previous.Enabled != current.Enabled || previous.Executable != current.Executable || previous.CredentialCommand != current.CredentialCommand || !slices.Equal(previous.Targets, current.Targets) {
		return true
	}
	if !current.Enabled {
		return false
	}
	left, leftErr := before.ResolveRuntime(configuration.ClientClaudeDesktop, "")
	right, rightErr := after.ResolveRuntime(configuration.ClientClaudeDesktop, "")
	if leftErr != nil || rightErr != nil || left != right {
		return true
	}
	return !slices.Equal(claudeDesktopModels(before, left), claudeDesktopModels(after, right))
}

func (claudeDesktopAdapter) Inspect(ctx context.Context, deps Dependencies, cfg configuration.Config, selected configuration.Runtime) Status {
	if err := ctx.Err(); err != nil {
		return Status{Issue: err.Error()}
	}
	binding := cfg.Clients[configuration.ClientClaudeDesktop]
	if !binding.Enabled {
		return Status{Issue: "Claude Desktop adapter is disabled", RepairAction: "aigw sync"}
	}
	available, err := discovery.ExecutableAvailable(binding.Executable)
	if err != nil || !available {
		return Status{Issue: "Claude Desktop executable is unavailable", RepairAction: "aigw repair"}
	}
	if len(binding.Targets) != 1 {
		return Status{Issue: "Claude Desktop configuration library is missing", RepairAction: "aigw repair"}
	}
	desired, err := claudeDesktopDesired(deps, cfg, selected)
	if err == nil {
		var plan claudedesktop.Plan
		plan, err = claudedesktop.Prepare(claudedesktop.PathsForLibrary(binding.Targets[0]), &desired)
		if err == nil && plan.Action != claudedesktop.ActionUnchanged {
			err = errors.New("Claude Desktop projection differs from the selected Route")
		}
	}
	if err != nil {
		return Status{Issue: err.Error(), RepairAction: "aigw sync"}
	}
	return Status{Ready: true}
}

func (claudeDesktopAdapter) Verify(context.Context, Dependencies, configuration.Config, configuration.Runtime, string) (Verification, error) {
	return Verification{}, errors.New("Claude Desktop verification requires explicit native Chat, Cowork, or Code acceptance")
}

func (claudeDesktopAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, configuration.ClientClaudeDesktop)
}

func claudeDesktopPlans(deps Dependencies, before, after configuration.Config) ([]claudedesktop.Plan, []string, error) {
	previous, current := before.Clients[configuration.ClientClaudeDesktop], after.Clients[configuration.ClientClaudeDesktop]
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired *claudedesktop.Desired
	if current.Enabled && len(current.Targets) > 0 {
		if len(current.Targets) != 1 {
			return nil, nil, errors.New("Claude Desktop requires one configuration library")
		}
		selected, err := after.ResolveRuntime(configuration.ClientClaudeDesktop, "")
		if err != nil {
			return nil, nil, err
		}
		projection, err := claudeDesktopDesired(deps, after, selected)
		if err != nil {
			return nil, nil, err
		}
		desired = &projection
	}
	targets := slices.Clone(current.Targets)
	for _, target := range previous.Targets {
		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}
	plans := make([]claudedesktop.Plan, 0, len(targets))
	for _, target := range targets {
		projection := desired
		if !current.Enabled || !slices.Contains(current.Targets, target) {
			projection = nil
		}
		plan, err := claudedesktop.Prepare(claudedesktop.PathsForLibrary(target), projection)
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	return plans, targets, nil
}

func claudeDesktopDesired(deps Dependencies, cfg configuration.Config, selected configuration.Runtime) (claudedesktop.Desired, error) {
	helper := selected.CredentialExecutable(deps.AIGWExecutable)
	arguments := []string{}
	if selected.UsesAIGWCredentialStore() {
		arguments = []string{"credential", configuration.ClientClaudeDesktop, selected.CredentialProjectionFingerprint(configuration.ClientClaudeDesktop)}
	}
	models := claudeDesktopModels(cfg, selected)
	return claudedesktop.Desired{
		BaseURL:              selected.Endpoint,
		CredentialExecutable: helper,
		CredentialArguments:  arguments,
		Models:               models,
	}, nil
}

func claudeDesktopModels(cfg configuration.Config, selected configuration.Runtime) []claudedesktop.Model {
	models := []claudedesktop.Model{{Name: selected.Model, Label: selected.RouteLabel}}
	seen := map[string]bool{selected.Model: true}
	spec := mustClientSpec(configuration.ClientClaudeDesktop)
	for _, routeID := range cfg.RouteIDs() {
		if routeID == selected.RouteID {
			continue
		}
		route := cfg.Routes[routeID]
		if route.Account != selected.AccountID || seen[route.Model] {
			continue
		}
		account := cfg.Accounts[route.Account]
		if !slices.Contains(spec.CompatibleRouteProtocols(account, route), selected.Protocol) {
			continue
		}
		models = append(models, claudedesktop.Model{Name: route.Model, Label: route.Label})
		seen[route.Model] = true
	}
	return models
}

func claudeDesktopTarget(current []string, discovered discovery.Result) string {
	if len(current) == 1 {
		return current[0]
	}
	surface, ok := discovered.Surface(string(surfaceidentity.ClaudeDesktopLibrary))
	if !ok || !surface.Present {
		return ""
	}
	return surface.ConfigPath
}
