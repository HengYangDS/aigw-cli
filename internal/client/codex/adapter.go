package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"aigw-cli/internal/client"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	surfaceidentity "aigw-cli/internal/surface"
)

// ID identifies the admitted Codex client family.
const ID = "codex"

// codexAdapter is the Codex client adapter behind the client seam.
type codexAdapter struct{}

// New returns the Codex adapter for the composition root to register.
func New() client.Adapter { return codexAdapter{} }

func (codexAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{
		ID: ID, Label: "Codex", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses},
		TargetFile: "config.toml", TargetsUsage: "repeat for multiple Codex homes", RetryBudgets: true,
		DefaultModelProvider: domain.ModelProviderAIGW, ClientNativeAuthentication: true,
		PublishedProfileProtocol: domain.ProtocolOpenAIResponses,
		Doctor:                   domain.DoctorWording{AdapterStates: true, RouteBinding: true},
	}
}

func (codexAdapter) Discover(source client.DiscoverySource) discovery.Result {
	path := filepath.Join(homeDirectory(source.Platform()), "config.toml")
	return discovery.Result{
		Executables: map[string]string{ID: source.Executable(ID)},
		Surfaces: []discovery.Surface{{
			ID:          string(HomeDefault),
			Product:     "Codex",
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  path,
			Present:     source.FilePresent(path),
			AutoManaged: true,
		}},
	}
}

// homeDirectory returns the explicit Codex Home or its platform default.
func homeDirectory(host discovery.Host) string {
	if home := host.Getenv("CODEX_HOME"); home != "" {
		return home
	}
	return filepath.Join(host.Home, ".codex")
}

// ValidateTarget rejects executable paths and non-Codex-Home surfaces where
// a writable configuration target is required.
func (codexAdapter) ValidateTarget(discovered discovery.Result, path string) error {
	if _, ok := discovered.SurfaceForExecutablePath(path); ok {
		return fmt.Errorf("an executable is not a Codex configuration target")
	}
	if surface, ok := discovered.SurfaceForConfigPath(path); ok && surface.ID != string(HomeDefault) {
		return fmt.Errorf("surface %s is not an AIGW Codex target", surface.ID)
	}
	return nil
}

func (codexAdapter) Converge(deps client.Dependencies, cfg *domain.Config, discovered discovery.Result) error {
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		if _, unselected := errors.AsType[*domain.RuntimeBindingUnselectedError](err); unselected {
			return nil
		}
		return err
	}
	binding, explicitlyConfigured := cfg.Clients[ID]
	if !explicitlyConfigured || !binding.Enabled {
		return nil
	}
	targets := convergedTargets(discovered, binding.Targets)
	executable, err := client.ResolveExecutable(ID, binding.Executable, discovered.Executable(ID))
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
	if executable != "" && len(targets) > 0 && available {
		cfg.SetClientActivation(ID, true, executable, targets)
	}
	return nil
}

func (codexAdapter) Plan(deps client.Dependencies, before, after domain.Config) ([]client.ProjectionPlan, error) {
	beforeRefs, afterRefs, previous, runtime, err := reconciliationInputs(deps, before, after)
	if err != nil {
		return nil, err
	}
	var plans []ProjectionPlan
	if deps.AuthorizedSelectionClient == ID {
		plans, err = PlanReconciliationAuthorizedTransition(beforeRefs, afterRefs, previous, runtime)
	} else {
		plans, err = PlanReconciliationTransition(beforeRefs, afterRefs, previous, runtime)
	}
	if err != nil {
		return nil, err
	}
	result := make([]client.ProjectionPlan, 0, len(plans))
	for _, plan := range plans {
		result = append(result, client.ProjectionPlan{Client: ID, Target: plan.Target, Action: string(plan.Action), Surface: plan.SurfaceID, Effects: plan.Effects, ChangesState: plan.ChangesState()})
	}
	return result, nil
}

func (codexAdapter) Apply(_ context.Context, deps client.Dependencies, before, after domain.Config) (client.ProjectionReceipt, error) {
	beforeRefs, afterRefs, previous, runtime, err := reconciliationInputs(deps, before, after)
	if err != nil {
		return nil, err
	}
	if deps.AuthorizedSelectionClient == ID {
		return ReconcileConfigsAuthorizedTransition(beforeRefs, afterRefs, previous, runtime)
	}
	return ReconcileConfigsTransition(beforeRefs, afterRefs, previous, runtime)
}

func (codexAdapter) ProjectionChanged(before, after domain.Config) bool {
	beforeBinding := before.Clients[ID]
	afterBinding := after.Clients[ID]
	if beforeBinding.Enabled != afterBinding.Enabled || beforeBinding.CredentialCommand != afterBinding.CredentialCommand {
		return true
	}
	if !afterBinding.Enabled {
		return false
	}
	if !beforeBinding.Enabled || !slices.Equal(beforeBinding.Targets, afterBinding.Targets) {
		return true
	}
	beforeRuntime, beforeErr := before.ResolveRuntime(ID, "")
	afterRuntime, afterErr := after.ResolveRuntime(ID, "")
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return beforeRuntime.AccountID != afterRuntime.AccountID ||
		!beforeRuntime.SameUpstream(afterRuntime) ||
		beforeRuntime.RouteID != afterRuntime.RouteID ||
		beforeRuntime.RouteLabel != afterRuntime.RouteLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model ||
		beforeRuntime.ModelProvider != afterRuntime.ModelProvider ||
		beforeRuntime.Authentication != afterRuntime.Authentication
}

func (codexAdapter) Inspect(_ context.Context, deps client.Dependencies, cfg domain.Config, runtime domain.Runtime) client.Status {
	binding := cfg.Clients[ID]
	if !binding.Enabled {
		return client.Status{Issue: "Codex adapter is disabled", RepairAction: "aigw sync"}
	}
	if binding.Executable == "" {
		return client.Status{Issue: "Codex executable is not configured", RepairAction: "aigw repair"}
	}
	if len(binding.Targets) == 0 {
		return client.Status{Issue: "Codex configuration target is missing", RepairAction: "aigw repair"}
	}
	status := client.Status{Ready: true, Checks: make([]client.Check, 0, len(binding.Targets))}
	for index, target := range binding.Targets {
		check := client.Check{ID: fmt.Sprintf("codex:target-%d", index+1), Ready: true, Detail: "route " + runtime.RouteID, Target: target}
		observed, readerErr := retainedRuntime(target, cfg, runtime, deps.AIGWExecutable)
		issue, detail, repair := "Codex configuration projection drift: ", "", "aigw sync"
		if readerErr != nil {
			detail = "Codex credential reader cannot be inspected"
		} else if err := ValidateConfig(target, observed); err != nil {
			detail = "Codex configuration target does not match selected binding"
			if conflict, ok := errors.AsType[*ConflictError](err); ok {
				// Synchronization refuses an outside edit, so advising it alone
				// would send the operator back to the same refusal.
				issue, detail, repair = "Codex configuration conflict: ", "AIGW-managed Codex settings were changed outside AIGW", conflict.NextAction()
			} else if errors.Is(err, os.ErrNotExist) {
				detail = "Codex configuration target is unavailable"
			}
		}
		if detail != "" {
			check.Ready = false
			check.Detail = detail
			check.RepairAction = repair
			status.Ready = false
			status.Issue = issue + detail
			status.RepairAction = repair
		}
		status.Checks = append(status.Checks, check)
	}

	return status
}

func (codexAdapter) Withdraw(cfg *domain.Config) {
	delete(cfg.Clients, ID)
}

func convergedTargets(discovered discovery.Result, current []string) []string {
	seen := map[string]bool{}
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	appendTarget := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	for _, path := range AutoManagedTargets(discovered) {
		appendTarget(path)
	}
	for _, path := range current {
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			if surface.ID == string(HomeDefault) {
				appendTarget(path)
			}
			continue
		}
		appendTarget(path)
	}
	return targets
}

func reconciliationInputs(deps client.Dependencies, before, after domain.Config) ([]TargetRef, []TargetRef, domain.Runtime, domain.Runtime, error) {
	beforeBinding := before.Clients[ID]
	afterBinding := after.Clients[ID]
	if !beforeBinding.Enabled && !afterBinding.Enabled {
		return nil, nil, domain.Runtime{}, domain.Runtime{}, nil
	}
	discovered, err := discover(deps)
	if err != nil {
		return nil, nil, domain.Runtime{}, domain.Runtime{}, err
	}
	beforeRefs, err := targetRefs(discovered, beforeBinding.Targets, bindingExecutable(discovered, beforeBinding))
	if err != nil {
		return nil, nil, domain.Runtime{}, domain.Runtime{}, err
	}
	afterRefs, err := targetRefs(discovered, afterBinding.Targets, bindingExecutable(discovered, afterBinding))
	if err != nil {
		return nil, nil, domain.Runtime{}, domain.Runtime{}, err
	}
	previous, _ := before.ResolveRuntime(ID, "")
	if previous.RequiresAccountToken() {
		previous.CredentialCommand = previous.CredentialExecutable(deps.AIGWExecutable)
	}
	if !afterBinding.Enabled {
		return beforeRefs, nil, previous, domain.Runtime{}, nil
	}
	runtime, err := after.ResolveRuntime(ID, "")
	if err != nil {
		return nil, nil, domain.Runtime{}, domain.Runtime{}, err
	}
	if runtime.RequiresAccountToken() {
		runtime.CredentialCommand = runtime.CredentialExecutable(deps.AIGWExecutable)
	}
	return beforeRefs, afterRefs, previous, runtime, nil
}

func discover(deps client.Dependencies) (discovery.Result, error) {
	if deps.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("codex surface discovery is unavailable")
	}
	return deps.Discovery.Discover(), nil
}

func bindingExecutable(discovered discovery.Result, binding domain.ClientBinding) string {
	if binding.Executable != "" {
		return binding.Executable
	}
	return discovered.Executable(ID)
}

func targetRefs(discovered discovery.Result, paths []string, executable string) ([]TargetRef, error) {
	refs := make([]TargetRef, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			return nil, fmt.Errorf("codex config target is empty")
		}
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			refs = append(refs, TargetRef{
				SurfaceID:      surface.ID,
				Authority:      surface.Authority,
				ProjectionMode: ProjectionFullSelection,
				Path:           path,
				Executable:     executable,
				CreateIfAbsent: surface.AutoManaged,
			})
			continue
		}
		sum := sha256.Sum256([]byte(filepath.Clean(path)))
		refs = append(refs, TargetRef{
			SurfaceID:      string(HomeExplicit(hex.EncodeToString(sum[:6]))),
			Authority:      string(surfaceidentity.AuthorityAIGW),
			ProjectionMode: ProjectionFullSelection,
			Path:           path,
			Executable:     executable,
		})
	}
	return refs, nil
}
