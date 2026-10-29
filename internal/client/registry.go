// Package client owns the operational contract and admission-ordered registry for every
// admitted local client. Client-specific formats remain in their own packages;
// shared workflows depend only on this contract.
package client

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/transaction"
)

// DiscoverySource is the bounded, client-neutral host observation surface used
// by adapters: executable and file lookups, and the observed platform from
// which each adapter derives its own native paths. It contains no mutation
// capability.
type DiscoverySource interface {
	Executable(name string) string
	Runnable(path string) bool
	FilePresent(path string) bool
	Platform() discovery.Host
}

// Dependencies are the shared capabilities supplied to one adapter operation.
// Host is the observed platform from which adapters derive their native paths.
type Dependencies struct {
	Secrets                   secrets.Store
	Runner                    process.VerificationRunner
	Discovery                 discovery.Discoverer
	Host                      discovery.Host
	AIGWExecutable            string
	AuthorizedSelectionClient string
}

// ProjectionPlan is a side-effect-free description of one owned projection
// effect. Surface is the identity of the host surface that owns the target.
// Effects lists every file that applying the plan may write or remove: the
// target itself when it is a file, its ownership state and any companion
// files. ChangesState identifies whether applying the plan changes one of them.
type ProjectionPlan struct {
	Client       string   `json:"client"`
	Target       string   `json:"target"`
	Action       string   `json:"action"`
	Surface      string   `json:"-"`
	Effects      []string `json:"-"`
	ChangesState bool     `json:"-"`
}

// ProjectionReceipt can compensate an applied client projection while its
// owned postimage remains unchanged.
type ProjectionReceipt interface {
	Rollback() error
}

// ErrProjectionRollbackFailed means an adapter may still consume its projected
// credential entrypoint after a failed multi-client operation.
var ErrProjectionRollbackFailed = errors.New("client projection rollback failed")

// Status is one adapter's read-only local readiness observation.
type Status struct {
	Ready               bool
	NativeModelOverride bool
	Issue               string
	RepairAction        string
	Checks              []Check
}

// Check is one adapter-owned, read-only diagnostic observation. Target names
// the native file the observation inspected, when it concerns one.
type Check struct {
	ID           string
	Ready        bool
	Detail       string
	RepairAction string
	Target       string
}

// Verification is the non-sensitive result of one explicit live client probe.
type Verification struct {
	Scope   string
	Version string
	SHA256  string
}

// Adapter is the complete operational boundary for one admitted client.
type Adapter interface {
	Spec() domain.ClientSpec
	Discover(source DiscoverySource) discovery.Result
	Converge(deps Dependencies, cfg *domain.Config, discovered discovery.Result) error
	// Plan completely describes Apply's owned projection effects at observation
	// time. Nil or empty plans mean no projection effects; ChangesState covers
	// every effect that would alter the current native state.
	Plan(deps Dependencies, before, after domain.Config) ([]ProjectionPlan, error)
	Apply(ctx context.Context, deps Dependencies, before, after domain.Config) (ProjectionReceipt, error)
	ProjectionChanged(before, after domain.Config) bool
	Inspect(ctx context.Context, deps Dependencies, cfg domain.Config, runtime domain.Runtime) Status
	Verify(ctx context.Context, deps Dependencies, cfg domain.Config, runtime domain.Runtime, prompt string) (Verification, error)
	// AdoptSelection records the client's own selection inside AIGW's projected
	// catalogue as its Client Binding and returns the adopted Route, or an empty
	// string when the client shows its bound selection or adopts nothing.
	AdoptSelection(deps Dependencies, cfg *domain.Config) (string, error)
	Withdraw(cfg *domain.Config)
}

// TargetValidator is implemented by an adapter whose configuration targets the
// operator names explicitly: it refuses a path that is not one of its targets.
type TargetValidator interface {
	ValidateTarget(discovered discovery.Result, path string) error
}

// HostValidator is implemented by an adapter whose native paths every AIGW
// process derives: it refuses a host on which they cannot be derived, and AIGW
// then does not start.
type HostValidator interface {
	ValidateHost(host discovery.Host) error
}

// Registry is the sole ordered operational registry for admitted clients.
type Registry struct {
	byID      map[string]Adapter
	ids       []string
	admission domain.Admission
}

// NewRegistry registers the operational adapters in the given order. Each
// adapter's Spec is its client's admission record, and that order is the
// admission order of every configuration that the registry serves.
func NewRegistry(adapters ...Adapter) (Registry, error) {
	byID := make(map[string]Adapter, len(adapters))
	ids := make([]string, 0, len(adapters))
	specs := make([]domain.ClientSpec, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			return Registry{}, fmt.Errorf("client adapter is nil")
		}
		spec := adapter.Spec()
		if err := spec.Validate(); err != nil {
			return Registry{}, fmt.Errorf("client adapter: %w", err)
		}
		if _, exists := byID[spec.ID]; exists {
			return Registry{}, fmt.Errorf("client adapter %q is registered more than once", spec.ID)
		}
		byID[spec.ID] = adapter
		ids = append(ids, spec.ID)
		specs = append(specs, spec)
	}
	return Registry{byID: byID, ids: ids, admission: domain.NewAdmission(specs...)}, nil
}

// Admission returns the admitted clients' specifications in admission order,
// the client set that every configuration of this program admits.
func (registry Registry) Admission() domain.Admission { return registry.admission }

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
func (registry Registry) Converge(deps Dependencies, before domain.Config, discovered discovery.Result, clients ...string) (domain.Config, error) {
	after := before.Clone()
	adapters, err := registry.selectAdapters(clients)
	if err != nil {
		return domain.Config{}, err
	}
	for _, adapter := range adapters {
		if err := adapter.Converge(deps, &after, discovered); err != nil {
			return domain.Config{}, err
		}
	}
	return after, nil
}

// Plan prepares requested adapters before any projection is written. An empty
// client list selects every admitted adapter.
func (registry Registry) Plan(deps Dependencies, before, after domain.Config, clientIDs ...string) ([]ProjectionPlan, error) {
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

// Apply rechecks current plans and skips converged, unchanged configurations.
// On success it reports native state changes and returns one compensation
// receipt. Earlier adapters are compensated on failure.
func (registry Registry) Apply(ctx context.Context, deps Dependencies, before, after domain.Config, clientIDs ...string) (resultReceipt ProjectionReceipt, projectionChanged bool, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	adapters, err := registry.selectAdapters(clientIDs)
	if err != nil {
		return nil, false, err
	}
	plans, err := registry.Plan(deps, before, after, clientIDs...)
	if err != nil {
		return nil, false, err
	}
	changed := slices.ContainsFunc(plans, func(plan ProjectionPlan) bool { return plan.ChangesState })
	if len(plans) > 0 && reflect.DeepEqual(before, after) && !changed {
		return projectionReceipts{}, false, ctx.Err()
	}
	receipts := make([]ProjectionReceipt, 0, len(adapters))
	defer func() {
		if resultErr == nil {
			return
		}
		if rollbackErr := rollbackReceipts(receipts); rollbackErr != nil {
			resultErr = fmt.Errorf("%w: client projection failed: %w; rollback also failed: %w", ErrProjectionRollbackFailed, resultErr, rollbackErr)
			return
		}
		if errors.Is(resultErr, transaction.ErrCompensationIncomplete) {
			// The failing adapter could not undo its own writes, which may still
			// reference the projected credential reader.
			resultErr = fmt.Errorf("%w: %w", ErrProjectionRollbackFailed, resultErr)
			return
		}
		resultErr = fmt.Errorf("client projection failed and prior adapters were rolled back: %w", resultErr)
	}()
	for _, adapter := range adapters {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		receipt, err := adapter.Apply(ctx, deps, before, after)
		if err != nil {
			return nil, false, err
		}
		receipts = append(receipts, receipt)
	}
	return projectionReceipts(receipts), changed, nil
}

type projectionReceipts []ProjectionReceipt

func (receipts projectionReceipts) Rollback() error { return rollbackReceipts(receipts) }

// ChangedClients returns clients whose persistent projection changes, in admission order.
func (registry Registry) ChangedClients(before, after domain.Config) []string {
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
func (registry Registry) Inspect(ctx context.Context, deps Dependencies, cfg domain.Config, clientID string, runtime domain.Runtime) Status {
	adapter, ok := registry.byID[clientID]
	if !ok {
		return Status{Issue: fmt.Sprintf("client %q has no admitted operational adapter", clientID), RepairAction: "aigw repair"}
	}
	return adapter.Inspect(ctx, deps, cfg, runtime)
}

// Verify runs one explicit live verification through the admitted adapter.
func (registry Registry) Verify(ctx context.Context, deps Dependencies, cfg domain.Config, clientID string, runtime domain.Runtime, explicitRoute string) (Verification, error) {
	adapter, err := registry.adapter(clientID)
	if err != nil {
		return Verification{}, err
	}
	return adapter.Verify(ctx, deps, cfg, runtime, explicitRoute)
}

// Withdraw removes the selected adapter from desired configuration. The
// synchronization transaction then uses the same adapter to remove owned state.
func (registry Registry) Withdraw(cfg *domain.Config, clientID string) error {
	adapter, err := registry.adapter(clientID)
	if err != nil {
		return err
	}
	adapter.Withdraw(cfg)
	return nil
}

// AdoptSelections records each admitted client's own selection as its Client
// Binding in admission order, and returns the adopted Route per client.
func (registry Registry) AdoptSelections(deps Dependencies, cfg *domain.Config) (map[string]string, error) {
	adopted := map[string]string{}
	for _, id := range registry.ids {
		routeID, err := registry.byID[id].AdoptSelection(deps, cfg)
		if err != nil {
			return nil, err
		}
		if routeID != "" {
			adopted[id] = routeID
		}
	}
	return adopted, nil
}

// ValidatesTargets reports whether the client's adapter validates explicit
// configuration targets.
func (registry Registry) ValidatesTargets(clientID string) bool {
	_, validates := registry.byID[clientID].(TargetValidator)
	return validates
}

// ValidateTarget refuses a path that the client's adapter does not accept as
// a configuration target.
func (registry Registry) ValidateTarget(clientID string, discovered discovery.Result, path string) error {
	adapter, err := registry.adapter(clientID)
	if err != nil {
		return err
	}
	if validator, validates := adapter.(TargetValidator); validates {
		return validator.ValidateTarget(discovered, path)
	}
	return nil
}

// ValidateHost refuses a host on which an admitted client cannot derive its
// native paths, with the first refusal in admission order.
func (registry Registry) ValidateHost(host discovery.Host) error {
	for _, id := range registry.ids {
		if validator, validates := registry.byID[id].(HostValidator); validates {
			if err := validator.ValidateHost(host); err != nil {
				return err
			}
		}
	}
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
