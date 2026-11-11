package claudedesktop

import (
	"context"
	"errors"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	surfaceidentity "aigw-cli/internal/adapter/surface"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// ID identifies Claude Desktop's independent third-party inference surface.
const ID = "claude-desktop"

// LibrarySurface identifies Claude Desktop's per-user third-party
// configuration library.
const LibrarySurface surfaceidentity.ID = "claude-desktop-config-library"

// claudeDesktopAdapter is the Claude Desktop client adapter behind the client seam.
type claudeDesktopAdapter struct{}

// New returns the Claude Desktop adapter for the composition root to register.
func New() adapter.Adapter { return claudeDesktopAdapter{} }

func (claudeDesktopAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{
		ID:                     ID,
		Label:                  "Claude Desktop",
		EndpointProtocols:      []domain.EndpointProtocol{domain.ProtocolAnthropic},
		RestartAfterProjection: true,
		QualifiedModes:         []string{"Chat", "Cowork", "Code"},
		QualifiedPlatforms:     []string{"macOS"},
		OptionalTarget:         "configLibrary",
		InAppAcceptance:        true,
	}
}

func (claudeDesktopAdapter) Discover(source adapter.DiscoverySource) discovery.Result {
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
		Executables: map[string]string{ID: executable},
		Surfaces: []discovery.Surface{{
			ID:          string(LibrarySurface),
			Executable:  executable,
			ConfigPath:  library,
			Present:     executable != "",
			AutoManaged: true,
		}},
	}
}

func (claudeDesktopAdapter) Converge(deps adapter.Dependencies, cfg *domain.Config, discovered discovery.Result) error {
	binding, executable, ready, err := adapter.ConvergeBinding(deps, cfg, discovered, ID, false)
	if err != nil || !ready {
		return err
	}
	if target := claudeDesktopTarget(binding.Targets, discovered); target != "" {
		cfg.SetClientActivation(ID, true, executable, []string{target})
	}
	return nil
}

func (claudeDesktopAdapter) Plan(deps adapter.Dependencies, before, after domain.Config) ([]adapter.ProjectionPlan, error) {
	plans, targets, err := claudeDesktopPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	result := make([]adapter.ProjectionPlan, 0, len(plans))
	for index, plan := range plans {
		result = append(result, adapter.ProjectionPlan{Client: ID, Target: targets[index], Action: string(plan.Action), Surface: string(LibrarySurface), Effects: plan.Effects(), ChangesState: plan.ChangesState()})
	}
	return result, nil
}

func (claudeDesktopAdapter) Apply(_ context.Context, deps adapter.Dependencies, before, after domain.Config) (adapter.ProjectionReceipt, error) {
	plans, _, err := claudeDesktopPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	receipts := adapter.ProjectionReceipts{}
	for _, plan := range plans {
		receipt, err := plan.Apply()
		if err != nil {
			return nil, platform.FailedWrite(err, receipts.Restore())
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (claudeDesktopAdapter) ProjectionChanged(before, after domain.Config) bool {
	previous, current := before.Clients[ID], after.Clients[ID]
	if previous.Enabled != current.Enabled || previous.Executable != current.Executable || previous.CredentialCommand != current.CredentialCommand || !slices.Equal(previous.Targets, current.Targets) {
		return true
	}
	if !current.Enabled {
		return false
	}
	left, leftErr := before.ResolveRuntime(ID, domain.ModelRef{})
	right, rightErr := after.ResolveRuntime(ID, domain.ModelRef{})
	if leftErr != nil || rightErr != nil || left != right {
		return true
	}
	return !slices.Equal(claudeDesktopModels(before, left), claudeDesktopModels(after, right)) ||
		claudeDesktopToolSearch(before, left) != claudeDesktopToolSearch(after, right)
}

func (claudeDesktopAdapter) Inspect(ctx context.Context, deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime) adapter.Status {
	if err := ctx.Err(); err != nil {
		return adapter.Status{Issue: err.Error()}
	}
	binding := cfg.Clients[ID]
	if !binding.Enabled {
		return adapter.Status{Issue: "Claude Desktop adapter is disabled", RepairAction: "aigw sync"}
	}
	available, err := discovery.ExecutableAvailable(binding.Executable)
	if err != nil || !available {
		return adapter.Status{Issue: "Claude Desktop executable is unavailable", RepairAction: "aigw sync"}
	}
	if len(binding.Targets) != 1 {
		return adapter.Status{Issue: "Claude Desktop configuration library is missing", RepairAction: "aigw sync"}
	}
	plan, err := claudeDesktopInspectionPlan(deps, cfg, selected, PathsForLibrary(binding.Targets[0]))
	if status, refused := adapter.ConflictStatus(err); refused {
		return status
	}
	if failed, atTarget := errors.AsType[*adapter.TargetError](err); atTarget {
		if status, unreadable := adapter.UnreadableStatus(err, failed.Path, deps.Host.Home); unreadable {
			return status
		}
	}
	if err != nil {
		return adapter.Status{Issue: "Claude Desktop files cannot be inspected", RepairAction: "aigw sync"}
	}
	if plan.Action != ActionUnchanged {
		return adapter.Status{Issue: "Claude Desktop files differ from the selected model", RepairAction: "aigw sync"}
	}
	return adapter.Status{Ready: true}
}

func claudeDesktopInspectionPlan(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime, paths Paths) (Plan, error) {
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
	reader, err := adapter.RetainedDefaultReader(deps.AIGWExecutable, false, func() (string, error) {
		return plan.ObservedCredentialExecutable(), nil
	})
	if err != nil || reader == deps.AIGWExecutable {
		return plan, err
	}
	desired.CredentialExecutable = reader
	return Prepare(paths, &desired)
}

func (claudeDesktopAdapter) Verify(context.Context, adapter.Dependencies, domain.Config, domain.Runtime, string) (adapter.Verification, error) {
	return adapter.Verification{}, errors.New("Claude Desktop verification requires explicit native Chat, Cowork, or Code acceptance")
}

// AdoptSelection adopts nothing: Claude Desktop has no native selection that AIGW
// records as its Client Binding.
func (claudeDesktopAdapter) AdoptSelection(adapter.Dependencies, *domain.Config) (domain.ModelRef, error) {
	return domain.ModelRef{}, nil
}

func (claudeDesktopAdapter) Withdraw(cfg *domain.Config) {
	delete(cfg.Clients, ID)
}

func claudeDesktopPlans(deps adapter.Dependencies, before, after domain.Config) ([]Plan, []string, error) {
	previous, current := before.Clients[ID], after.Clients[ID]
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired *Desired
	if current.Enabled && len(current.Targets) > 0 {
		if len(current.Targets) != 1 {
			return nil, nil, errors.New("Claude Desktop requires one configuration library")
		}
		selected, err := after.ResolveRuntime(ID, domain.ModelRef{})
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

func claudeDesktopDesired(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime) (Desired, error) {
	helper := selected.CredentialExecutable(deps.AIGWExecutable)
	arguments := []string{}
	if selected.UsesAIGWCredentialStore() {
		arguments = []string{"credential", ID, selected.CredentialProjectionFingerprint(ID)}
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

// claudeDesktopModelRefs lists the models the profile exposes: the selected
// model, then, in configuration order, the first compatible model of its
// provider of each other family. A model that is its own family counts as one
// family.
func claudeDesktopModelRefs(cfg domain.Config, selected domain.Runtime) []domain.ModelRef {
	refs := []domain.ModelRef{selected.Model}
	seen := map[string]bool{cfg.Models[selected.Model].FamilyID(): true}
	spec := claudeDesktopAdapter{}.Spec()
	for _, ref := range cfg.ProviderModelRefs(selected.ProviderID) {
		model := cfg.Models[ref]
		if ref == selected.Model || seen[model.FamilyID()] {
			continue
		}
		provider := cfg.Providers[model.Provider]
		if !slices.Contains(spec.CompatibleModelProtocols(provider, model), selected.Protocol) {
			continue
		}
		refs = append(refs, ref)
		seen[model.FamilyID()] = true
	}
	return refs
}

func claudeDesktopModels(cfg domain.Config, selected domain.Runtime) []Model {
	refs := claudeDesktopModelRefs(cfg, selected)
	models := []Model{{Name: selected.UpstreamModel, Label: selected.ModelLabel}}
	for _, ref := range refs[1:] {
		models = append(models, Model{Name: ref.ID(), Label: cfg.ModelLabel(ref)})
	}
	return models
}

// ToolSearch reports whether cfg enables Claude Desktop's host tool search:
// Desktop is enabled with a selection, and every model that its profile lists
// declares tool_search on the selected protocol. It names the listed models
// that do not; a disabled Desktop, or one without a selection, lists none.
func (claudeDesktopAdapter) ToolSearch(cfg domain.Config) (bool, []domain.ModelRef) {
	if !cfg.Clients[ID].Enabled {
		return false, nil
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return false, nil
	}
	lacking := claudeDesktopLackingToolSearch(cfg, selected)
	return len(lacking) == 0, lacking
}

// claudeDesktopToolSearch enables Desktop's profile-wide host tool search only
// when every exposed model qualified it on the selected protocol.
func claudeDesktopToolSearch(cfg domain.Config, selected domain.Runtime) bool {
	return len(claudeDesktopLackingToolSearch(cfg, selected)) == 0
}

// claudeDesktopLackingToolSearch returns, in the profile's order, the exposed
// models that did not qualify tool search on the selected protocol.
func claudeDesktopLackingToolSearch(cfg domain.Config, selected domain.Runtime) []domain.ModelRef {
	var lacking []domain.ModelRef
	for _, ref := range claudeDesktopModelRefs(cfg, selected) {
		if !cfg.Models[ref].Declares(selected.Protocol, domain.CapabilityToolSearch) {
			lacking = append(lacking, ref)
		}
	}
	return lacking
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
