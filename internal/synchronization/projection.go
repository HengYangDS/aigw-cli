package synchronization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

// ProjectionPlan describes one non-secret client configuration mutation.
type ProjectionPlan struct {
	Client string `json:"client"`
	Target string `json:"target"`
	Action string `json:"action"`
}

// Plan returns every side-effect-free client projection change for a
// configuration transition, including target removals.
func (s Synchronizer) Plan(before, after configuration.Config) ([]ProjectionPlan, error) {
	beforeRefs, afterRefs, runtime, err := s.reconciliationInputs(before, after)
	if err != nil {
		return nil, err
	}
	codexPlans, err := codex.PlanReconciliation(beforeRefs, afterRefs, runtime)
	if err != nil {
		return nil, err
	}
	plans := make([]ProjectionPlan, 0, len(codexPlans)+1)
	for _, plan := range codexPlans {
		plans = append(plans, ProjectionPlan{Client: configuration.ClientCodex, Target: plan.Target, Action: plan.Action})
	}
	if !ClaudeProjectionChanged(before, after) {
		return plans, nil
	}
	if s.ClaudeSettingsPath == "" {
		return nil, fmt.Errorf("Claude settings path is unavailable")
	}
	disabled := !after.Adapters[configuration.ClientClaude].Enabled
	var claudeRuntime configuration.Runtime
	if !disabled {
		claudeRuntime, err = after.ResolveRuntime(configuration.ClientClaude, "")
		if err != nil {
			return nil, err
		}
	}
	claudePlan, err := claude.PlanSettings(s.ClaudeSettingsPath, disabled, claudeRuntime, s.AIGWExecutable)
	if err != nil {
		return nil, err
	}
	return append(plans, ProjectionPlan{Client: configuration.ClientClaude, Target: claudePlan.Target, Action: claudePlan.Action}), nil
}

// Reconcile applies every admitted client projection for one configuration
// transition. Codex and Claude retain separate format owners while this package
// provides the single transaction boundary.
func (s Synchronizer) Reconcile(_ context.Context, before, after configuration.Config) error {
	beforeRefs, afterRefs, runtime, err := s.reconciliationInputs(before, after)
	if err != nil {
		return err
	}
	if _, err = codex.ReconcileConfigs(beforeRefs, afterRefs, runtime); err != nil {
		return err
	}
	if !ClaudeProjectionChanged(before, after) {
		return nil
	}
	if s.ClaudeSettingsPath == "" {
		return fmt.Errorf("Claude settings path is unavailable")
	}
	adapter := after.Adapters[configuration.ClientClaude]
	if !adapter.Enabled {
		_, err = claude.ReconcileSettings(s.ClaudeSettingsPath, true, configuration.Runtime{}, "")
		return err
	}
	claudeRuntime, err := after.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return err
	}
	_, err = claude.ReconcileSettings(s.ClaudeSettingsPath, false, claudeRuntime, s.AIGWExecutable)
	return err
}

func (s Synchronizer) reconciliationInputs(before, after configuration.Config) ([]codex.TargetRef, []codex.TargetRef, configuration.Runtime, error) {
	beforeAdapter := before.Adapters[configuration.ClientCodex]
	afterAdapter := after.Adapters[configuration.ClientCodex]
	if !beforeAdapter.Enabled && !afterAdapter.Enabled {
		return nil, nil, configuration.Runtime{}, nil
	}
	discovered, err := s.discoveredResult()
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	beforeRefs, err := codexTargetRefs(discovered, beforeAdapter.Targets, codexExecutable(discovered, beforeAdapter))
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	afterRefs, err := codexTargetRefs(discovered, afterAdapter.Targets, codexExecutable(discovered, afterAdapter))
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	if !afterAdapter.Enabled {
		return beforeRefs, nil, configuration.Runtime{}, nil
	}
	runtime, err := after.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	runtime.CredentialCommand = s.AIGWExecutable
	return beforeRefs, afterRefs, runtime, nil
}

func (s Synchronizer) discoveredResult() (discovery.Result, error) {
	if s.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("Codex surface discovery is unavailable")
	}
	return s.Discovery.Discover(), nil
}

// codexExecutable prefers the client the configuration explicitly names and
// falls back to the discovered one, matching how the repair workflow resolves
// the same client.
func codexExecutable(discovered discovery.Result, adapter configuration.AdapterConfig) string {
	if adapter.Executable != "" {
		return adapter.Executable
	}
	return discovered.Executable(configuration.ClientCodex)
}

func codexTargetRefs(discovered discovery.Result, paths []string, executable string) ([]codex.TargetRef, error) {
	refs := make([]codex.TargetRef, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			return nil, fmt.Errorf("Codex config target is empty")
		}
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			refs = append(refs, codex.TargetRef{
				SurfaceID:      surface.ID,
				Authority:      surface.Authority,
				ProjectionMode: codex.ProjectionFullSelection,
				Path:           path,
				Executable:     executable,
				CreateIfAbsent: surface.AutoManaged,
			})
			continue
		}
		sum := sha256.Sum256([]byte(filepath.Clean(path)))
		refs = append(refs, codex.TargetRef{
			SurfaceID:      string(surfaceidentity.CodexHomeExplicit(hex.EncodeToString(sum[:6]))),
			Authority:      string(surfaceidentity.AuthorityAIGW),
			ProjectionMode: codex.ProjectionFullSelection,
			Path:           path,
			Executable:     executable,
		})
	}
	return refs, nil
}

// ProjectionChanged reports whether a transition changes the persistent Codex
// projection. Invalid runtime state is conservatively treated as changed.
func ProjectionChanged(before, after configuration.Config) bool {
	beforeAdapter := before.Adapters[configuration.ClientCodex]
	afterAdapter := after.Adapters[configuration.ClientCodex]
	if beforeAdapter.Enabled != afterAdapter.Enabled {
		return true
	}
	if !afterAdapter.Enabled {
		return false
	}
	if !beforeAdapter.Enabled || !slices.Equal(beforeAdapter.Targets, afterAdapter.Targets) {
		return true
	}
	beforeRuntime, beforeErr := before.ResolveRuntime(configuration.ClientCodex, "")
	afterRuntime, afterErr := after.ResolveRuntime(configuration.ClientCodex, "")
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return beforeRuntime.ProfileID != afterRuntime.ProfileID ||
		beforeRuntime.ProfileLabel != afterRuntime.ProfileLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model ||
		beforeRuntime.ModelProvider != afterRuntime.ModelProvider
}

// ClaudeProjectionChanged reports whether a transition changes the persistent
// Claude Code settings projection. The executable path is readiness metadata,
// not part of the settings projection itself.
func ClaudeProjectionChanged(before, after configuration.Config) bool {
	beforeAdapter := before.Adapters[configuration.ClientClaude]
	afterAdapter := after.Adapters[configuration.ClientClaude]
	if beforeAdapter.Enabled != afterAdapter.Enabled {
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
	return beforeRuntime.AccountID != afterRuntime.AccountID ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model
}
