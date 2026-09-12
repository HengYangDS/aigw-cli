package client

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
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	domainverification "aigw-cli/internal/verification"
)

var defaultRegistry = mustRegistry(
	configuration.AdmittedClientSpecs(),
	codexAdapter{},
	claudeAdapter{},
)

// DefaultRegistry returns the immutable built-in adapter registry.
func DefaultRegistry() Registry { return defaultRegistry }

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

type codexAdapter struct{}

func (codexAdapter) Spec() configuration.ClientSpec {
	return mustClientSpec(configuration.ClientCodex)
}

func (codexAdapter) Discover(source DiscoverySource) discovery.Result {
	path := filepath.Join(source.HomeDirectory(), ".codex", "config.toml")
	return discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: source.Executable(configuration.ClientCodex)},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Product:     "Codex",
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  path,
			Present:     source.FilePresent(path),
			AutoManaged: true,
		}},
	}
}

func (codexAdapter) Converge(deps Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return nil
	}
	adapter := cfg.Adapters[configuration.ClientCodex]
	targets := codexTargets(discovered, adapter.Targets)
	executable, err := resolveExecutable(configuration.ClientCodex, adapter.Executable, discovered.Executable(configuration.ClientCodex))
	if err != nil {
		return err
	}
	available := !runtime.RequiresAccountToken()
	if runtime.RequiresAccountToken() {
		available, err = secretAvailable(deps.Secrets, runtime.AccountID)
		if err != nil {
			return err
		}
	}
	if executable != "" && len(targets) > 0 && (adapter.Enabled || available) {
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: targets}
	} else if adapter.Enabled && len(targets) == 0 {
		delete(cfg.Adapters, configuration.ClientCodex)
	}
	return nil
}

func (codexAdapter) Plan(deps Dependencies, before, after configuration.Config) ([]ProjectionPlan, error) {
	beforeRefs, afterRefs, runtime, err := codexReconciliationInputs(deps, before, after)
	if err != nil {
		return nil, err
	}
	plans, err := codex.PlanReconciliation(beforeRefs, afterRefs, runtime)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectionPlan, 0, len(plans))
	for _, plan := range plans {
		result = append(result, ProjectionPlan{Client: configuration.ClientCodex, Target: plan.Target, Action: plan.Action})
	}
	return result, nil
}

func (codexAdapter) Apply(_ context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	beforeRefs, afterRefs, runtime, err := codexReconciliationInputs(deps, before, after)
	if err != nil {
		return nil, err
	}
	return codex.ReconcileConfigs(beforeRefs, afterRefs, runtime)
}

func (codexAdapter) ProjectionChanged(before, after configuration.Config) bool {
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
	return beforeRuntime.AccountID != afterRuntime.AccountID ||
		beforeRuntime.ProfileID != afterRuntime.ProfileID ||
		beforeRuntime.ProfileLabel != afterRuntime.ProfileLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model ||
		beforeRuntime.ModelProvider != afterRuntime.ModelProvider ||
		beforeRuntime.Authentication != afterRuntime.Authentication
}

func (codexAdapter) Inspect(_ context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime) Status {
	if deps.AIGWExecutable != "" {
		runtime.CredentialCommand = deps.AIGWExecutable
	}
	adapter := cfg.Adapters[configuration.ClientCodex]
	if !adapter.Enabled {
		return Status{Issue: "Codex adapter is disabled", RepairAction: "aigw sync"}
	}
	if adapter.Executable == "" {
		return Status{Issue: "Codex executable is not configured", RepairAction: "aigw repair"}
	}
	if len(adapter.Targets) == 0 {
		return Status{Issue: "Codex configuration target is missing", RepairAction: "aigw repair"}
	}
	status := Status{Ready: true, Checks: make([]Check, 0, len(adapter.Targets))}
	for index, target := range adapter.Targets {
		check := Check{ID: fmt.Sprintf("codex:target-%d", index+1), Ready: true, Detail: "profile " + runtime.ProfileID}
		if err := codex.ValidateConfig(target, runtime); err != nil {
			check.Ready = false
			check.Detail = err.Error()
			check.RepairAction = "aigw sync"
			status.Ready = false
			status.Issue = "Codex configuration projection drift: " + err.Error()
			status.RepairAction = "aigw sync"
		}
		status.Checks = append(status.Checks, check)
	}

	return status
}

func (codexAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime) (Verification, error) {
	if deps.AIGWExecutable != "" {
		runtime.CredentialCommand = deps.AIGWExecutable
	}
	verifyCtx, cancel := context.WithTimeout(ctx, domainverification.ProtocolTimeout)
	defer cancel()
	identity, err := domainverification.VerifyCodexInvocation(verifyCtx, deps.Runner, cfg, runtime)
	return Verification{Version: identity.Version, SHA256: identity.SHA256}, err
}

func (codexAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Adapters, configuration.ClientCodex)
}

type claudeAdapter struct{}

func (claudeAdapter) Spec() configuration.ClientSpec {
	return mustClientSpec(configuration.ClientClaude)
}

func (claudeAdapter) Discover(source DiscoverySource) discovery.Result {
	return discovery.Result{Executables: map[string]string{configuration.ClientClaude: source.Executable(configuration.ClientClaude)}}
}

func (claudeAdapter) Converge(deps Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return nil
	}
	adapter := cfg.Adapters[configuration.ClientClaude]
	executable, err := resolveExecutable(configuration.ClientClaude, adapter.Executable, discovered.Executable(configuration.ClientClaude))
	if err != nil {
		return err
	}
	available, err := secretAvailable(deps.Secrets, runtime.AccountID)
	if err != nil {
		return err
	}
	if executable != "" && (adapter.Enabled || available) {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
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
	plan, err := claude.PlanSettings(deps.ClaudeSettingsPath, disabled, runtime, deps.AIGWExecutable, previous.Model)
	if err != nil {
		return nil, err
	}
	return []ProjectionPlan{{Client: configuration.ClientClaude, Target: plan.Target, Action: plan.Action}}, nil
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
	return claude.ReconcileSettings(deps.ClaudeSettingsPath, disabled, runtime, deps.AIGWExecutable, previous.Model)
}

func (claudeAdapter) ProjectionChanged(before, after configuration.Config) bool {
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
	return beforeRuntime.AccountID != afterRuntime.AccountID || beforeRuntime.Endpoint != afterRuntime.Endpoint || beforeRuntime.Model != afterRuntime.Model
}

func (claudeAdapter) Inspect(_ context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime) Status {
	adapter := cfg.Adapters[configuration.ClientClaude]
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
	if err := claude.ValidateSettings(deps.ClaudeSettingsPath, runtime, deps.AIGWExecutable); err != nil {
		return Status{Issue: err.Error(), RepairAction: "aigw sync"}
	}
	return Status{Ready: true}
}

func (claudeAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime) (Verification, error) {
	if deps.Secrets == nil {
		return Verification{}, fmt.Errorf("Token for account %q is unavailable: secret store is unavailable", runtime.AccountID)
	}
	token, err := deps.Secrets.Get(runtime.AccountID)
	if err != nil {
		instruction, _ := credential.TokenRecovery(deps.Secrets, runtime.AccountID)
		return Verification{}, fmt.Errorf("Token for account %q is unavailable: %w; %s", runtime.AccountID, err, instruction)
	}
	adapter := cfg.Adapters[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return Verification{}, fmt.Errorf("Claude adapter is disabled; run `aigw repair`")
	}
	ready, err := discovery.ExecutableAvailable(adapter.Executable)
	if err != nil {
		return Verification{}, fmt.Errorf("inspect Claude executable: %w", err)
	}
	if !ready {
		return Verification{}, fmt.Errorf("Claude executable is unavailable; run `aigw repair`")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, domainverification.ProtocolTimeout)
	defer cancel()
	runtime.CredentialCommand = deps.AIGWExecutable
	return Verification{}, domainverification.VerifyClaudeRuntime(verifyCtx, deps.Runner, adapter.Executable, deps.ClaudeSettingsPath, runtime, token)
}

func (claudeAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Adapters, configuration.ClientClaude)
}

func mustRegistry(specs []configuration.ClientSpec, adapters ...Adapter) Registry {
	registry, err := NewRegistry(specs, adapters...)
	if err != nil {
		panic(err)
	}
	return registry
}

func mustClientSpec(clientID string) configuration.ClientSpec {
	spec, ok := configuration.ClientSpecFor(clientID)
	if !ok {
		panic("missing client admission: " + clientID)
	}
	return spec
}

func resolveExecutable(clientID, configured, discovered string) (string, error) {
	available, err := discovery.ExecutableAvailable(configured)
	if err != nil {
		return "", fmt.Errorf("inspect configured %s executable: %w", clientID, err)
	}
	if available || discovered == "" {
		return configured, nil
	}
	return discovered, nil
}

func secretAvailable(store secrets.Store, accountID string) (bool, error) {
	if store == nil {
		return false, nil
	}
	return store.Exists(accountID)
}

func codexTargets(discovered discovery.Result, current []string) []string {
	seen := map[string]bool{}
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	appendTarget := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	for _, path := range discovered.AutoManagedCodexTargets() {
		appendTarget(path)
	}
	for _, path := range current {
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			if surface.ID == string(surfaceidentity.CodexHomeDefault) {
				appendTarget(path)
			}
			continue
		}
		appendTarget(path)
	}
	return targets
}

func codexReconciliationInputs(deps Dependencies, before, after configuration.Config) ([]codex.TargetRef, []codex.TargetRef, configuration.Runtime, error) {
	beforeAdapter := before.Adapters[configuration.ClientCodex]
	afterAdapter := after.Adapters[configuration.ClientCodex]
	if !beforeAdapter.Enabled && !afterAdapter.Enabled {
		return nil, nil, configuration.Runtime{}, nil
	}
	discovered, err := discover(deps)
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
	if runtime.RequiresAccountToken() {
		runtime.CredentialCommand = deps.AIGWExecutable
	}
	return beforeRefs, afterRefs, runtime, nil
}

func discover(deps Dependencies) (discovery.Result, error) {
	if deps.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("Codex surface discovery is unavailable")
	}
	return deps.Discovery.Discover(), nil
}

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

func claudeProjectionRequired(before, after configuration.Config) bool {
	return before.Adapters[configuration.ClientClaude].Enabled || after.Adapters[configuration.ClientClaude].Enabled
}

func claudeProjectionInput(cfg configuration.Config) (bool, configuration.Runtime, error) {
	disabled := !cfg.Adapters[configuration.ClientClaude].Enabled
	if disabled {
		return true, configuration.Runtime{}, nil
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	return false, runtime, err
}
