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

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// ID identifies the admitted Codex client family.
const ID = "codex"

// modelProviderAIGW is the stable provider identifier written into
// AIGW-owned Codex projections.
const modelProviderAIGW = "aigw"

// codexAdapter is the Codex client adapter behind the client seam.
type codexAdapter struct{}

// New returns the Codex adapter for the composition root to register.
func New() adapter.Adapter { return codexAdapter{} }

func (codexAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{
		ID: ID, Label: "Codex", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolResponses},
		TargetFile: "config.toml", TargetsUsage: "repeat for multiple Codex homes", RetryBudgets: true, OperatorCreatesTargets: true,
		DefaultModelProvider: modelProviderAIGW, ClientNativeAuthentication: true,
	}
}

func (codexAdapter) Discover(source adapter.DiscoverySource) discovery.Result {
	path := filepath.Join(homeDirectory(source.Platform()), "config.toml")
	return discovery.Result{
		Executables: map[string]string{ID: source.Executable(ID)},
		Surfaces: []discovery.Surface{{
			ID:          string(HomeDefault),
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

func (codexAdapter) Converge(deps adapter.Dependencies, cfg *domain.Config, discovered discovery.Result) error {
	binding, executable, ready, err := adapter.ConvergeBinding(deps, cfg, discovered, ID, false)
	if err != nil || !ready {
		return err
	}
	if targets := convergedTargets(discovered, binding.Targets); len(targets) > 0 {
		cfg.SetClientActivation(ID, true, executable, targets)
	}
	return nil
}

func (codexAdapter) Plan(deps adapter.Dependencies, before, after domain.Config) ([]adapter.ProjectionPlan, error) {
	beforeRefs, afterRefs, previous, runtime, err := reconciliationInputs(deps, before, after)
	if err != nil {
		return nil, err
	}
	plans, err := planCodexReconciliationTransition(beforeRefs, afterRefs, previous, runtime, deps.AuthorizedSelectionClient == ID)
	if err != nil {
		return nil, err
	}
	result := make([]adapter.ProjectionPlan, 0, len(plans))
	for _, plan := range plans {
		result = append(result, adapter.ProjectionPlan{Client: ID, Target: plan.Target, Action: string(plan.Action), Surface: plan.SurfaceID, Effects: plan.Effects, ChangesState: plan.ChangesState()})
	}
	return result, nil
}

func (codexAdapter) Apply(_ context.Context, deps adapter.Dependencies, before, after domain.Config) (adapter.ProjectionReceipt, error) {
	beforeRefs, afterRefs, previous, runtime, err := reconciliationInputs(deps, before, after)
	if err != nil {
		return nil, err
	}
	return ReconcileConfigs(beforeRefs, afterRefs, previous, runtime, deps.AuthorizedSelectionClient == ID)
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
	beforeRuntime, beforeErr := before.ResolveRuntime(ID, domain.ModelRef{})
	afterRuntime, afterErr := after.ResolveRuntime(ID, domain.ModelRef{})
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return beforeRuntime.ProviderID != afterRuntime.ProviderID ||
		!beforeRuntime.SameUpstream(afterRuntime) ||
		beforeRuntime.Model != afterRuntime.Model ||
		beforeRuntime.ModelLabel != afterRuntime.ModelLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.UpstreamModel != afterRuntime.UpstreamModel ||
		beforeRuntime.ModelProvider != afterRuntime.ModelProvider ||
		beforeRuntime.Authentication != afterRuntime.Authentication
}

func (codexAdapter) Inspect(_ context.Context, deps adapter.Dependencies, cfg domain.Config, runtime domain.Runtime) adapter.Status {
	binding := cfg.Clients[ID]
	if !binding.Enabled {
		return adapter.Status{Issue: "Codex adapter is disabled", RepairAction: "aigw sync"}
	}
	if binding.Executable == "" {
		return adapter.Status{Issue: "Codex executable is not configured", RepairAction: "aigw sync"}
	}
	if len(binding.Targets) == 0 {
		return adapter.Status{Issue: "Codex configuration target is missing", RepairAction: "aigw sync"}
	}
	status := adapter.Status{Ready: true, Checks: make([]adapter.Check, 0, len(binding.Targets))}
	for index, target := range binding.Targets {
		check := adapter.Check{ID: fmt.Sprintf("codex:target-%d", index+1), Ready: true, Detail: "model " + runtime.Model.String(), Target: target}
		issue, detail, repair := inspectTarget(deps, cfg, runtime, target)
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

// inspectTarget inspects one configured Codex file: what does not match the
// selection, with its issue and repair, or no detail when it matches.
func inspectTarget(deps adapter.Dependencies, cfg domain.Config, runtime domain.Runtime, target string) (issue, detail, repair string) {
	issue, repair = "Codex configuration projection drift: ", "aigw sync"
	// A missing file, or a path that a file in the way or a link that leads
	// nowhere blocks, which Windows reports as missing, is found before the
	// credential reader that it names is read through it, which would only fail.
	if _, err := os.Stat(target); err != nil {
		if failure := platform.ClassifyPath(target, platform.FileExpected, err); failure.Absent() || failure.Blocked() {
			return issue, "Codex configuration target is unavailable", missingTargetRepair(deps, failure)
		}
	}
	if _, err := adapter.CaptureTarget(target, true); err != nil {
		if status, unreadable := adapter.UnreadableStatus(err, target, deps.Host.Home); unreadable {
			return "", status.Issue, status.RepairAction
		}
	}
	if _, err := adapter.CaptureTarget(codexRecordPath(target), true); err != nil {
		if status, unreadable := adapter.UnreadableStatus(err, codexRecordPath(target), deps.Host.Home); unreadable {
			return "", status.Issue, status.RepairAction
		}
	}
	observed, readerErr := retainedRuntime(target, cfg, runtime, deps.AIGWExecutable)
	err := ValidateConfig(target, observed)
	if conflict, ok := errors.AsType[*adapter.ConflictError](err); ok {
		// Synchronization refuses an outside edit, so advising it alone would
		// send the operator back to the same refusal. A credential command
		// that AIGW did not write also fails the reader's inspection, so this
		// comes first.
		return "", conflict.Error(), conflict.NextAction()
	}
	if readerErr != nil {
		return issue, "Codex credential reader cannot be inspected", repair
	}
	switch {
	case err == nil:
		return issue, "", repair
	case errors.Is(err, os.ErrNotExist):
		return issue, "Codex configuration target is unavailable", missingTargetRepair(deps, platform.ClassifyPath(target, platform.FileExpected, err))
	default:
		return issue, "Codex configuration target does not match selected binding", repair
	}
}

// missingTargetRepair is the step, for a check, of a configured Codex file that
// failure classifies as missing or blocked. AIGW's record of a missing file in
// Codex's default home is decided first; otherwise the one fix of the path's
// condition: sync creates the default home's file again, and anywhere else the
// operator creates it, because AIGW does not create a Codex home.
func missingTargetRepair(deps adapter.Dependencies, failure *platform.PathConditionError) string {
	path := failure.Path
	autoManaged := false
	if discovered, err := discover(deps); err == nil {
		surface, ok := discovered.SurfaceForConfigPath(path)
		autoManaged = ok && surface.AutoManaged
	}
	if record := codexRecordPath(path); autoManaged && failure.Absent() {
		if _, statErr := os.Lstat(record); statErr == nil {
			return adapter.MissingWithRecord("Codex", path, record, "").NextAction()
		}
	}
	return platform.PathFix(failure.Owned(!autoManaged), platform.PathContext{Home: deps.Host.Home, Check: true})
}

func (codexAdapter) Withdraw(cfg *domain.Config) {
	delete(cfg.Clients, ID)
}

func convergedTargets(discovered discovery.Result, current []string) []string {
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	// A path that names a file already listed adds nothing.
	appendTarget := func(path string) {
		if path != "" && !slices.ContainsFunc(targets, func(kept string) bool { return sameTargetFile(kept, path) }) {
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

func reconciliationInputs(deps adapter.Dependencies, before, after domain.Config) ([]TargetRef, []TargetRef, domain.Runtime, domain.Runtime, error) {
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
	previous, _ := before.ResolveRuntime(ID, domain.ModelRef{})
	if previous.RequiresProviderToken() {
		previous.CredentialCommand = previous.CredentialExecutable(deps.AIGWExecutable)
	}
	if !afterBinding.Enabled {
		return beforeRefs, nil, previous, domain.Runtime{}, nil
	}
	runtime, err := after.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return nil, nil, domain.Runtime{}, domain.Runtime{}, err
	}
	if runtime.RequiresProviderToken() {
		runtime.CredentialCommand = runtime.CredentialExecutable(deps.AIGWExecutable)
	}
	return beforeRefs, afterRefs, previous, runtime, nil
}

func discover(deps adapter.Dependencies) (discovery.Result, error) {
	if deps.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("codex surface discovery is unavailable")
	}
	return deps.Discovery.Discover(ID), nil
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
				Path:           path,
				Executable:     executable,
				CreateIfAbsent: surface.AutoManaged,
			})
			continue
		}
		sum := sha256.Sum256([]byte(filepath.Clean(path)))
		refs = append(refs, TargetRef{
			SurfaceID:  string(HomeExplicit(hex.EncodeToString(sum[:6]))),
			Path:       path,
			Executable: executable,
		})
	}
	return refs, nil
}
