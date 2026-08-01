package synchronization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

// Plan returns the side-effect-free Codex projection changes for a
// configuration transition, including target removals.
func (s Synchronizer) Plan(before, after configuration.Config) ([]codex.ProjectionPlan, error) {
	beforeRefs, afterRefs, runtime, err := s.reconciliationInputs(before, after)
	if err != nil {
		return nil, err
	}
	return codex.PlanReconciliation(beforeRefs, afterRefs, runtime)
}

// Reconcile applies the Codex projection for one configuration transition.
// Claude resolves its route inside the process-bound launcher and therefore
// has no persistent projection here.
func (s Synchronizer) Reconcile(_ context.Context, before, after configuration.Config) error {
	beforeRefs, afterRefs, runtime, err := s.reconciliationInputs(before, after)
	if err != nil {
		return err
	}
	_, err = codex.ReconcileConfigs(beforeRefs, afterRefs, runtime)
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
	beforeRefs, err := codexTargetRefs(discovered, beforeAdapter.Targets)
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	afterRefs, err := codexTargetRefs(discovered, afterAdapter.Targets)
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	if !afterAdapter.Enabled {
		return beforeRefs, nil, configuration.Runtime{}, nil
	}
	runtime, _, err := after.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return nil, nil, configuration.Runtime{}, err
	}
	return beforeRefs, afterRefs, runtime, nil
}

func (s Synchronizer) discoveredResult() (discovery.Result, error) {
	if s.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("Codex surface discovery is unavailable")
	}
	return s.Discovery.Discover(), nil
}

func codexTargetRefs(discovered discovery.Result, paths []string) ([]codex.TargetRef, error) {
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
			})
			continue
		}
		sum := sha256.Sum256([]byte(filepath.Clean(path)))
		refs = append(refs, codex.TargetRef{
			SurfaceID:      string(surfaceidentity.CodexCLIExplicit(hex.EncodeToString(sum[:6]))),
			Authority:      string(surfaceidentity.AuthorityAIGW),
			ProjectionMode: codex.ProjectionFullSelection,
			Path:           path,
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
	beforeRuntime, _, beforeErr := before.ResolveRuntime(configuration.ClientCodex, "")
	afterRuntime, _, afterErr := after.ResolveRuntime(configuration.ClientCodex, "")
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return beforeRuntime.ProfileID != afterRuntime.ProfileID ||
		beforeRuntime.ProfileLabel != afterRuntime.ProfileLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model ||
		beforeRuntime.CodexResponsesStorage != afterRuntime.CodexResponsesStorage
}
