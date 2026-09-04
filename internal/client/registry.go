// Package client owns the operational contract and ordered registry for every
// admitted local client. Client-specific formats remain in their own packages;
// shared workflows depend only on this contract.
package client

import (
	"context"
	"fmt"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

// Runner is the process capability available to client adapters.
type Runner interface {
	Run(context.Context, process.Plan) error
}

// DiscoverySource is the bounded host observation surface used by adapters.
// It contains no mutation capability.
type DiscoverySource interface {
	Executable(string) string
	HomeDirectory() string
	FilePresent(string) bool
}

// Dependencies are the shared capabilities supplied to one adapter operation.
type Dependencies struct {
	Secrets            secrets.Store
	Runner             Runner
	Discovery          discovery.Discoverer
	ClaudeSettingsPath string
	AIGWExecutable     string
}

// ProjectionPlan describes one side-effect-free client projection change.
type ProjectionPlan struct {
	Client string `json:"client"`
	Target string `json:"target"`
	Action string `json:"action"`
}

// ProjectionReceipt can compensate an applied client projection while its
// owned postimage remains unchanged.
type ProjectionReceipt interface {
	Rollback() error
}

type rollbackFunc func() error

func (rollback rollbackFunc) Rollback() error { return rollback() }

// Status is one adapter's read-only local readiness observation.
type Status struct {
	Ready                bool
	Issue                string
	RepairAction         string
	NativeAuthentication string
	Checks               []Check
}

// InspectionOptions declare which read-only observations may execute while an
// adapter reports status. Native authentication may start a bounded client
// status command, so callers opt in explicitly.
type InspectionOptions struct {
	NativeAuthentication bool
}

// Check is one adapter-owned, read-only diagnostic observation.
type Check struct {
	ID           string
	Ready        bool
	Detail       string
	RepairAction string
}

// Verification is the non-sensitive result of one explicit live client probe.
type Verification struct {
	Version string
	SHA256  string
}

// Adapter is the complete operational boundary for one admitted client.
type Adapter interface {
	Spec() configuration.ClientSpec
	Discover(DiscoverySource) discovery.Result
	Converge(Dependencies, *configuration.Config, discovery.Result) error
	Plan(Dependencies, configuration.Config, configuration.Config) ([]ProjectionPlan, error)
	Apply(context.Context, Dependencies, configuration.Config, configuration.Config) (ProjectionReceipt, error)
	ProjectionChanged(configuration.Config, configuration.Config) bool
	CredentialBindingChanged(configuration.Config, configuration.Config) bool
	UsesCredentialAccount(configuration.Config, string) bool
	BindCredential(context.Context, Dependencies, configuration.Config, []string) error
	Inspect(context.Context, Dependencies, configuration.Config, configuration.Runtime, InspectionOptions) Status
	Verify(context.Context, Dependencies, configuration.Config, configuration.Runtime) (Verification, error)
	Withdraw(*configuration.Config)
}

// Registry is the sole ordered operational registry for admitted clients.
type Registry struct {
	ordered []Adapter
	byID    map[string]Adapter
	ids     []string
}

// NewRegistry validates that the operational adapters exactly implement the
// supplied admission records, in their declared order.
func NewRegistry(specs []configuration.ClientSpec, adapters ...Adapter) (Registry, error) {
	byID := make(map[string]Adapter, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			return Registry{}, fmt.Errorf("client adapter is nil")
		}
		spec := adapter.Spec()
		if spec.ID == "" {
			return Registry{}, fmt.Errorf("client adapter ID is empty")
		}
		if _, exists := byID[spec.ID]; exists {
			return Registry{}, fmt.Errorf("client adapter %q is registered more than once", spec.ID)
		}
		byID[spec.ID] = adapter
	}
	admitted := make(map[string]configuration.ClientSpec, len(specs))
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		if seen[spec.ID] {
			return Registry{}, fmt.Errorf("client admission %q is declared more than once", spec.ID)
		}
		seen[spec.ID] = true
		admitted[spec.ID] = spec
		adapter, ok := byID[spec.ID]
		if !ok {
			return Registry{}, fmt.Errorf("client admission %q has no operational adapter", spec.ID)
		}
		if adapter.Spec() != spec {
			return Registry{}, fmt.Errorf("client adapter %q does not match its admission record", spec.ID)
		}
	}
	if len(admitted) != len(adapters) {
		return Registry{}, fmt.Errorf("operational client registry contains an unadmitted adapter")
	}
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		ids = append(ids, spec.ID)
	}
	return Registry{ordered: append([]Adapter(nil), adapters...), byID: byID, ids: ids}, nil
}

// Discover observes every admitted client through its adapter.
func (registry Registry) Discover(source DiscoverySource) discovery.Result {
	result := discovery.Result{Executables: make(map[string]string)}
	for _, adapter := range registry.ordered {
		observed := adapter.Discover(source)
		for clientID, executable := range observed.Executables {
			result.Executables[clientID] = executable
		}
		result.Surfaces = append(result.Surfaces, observed.Surfaces...)
	}
	return result
}

// Converge derives adapter configuration for the requested clients. An empty
// list means every admitted adapter.
func (registry Registry) Converge(deps Dependencies, before configuration.Config, discovered discovery.Result, clients ...string) (configuration.Config, error) {
	after := before.Clone()
	adapters, err := registry.selectAdapters(clients)
	if err != nil {
		return configuration.Config{}, err
	}
	for _, adapter := range adapters {
		if err := adapter.Converge(deps, &after, discovered); err != nil {
			return configuration.Config{}, err
		}
	}
	return after, nil
}

// Plan prepares every adapter before any projection is written.
func (registry Registry) Plan(deps Dependencies, before, after configuration.Config) ([]ProjectionPlan, error) {
	plans := make([]ProjectionPlan, 0)
	for _, adapter := range registry.ordered {
		adapterPlans, err := adapter.Plan(deps, before, after)
		if err != nil {
			return nil, err
		}
		plans = append(plans, adapterPlans...)
	}
	return plans, nil
}

// Apply executes the already-preparable adapter set and compensates successful
// earlier adapters in reverse order if a later adapter fails.
func (registry Registry) Apply(ctx context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	if _, err := registry.Plan(deps, before, after); err != nil {
		return nil, err
	}
	receipts := make([]ProjectionReceipt, 0, len(registry.ordered))
	for _, adapter := range registry.ordered {
		receipt, err := adapter.Apply(ctx, deps, before, after)
		if err != nil {
			if rollbackErr := rollbackReceipts(receipts); rollbackErr != nil {
				return nil, fmt.Errorf("client projection failed: %w; rollback also failed: %v", err, rollbackErr)
			}
			return nil, fmt.Errorf("client projection failed and prior adapters were rolled back: %w", err)
		}
		receipts = append(receipts, receipt)
	}
	return rollbackFunc(func() error { return rollbackReceipts(receipts) }), nil
}

// ProjectionChanged reports whether any admitted adapter owns a changed
// persistent projection.
func (registry Registry) ProjectionChanged(before, after configuration.Config) bool {
	for _, adapter := range registry.ordered {
		if adapter.ProjectionChanged(before, after) {
			return true
		}
	}
	return false
}

// CredentialBindingChanged reports whether any admitted adapter must refresh
// native authentication for a configuration transition.
func (registry Registry) CredentialBindingChanged(before, after configuration.Config) bool {
	for _, adapter := range registry.ordered {
		if adapter.CredentialBindingChanged(before, after) {
			return true
		}
	}
	return false
}

// UsesCredentialAccount reports whether an admitted native credential binding
// currently depends on accountID.
func (registry Registry) UsesCredentialAccount(cfg configuration.Config, accountID string) bool {
	for _, adapter := range registry.ordered {
		if adapter.UsesCredentialAccount(cfg, accountID) {
			return true
		}
	}
	return false
}

// BindChangedCredentials updates only adapters whose native binding changed.
func (registry Registry) BindChangedCredentials(ctx context.Context, deps Dependencies, before, after configuration.Config) error {
	for _, adapter := range registry.ordered {
		if !adapter.CredentialBindingChanged(before, after) {
			continue
		}
		if err := adapter.BindCredential(ctx, deps, after, nil); err != nil {
			return err
		}
	}
	return nil
}

// BindCredentialsForAccount refreshes every admitted native credential
// projection that currently depends on accountID.
func (registry Registry) BindCredentialsForAccount(ctx context.Context, deps Dependencies, cfg configuration.Config, accountID string) error {
	for _, adapter := range registry.ordered {
		if !adapter.UsesCredentialAccount(cfg, accountID) {
			continue
		}
		if err := adapter.BindCredential(ctx, deps, cfg, nil); err != nil {
			return err
		}
	}
	return nil
}

// BindCredential updates one admitted client's native credential projection.
func (registry Registry) BindCredential(ctx context.Context, deps Dependencies, cfg configuration.Config, clientID string, targets []string) error {
	adapter, err := registry.adapter(clientID)
	if err != nil {
		return err
	}
	return adapter.BindCredential(ctx, deps, cfg, targets)
}

// Inspect observes one client without mutating it. Unknown clients are reported
// as an unavailable status so read-only callers need no parallel error path.
func (registry Registry) Inspect(ctx context.Context, deps Dependencies, cfg configuration.Config, clientID string, runtime configuration.Runtime, options InspectionOptions) Status {
	adapter, ok := registry.byID[clientID]
	if !ok {
		return Status{Issue: fmt.Sprintf("client %q has no admitted operational adapter", clientID), RepairAction: "aigw repair"}
	}
	return adapter.Inspect(ctx, deps, cfg, runtime, options)
}

// Verify runs one explicit live verification through the admitted adapter.
func (registry Registry) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, clientID string, runtime configuration.Runtime) (Verification, error) {
	adapter, err := registry.adapter(clientID)
	if err != nil {
		return Verification{}, err
	}
	return adapter.Verify(ctx, deps, cfg, runtime)
}

// Withdraw removes the selected adapter from desired configuration. The
// synchronization transaction then uses the same adapter to remove owned state.
func (registry Registry) Withdraw(cfg *configuration.Config, clientID string) error {
	adapter, err := registry.adapter(clientID)
	if err != nil {
		return err
	}
	adapter.Withdraw(cfg)
	return nil
}

// IDs returns admitted clients in the registry's stable order.
func (registry Registry) IDs() []string {
	return append([]string(nil), registry.ids...)
}

// Empty reports whether the registry has no admitted adapters.
func (registry Registry) Empty() bool { return len(registry.ordered) == 0 }

func (registry Registry) selectAdapters(clientIDs []string) ([]Adapter, error) {
	if len(clientIDs) == 0 {
		return registry.ordered, nil
	}
	selected := make([]Adapter, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		adapter, err := registry.adapter(clientID)
		if err != nil {
			return nil, err
		}
		selected = append(selected, adapter)
	}
	return selected, nil
}

func (registry Registry) adapter(clientID string) (Adapter, error) {
	adapter, ok := registry.byID[clientID]
	if !ok {
		return nil, fmt.Errorf("client %q has no admitted operational adapter", clientID)
	}
	return adapter, nil
}

func rollbackReceipts(receipts []ProjectionReceipt) error {
	for index := len(receipts) - 1; index >= 0; index-- {
		if receipts[index] == nil {
			continue
		}
		if err := receipts[index].Rollback(); err != nil {
			return err
		}
	}
	return nil
}
