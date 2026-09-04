package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	domainverification "aigw-cli/internal/verification"
)

const nativeAuthenticationInspectionTimeout = 5 * time.Second

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
	return beforeRuntime.ProfileID != afterRuntime.ProfileID ||
		beforeRuntime.ProfileLabel != afterRuntime.ProfileLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model ||
		beforeRuntime.ModelProvider != afterRuntime.ModelProvider ||
		beforeRuntime.Authentication != afterRuntime.Authentication
}

func (codexAdapter) CredentialBindingChanged(before, after configuration.Config) bool {
	beforeAdapter := before.Adapters[configuration.ClientCodex]
	afterAdapter := after.Adapters[configuration.ClientCodex]
	if !afterAdapter.Enabled {
		return false
	}
	if !beforeAdapter.Enabled || !slices.Equal(beforeAdapter.Targets, afterAdapter.Targets) {
		runtime, err := after.ResolveRuntime(configuration.ClientCodex, "")
		return err != nil || runtime.RequiresAccountToken() && runtime.ModelProvider == configuration.ModelProviderAIGW
	}
	beforeRuntime, beforeErr := before.ResolveRuntime(configuration.ClientCodex, "")
	afterRuntime, afterErr := after.ResolveRuntime(configuration.ClientCodex, "")
	if afterErr != nil {
		return true
	}
	if !afterRuntime.RequiresAccountToken() || afterRuntime.ModelProvider != configuration.ModelProviderAIGW {
		return false
	}
	return beforeErr != nil || !beforeRuntime.RequiresAccountToken() || beforeRuntime.ModelProvider != configuration.ModelProviderAIGW || beforeRuntime.AccountID != afterRuntime.AccountID
}

func (codexAdapter) UsesCredentialAccount(cfg configuration.Config, accountID string) bool {
	adapter := cfg.Adapters[configuration.ClientCodex]
	if !adapter.Enabled {
		return false
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	return err == nil && runtime.RequiresAccountToken() && runtime.ModelProvider == configuration.ModelProviderAIGW && runtime.AccountID == accountID
}

func (codexAdapter) BindCredential(ctx context.Context, deps Dependencies, cfg configuration.Config, targets []string) error {
	adapter := cfg.Adapters[configuration.ClientCodex]
	if !adapter.Enabled {
		return fmt.Errorf("Codex authentication requires an enabled adapter")
	}
	if targets == nil {
		targets = adapter.Targets
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return err
	}
	if !runtime.RequiresAccountToken() || runtime.ModelProvider != configuration.ModelProviderAIGW {
		return nil
	}
	if adapter.Executable == "" || deps.Runner == nil {
		return fmt.Errorf("Codex authentication requires an enabled adapter executable")
	}
	if deps.Secrets == nil {
		return fmt.Errorf("Token for the Codex route is unavailable: secret store is unavailable")
	}
	token, err := deps.Secrets.Get(runtime.AccountID)
	if err != nil {
		return fmt.Errorf("Token for the Codex route is unavailable: %w", err)
	}
	for _, target := range targets {
		plan, err := codex.LoginPlan(adapter.Executable, filepath.Dir(target), token)
		if err != nil {
			return err
		}
		targetCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err = deps.Runner.Run(targetCtx, plan)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func (codexAdapter) Inspect(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime, options InspectionOptions) Status {
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
	if !status.Ready {
		return status
	}
	if !options.NativeAuthentication {
		return status
	}
	if !runtime.RequiresAccountToken() || runtime.ModelProvider != configuration.ModelProviderAIGW {
		status.NativeAuthentication = "not_required"
		return status
	}
	status.NativeAuthentication = "not_proven"
	capture, ok := deps.Runner.(process.CaptureRunner)
	if !ok {
		return status
	}
	for _, target := range adapter.Targets {
		plan, err := codex.LoginStatusPlan(adapter.Executable, filepath.Dir(target))
		if err != nil {
			return status
		}
		probeCtx, cancel := context.WithTimeout(ctx, nativeAuthenticationInspectionTimeout)
		_, err = capture.RunCapture(probeCtx, plan)
		cancel()
		if err != nil {
			return status
		}
	}
	status.NativeAuthentication = "present"
	return status
}

func (codexAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, runtime configuration.Runtime) (Verification, error) {
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
	plan, err := claude.PlanSettings(deps.ClaudeSettingsPath, disabled, runtime, deps.AIGWExecutable)
	if err != nil {
		return nil, err
	}
	return []ProjectionPlan{{Client: configuration.ClientClaude, Target: plan.Target, Action: plan.Action}}, nil
}

func (claudeAdapter) Apply(_ context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	if !claudeProjectionRequired(before, after) {
		return rollbackFunc(func() error { return nil }), nil
	}
	disabled, runtime, err := claudeProjectionInput(after)
	if err != nil {
		return nil, err
	}
	if _, err := claude.ReconcileSettings(deps.ClaudeSettingsPath, disabled, runtime, deps.AIGWExecutable); err != nil {
		return nil, err
	}
	return rollbackFunc(func() error {
		rollbackDisabled, rollbackRuntime, err := claudeProjectionInput(before)
		if err != nil {
			return err
		}
		_, err = claude.ReconcileSettings(deps.ClaudeSettingsPath, rollbackDisabled, rollbackRuntime, deps.AIGWExecutable)
		return err
	}), nil
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

func (claudeAdapter) CredentialBindingChanged(configuration.Config, configuration.Config) bool {
	return false
}

func (claudeAdapter) UsesCredentialAccount(configuration.Config, string) bool { return false }

func (claudeAdapter) BindCredential(context.Context, Dependencies, configuration.Config, []string) error {
	return nil
}

func (claudeAdapter) Inspect(_ context.Context, _ Dependencies, cfg configuration.Config, _ configuration.Runtime, _ InspectionOptions) Status {
	adapter := cfg.Adapters[configuration.ClientClaude]
	if !adapter.Enabled {
		return Status{Issue: "Claude adapter is disabled", RepairAction: "aigw sync"}
	}
	if adapter.Executable == "" {
		return Status{Issue: "Claude executable is not configured", RepairAction: "aigw repair"}
	}
	ready, err := claude.Ready(adapter.Executable)
	if err != nil {
		return Status{Issue: "Cannot inspect Claude executable", RepairAction: "aigw repair"}
	}
	if !ready {
		return Status{Issue: "Claude executable is unavailable", RepairAction: "aigw repair"}
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
	verifyCtx, cancel := context.WithTimeout(ctx, domainverification.ProtocolTimeout)
	defer cancel()
	return Verification{}, domainverification.VerifyClaudeInvocation(verifyCtx, deps.Runner, cfg, runtime, token)
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
	if runtime.RequiresAccountToken() && runtime.ModelProvider != configuration.ModelProviderAIGW {
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
