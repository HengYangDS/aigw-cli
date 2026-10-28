package claudedesktop

import (
	"context"
	"errors"
	"slices"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"aigw-cli/internal/transaction"
)

// LibrarySurface identifies Claude Desktop's per-user third-party
// configuration library.
const LibrarySurface surfaceidentity.ID = "claude-desktop-config-library"

// claudeDesktopAdapter is the Claude Desktop client adapter behind the client seam.
type claudeDesktopAdapter struct{}

// New returns the Claude Desktop adapter for the composition root to register.
func New() client.Adapter { return claudeDesktopAdapter{} }

func (claudeDesktopAdapter) Spec() configuration.ClientSpec {
	return client.AdmittedSpec(configuration.ClientClaudeDesktop)
}

func (claudeDesktopAdapter) Discover(source client.DiscoverySource) discovery.Result {
	host := source.Platform()
	executable := ""
	for _, candidate := range ApplicationPathsFor(host.GOOS, map[string]string{"HOME": host.Home, "LOCALAPPDATA": host.Getenv("LOCALAPPDATA")}) {
		if source.Runnable(candidate) {
			executable = candidate
			break
		}
	}
	library, _ := LibraryPathFor(host.GOOS, map[string]string{
		"HOME": host.Home, "XDG_CONFIG_HOME": host.Getenv("XDG_CONFIG_HOME"), "LOCALAPPDATA": host.Getenv("LOCALAPPDATA"),
	})
	return discovery.Result{
		Executables: map[string]string{configuration.ClientClaudeDesktop: executable},
		Surfaces: []discovery.Surface{{
			ID:          string(LibrarySurface),
			Product:     "Claude Desktop",
			Authority:   string(surfaceidentity.AuthorityAIGW),
			Executable:  executable,
			ConfigPath:  library,
			Present:     executable != "",
			AutoManaged: true,
		}},
	}
}

func (claudeDesktopAdapter) Converge(deps client.Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
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
	executable, err := client.ResolveExecutable(configuration.ClientClaudeDesktop, binding.Executable, discovered.Executable(configuration.ClientClaudeDesktop))
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
	target := claudeDesktopTarget(binding.Targets, discovered)
	if executable != "" && target != "" && available {
		cfg.SetClientActivation(configuration.ClientClaudeDesktop, true, executable, []string{target})
	}
	return nil
}

func (claudeDesktopAdapter) Plan(deps client.Dependencies, before, after configuration.Config) ([]client.ProjectionPlan, error) {
	plans, targets, err := claudeDesktopPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	result := make([]client.ProjectionPlan, 0, len(plans))
	for index, plan := range plans {
		result = append(result, client.ProjectionPlan{Client: configuration.ClientClaudeDesktop, Target: targets[index], Action: string(plan.Action), Surface: string(LibrarySurface), ChangesState: plan.ChangesState()})
	}
	return result, nil
}

type claudeDesktopReceipt []Receipt

func (receipt claudeDesktopReceipt) Rollback() error {
	var result error
	for _, item := range slices.Backward(receipt) {
		result = errors.Join(result, item.Rollback())
	}
	return result
}

func (claudeDesktopAdapter) Apply(_ context.Context, deps client.Dependencies, before, after configuration.Config) (client.ProjectionReceipt, error) {
	plans, _, err := claudeDesktopPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	receipts := claudeDesktopReceipt{}
	for _, plan := range plans {
		receipt, err := plan.Apply()
		if err != nil {
			return nil, transaction.CompensationFailed(err, receipts.Rollback())
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
	return !slices.Equal(claudeDesktopModels(before, left), claudeDesktopModels(after, right)) ||
		claudeDesktopToolSearch(before, left) != claudeDesktopToolSearch(after, right)
}

func (claudeDesktopAdapter) Inspect(ctx context.Context, deps client.Dependencies, cfg configuration.Config, selected configuration.Runtime) client.Status {
	if err := ctx.Err(); err != nil {
		return client.Status{Issue: err.Error()}
	}
	binding := cfg.Clients[configuration.ClientClaudeDesktop]
	if !binding.Enabled {
		return client.Status{Issue: "Claude Desktop adapter is disabled", RepairAction: "aigw sync"}
	}
	available, err := discovery.ExecutableAvailable(binding.Executable)
	if err != nil || !available {
		return client.Status{Issue: "Claude Desktop executable is unavailable", RepairAction: "aigw repair"}
	}
	if len(binding.Targets) != 1 {
		return client.Status{Issue: "Claude Desktop configuration library is missing", RepairAction: "aigw repair"}
	}
	plan, err := claudeDesktopInspectionPlan(deps, cfg, selected, PathsForLibrary(binding.Targets[0]))
	if err != nil {
		return client.Status{Issue: "Claude Desktop projection cannot be inspected", RepairAction: "aigw sync"}
	}
	if plan.Action != ActionUnchanged {
		return client.Status{Issue: "Claude Desktop projection differs from the selected Route", RepairAction: "aigw sync"}
	}
	return client.Status{Ready: true}
}

func claudeDesktopInspectionPlan(deps client.Dependencies, cfg configuration.Config, selected configuration.Runtime, paths Paths) (Plan, error) {
	desired, err := claudeDesktopDesired(deps, cfg, selected)
	if err != nil {
		return Plan{}, err
	}
	plan, err := Prepare(paths, &desired)
	if err != nil || plan.Action == ActionUnchanged || !selected.UsesAIGWCredentialStore() {
		return plan, err
	}
	if plan.ObservedCredentialExecutable() == "" {
		return plan, nil
	}
	reader, err := client.RetainedDefaultReader(deps.AIGWExecutable, false, func() (string, error) {
		return plan.ObservedCredentialExecutable(), nil
	})
	if err != nil || reader == deps.AIGWExecutable {
		return plan, err
	}
	desired.CredentialExecutable = reader
	return Prepare(paths, &desired)
}

func (claudeDesktopAdapter) Verify(context.Context, client.Dependencies, configuration.Config, configuration.Runtime, string) (client.Verification, error) {
	return client.Verification{}, errors.New("Claude Desktop verification requires explicit native Chat, Cowork, or Code acceptance")
}

// AdoptSelection adopts nothing: Claude Desktop has no native selection that AIGW
// records as its Client Binding.
func (claudeDesktopAdapter) AdoptSelection(client.Dependencies, *configuration.Config) (string, error) {
	return "", nil
}

func (claudeDesktopAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, configuration.ClientClaudeDesktop)
}

func claudeDesktopPlans(deps client.Dependencies, before, after configuration.Config) ([]Plan, []string, error) {
	previous, current := before.Clients[configuration.ClientClaudeDesktop], after.Clients[configuration.ClientClaudeDesktop]
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired *Desired
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
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
		projection := desired
		if !current.Enabled || !slices.Contains(current.Targets, target) {
			projection = nil
		}
		plan, err := Prepare(PathsForLibrary(target), projection)
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	return plans, targets, nil
}

func claudeDesktopDesired(deps client.Dependencies, cfg configuration.Config, selected configuration.Runtime) (Desired, error) {
	helper := selected.CredentialExecutable(deps.AIGWExecutable)
	arguments := []string{}
	if selected.UsesAIGWCredentialStore() {
		arguments = []string{"credential", configuration.ClientClaudeDesktop, selected.CredentialProjectionFingerprint(configuration.ClientClaudeDesktop)}
	}
	models := claudeDesktopModels(cfg, selected)
	return Desired{
		BaseURL:              selected.Endpoint,
		CredentialExecutable: helper,
		CredentialArguments:  arguments,
		Models:               models,
		ToolSearch:           claudeDesktopToolSearch(cfg, selected),
	}, nil
}

// claudeDesktopRouteIDs lists the Routes the profile exposes: the selected
// Route, then each compatible Route of its Account with a distinct Model.
func claudeDesktopRouteIDs(cfg configuration.Config, selected configuration.Runtime) []string {
	routeIDs := []string{selected.RouteID}
	seen := map[string]bool{cfg.Routes[selected.RouteID].Model: true}
	spec := client.AdmittedSpec(configuration.ClientClaudeDesktop)
	for _, routeID := range cfg.CatalogRouteIDs() {
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
		routeIDs = append(routeIDs, routeID)
		seen[route.Model] = true
	}
	return routeIDs
}

func claudeDesktopModels(cfg configuration.Config, selected configuration.Runtime) []Model {
	routeIDs := claudeDesktopRouteIDs(cfg, selected)
	models := []Model{{Name: selected.Model, Label: selected.RouteLabel}}
	for _, routeID := range routeIDs[1:] {
		models = append(models, Model{Name: cfg.Routes[routeID].UpstreamModelID(), Label: cfg.RouteLabel(routeID)})
	}
	return models
}

// claudeDesktopToolSearch enables Desktop's profile-wide host tool search only
// when every exposed Route qualified it on the selected protocol.
func claudeDesktopToolSearch(cfg configuration.Config, selected configuration.Runtime) bool {
	for _, routeID := range claudeDesktopRouteIDs(cfg, selected) {
		if !cfg.Routes[routeID].Declares(selected.Protocol, configuration.CapabilityToolSearch) {
			return false
		}
	}
	return true
}

func claudeDesktopTarget(current []string, discovered discovery.Result) string {
	if len(current) == 1 {
		return current[0]
	}
	surface, ok := discovered.Surface(string(LibrarySurface))
	if !ok || !surface.Present {
		return ""
	}
	return surface.ConfigPath
}
