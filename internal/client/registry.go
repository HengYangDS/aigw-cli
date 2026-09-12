// Package client owns the operational contract and admission-ordered registry for every
// admitted local client. Client-specific formats remain in their own packages;
// shared workflows depend only on this contract.
package client

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

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
	Runner             process.CaptureRunner
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

// Status is one adapter's read-only local readiness observation.
type Status struct {
	Ready        bool
	Issue        string
	RepairAction string
	Checks       []Check
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
	Inspect(context.Context, Dependencies, configuration.Config, configuration.Runtime) Status
	Verify(context.Context, Dependencies, configuration.Config, configuration.Runtime) (Verification, error)
	Withdraw(*configuration.Config)
}

// Registry is the sole ordered operational registry for admitted clients.
type Registry struct {
	byID map[string]Adapter
	ids  []string
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
	seen := make(map[string]bool, len(specs))
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		if seen[spec.ID] {
			return Registry{}, fmt.Errorf("client admission %q is declared more than once", spec.ID)
		}
		seen[spec.ID] = true
		adapter, ok := byID[spec.ID]
		if !ok {
			return Registry{}, fmt.Errorf("client admission %q has no operational adapter", spec.ID)
		}
		if adapter.Spec() != spec {
			return Registry{}, fmt.Errorf("client adapter %q does not match its admission record", spec.ID)
		}
		ids = append(ids, spec.ID)
	}
	if len(specs) != len(adapters) {
		return Registry{}, fmt.Errorf("operational client registry contains an unadmitted adapter")
	}
	return Registry{byID: byID, ids: ids}, nil
}

// Discover observes every admitted client through its adapter.
func (registry Registry) Discover(source DiscoverySource) discovery.Result {
	result := discovery.Result{Executables: make(map[string]string)}
	for _, id := range registry.ids {
		adapter := registry.byID[id]
		observed := adapter.Discover(source)
		maps.Copy(result.Executables, observed.Executables)
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

// Plan prepares requested adapters before any projection is written. An empty
// client list selects every admitted adapter.
func (registry Registry) Plan(deps Dependencies, before, after configuration.Config, clientIDs ...string) ([]ProjectionPlan, error) {
	adapters, err := registry.selectAdapters(clientIDs)
	if err != nil {
		return nil, err
	}
	plans := make([]ProjectionPlan, 0)
	for _, adapter := range adapters {
		adapterPlans, err := adapter.Plan(deps, before, after)
		if err != nil {
			return nil, err
		}
		plans = append(plans, adapterPlans...)
	}
	return plans, nil
}

// Apply executes the already-preparable adapter set and compensates successful
// earlier adapters in reverse order if a later adapter fails or cancellation
// prevents the next adapter from starting.
func (registry Registry) Apply(ctx context.Context, deps Dependencies, before, after configuration.Config, clientIDs ...string) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	adapters, err := registry.selectAdapters(clientIDs)
	if err != nil {
		return err
	}
	if _, err := registry.Plan(deps, before, after, clientIDs...); err != nil {
		return err
	}
	receipts := make([]ProjectionReceipt, 0, len(adapters))
	defer func() {
		if resultErr == nil {
			return
		}
		if rollbackErr := rollbackReceipts(receipts); rollbackErr != nil {
			resultErr = fmt.Errorf("client projection failed: %w; rollback also failed: %w", resultErr, rollbackErr)
			return
		}
		resultErr = fmt.Errorf("client projection failed and prior adapters were rolled back: %w", resultErr)
	}()
	for _, adapter := range adapters {
		if err := ctx.Err(); err != nil {
			return err
		}
		receipt, err := adapter.Apply(ctx, deps, before, after)
		if err != nil {
			return err
		}
		receipts = append(receipts, receipt)
	}
	return nil
}

// ChangedClients returns clients whose persistent projection changes, in admission order.
func (registry Registry) ChangedClients(before, after configuration.Config) []string {
	var changed []string
	for _, id := range registry.ids {
		adapter := registry.byID[id]
		if adapter.ProjectionChanged(before, after) {
			changed = append(changed, id)
		}
	}
	return changed
}

// Inspect observes one client without mutating it. Unknown clients are reported
// as an unavailable status so read-only callers need no parallel error path.
func (registry Registry) Inspect(ctx context.Context, deps Dependencies, cfg configuration.Config, clientID string, runtime configuration.Runtime) Status {
	adapter, ok := registry.byID[clientID]
	if !ok {
		return Status{Issue: fmt.Sprintf("client %q has no admitted operational adapter", clientID), RepairAction: "aigw repair"}
	}
	return adapter.Inspect(ctx, deps, cfg, runtime)
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
	return slices.Clone(registry.ids)
}

// Empty reports whether the registry has no admitted adapters.
func (registry Registry) Empty() bool { return len(registry.ids) == 0 }

func (registry Registry) selectAdapters(clientIDs []string) ([]Adapter, error) {
	if len(clientIDs) == 0 {
		clientIDs = registry.ids
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
	var failures []error
	for _, receipt := range slices.Backward(receipts) {
		if receipt == nil {
			continue
		}
		if err := receipt.Rollback(); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
