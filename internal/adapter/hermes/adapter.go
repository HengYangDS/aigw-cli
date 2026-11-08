package hermes

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	surfaceidentity "aigw-cli/internal/adapter/surface"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
)

// ID identifies the independently configured Hermes Agent.
const ID = "hermes"

// hermesAdapter is the Hermes client adapter behind the client seam.
type hermesAdapter struct{}

// New returns the Hermes adapter for the composition root to register.
func New() adapter.Adapter { return hermesAdapter{} }

// HomeSurface identifies the configuration of the Hermes home.
const HomeSurface surfaceidentity.ID = "hermes-home-default"

const hermesSurface = string(HomeSurface)

func (hermesAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{
		ID:    ID,
		Label: "Hermes",
		EndpointProtocols: []domain.EndpointProtocol{
			domain.ProtocolOpenAIResponses, domain.ProtocolAnthropic, domain.ProtocolOpenAIChatCompletions,
		},
		SessionsKeepModel: true,
		OptionalTarget:    "config.yaml",
	}
}

func (hermesAdapter) Discover(source adapter.DiscoverySource) discovery.Result {
	executable := source.Executable(ID)
	path := filepath.Join(homeDirectory(source.Platform()), "config.yaml")
	return discovery.Result{
		Executables: map[string]string{ID: executable},
		Surfaces:    []discovery.Surface{{ID: hermesSurface, Executable: executable, ConfigPath: path, Present: source.FilePresent(path)}},
	}
}

// homeDirectory resolves the native Hermes configuration root without creating it.
func homeDirectory(host discovery.Host) string {
	if home := host.Getenv("HERMES_HOME"); home != "" {
		return home
	}
	return filepath.Join(host.Home, ".hermes")
}

func (hermesAdapter) Converge(deps adapter.Dependencies, cfg *domain.Config, discovered discovery.Result) error {
	// Hermes activates only a runnable executable, which it checks before it
	// reads the secret store.
	binding, executable, ready, err := adapter.ConvergeBinding(deps, cfg, discovered, ID, true)
	if err != nil || !ready {
		return err
	}
	if len(binding.Targets) == 0 {
		surface, found := discovered.Surface(hermesSurface)
		if !found || surface.ConfigPath == "" {
			return nil
		}
		binding.Targets = []string{surface.ConfigPath}
	}
	cfg.SetClientActivation(ID, true, executable, binding.Targets)
	return nil
}

func hermesDesired(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime) (Desired, error) {
	catalog, err := hermesCatalog(cfg)
	if err != nil {
		return Desired{}, err
	}
	connected := map[string]bool{selected.ProviderID: true}
	desired := Desired{SelectedProvider: hermesProviderID(selected.ProviderID, selected.Protocol), SelectedModel: selected.UpstreamModel}
	for _, item := range catalog {
		if _, observed := connected[item.Provider]; !observed {
			available, err := adapter.SecretAvailable(deps.Secrets, item.Provider)
			if err != nil {
				return Desired{}, fmt.Errorf("inspect Hermes Account %q credential: %w", item.Provider, err)
			}
			connected[item.Provider] = available
		}
		if !connected[item.Provider] {
			continue
		}
		command, err := secret.Command(item.Runtime.CredentialExecutable(deps.AIGWExecutable), ID, item.Runtime.CredentialProjectionFingerprint(ID), runtime.GOOS)
		if err != nil {
			return Desired{}, err
		}
		desired.Providers = append(desired.Providers, Provider{
			ID: item.ID, Name: item.Name, Endpoint: item.Endpoint, Protocol: string(item.Protocol), CredentialCommand: command, Models: item.Models,
		})
	}
	if err := desired.Validate(); err != nil {
		return Desired{}, err
	}
	return desired, nil
}

type hermesCatalogProvider struct {
	ID       string
	Name     string
	Provider string
	Protocol domain.EndpointProtocol
	Endpoint string
	Runtime  domain.Runtime
	Models   []string
}

func hermesCatalog(cfg domain.Config) ([]hermesCatalogProvider, error) {
	spec := hermesAdapter{}.Spec()
	providers := make(map[string]hermesCatalogProvider)
	var ids []string
	for _, routeID := range cfg.CatalogModelIDs() {
		route := cfg.Models[routeID]
		account := cfg.Providers[route.Provider]
		for _, protocol := range spec.CompatibleModelProtocols(account, route) {
			id := hermesProviderID(route.Provider, protocol)
			item, present := providers[id]
			if !present {
				ids = append(ids, id)
				selected, err := cfg.ResolveModelProtocol(ID, routeID, protocol)
				if err != nil {
					return nil, err
				}
				item = hermesCatalogProvider{ID: id, Name: account.Label + " · " + domain.ProtocolLabel(protocol), Provider: route.Provider, Protocol: protocol, Endpoint: selected.Endpoint, Runtime: selected}
			}
			if !slices.Contains(item.Models, route.UpstreamModelID()) {
				item.Models = append(item.Models, route.UpstreamModelID())
			}
			providers[id] = item
		}
	}
	result := make([]hermesCatalogProvider, 0, len(providers))
	for _, id := range ids {
		result = append(result, providers[id])
	}
	slices.SortFunc(result, func(left, right hermesCatalogProvider) int {
		if order := cfg.CompareProviders(left.Provider, right.Provider); order != 0 {
			return order
		}
		return domain.CompareProtocols(left.Protocol, right.Protocol)
	})
	return result, nil
}

func hermesProviderID(accountID string, protocol domain.EndpointProtocol) string {
	return "aigw-" + accountID + "-" + strings.ReplaceAll(string(protocol), "_", "-")
}

func hermesPlans(deps adapter.Dependencies, before, after domain.Config) ([]Plan, []string, error) {
	previous, current := before.Clients[ID], after.Clients[ID]
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired Desired
	if current.Enabled && len(current.Targets) > 0 {
		if len(current.Targets) != 1 {
			return nil, nil, errors.New("hermes requires one configured home")
		}
		selected, err := after.ResolveRuntime(ID, "")
		if err != nil {
			return nil, nil, err
		}
		desired, err = hermesDesired(deps, after, selected)
		if err != nil {
			return nil, nil, err
		}
	}
	targets := slices.Clone(current.Targets)
	for _, target := range previous.Targets {
		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
		var projection *Desired
		if current.Enabled && len(current.Targets) > 0 && slices.Contains(current.Targets, target) {
			projection = &desired
		}
		plan, err := Prepare(target, projection)
		if err != nil && projection != nil {
			authorized := deps.AuthorizedSelectionClient == ID
			previous, previousErr := hermesPreviousDesired(deps, before)
			switch {
			case previousErr != nil && authorized:
				return nil, nil, previousErr
			case previousErr != nil:
				// Without a previous projection there is no proved selection to adopt.
			case authorized:
				plan, err = PrepareAuthorizedSelection(target, *projection, previous)
			default:
				plan, err = PrepareAdoptedSelection(target, *projection, previous)
			}
		}
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	return plans, targets, nil
}

func hermesPreviousDesired(deps adapter.Dependencies, before domain.Config) (Desired, error) {
	selected, err := before.ResolveRuntime(ID, "")
	if err != nil {
		return Desired{}, err
	}
	return hermesDesired(deps, before, selected)
}

// AdoptSelection records Hermes' own model choice as its Client Binding
// when the operator selected another model of AIGW's projected catalog in
// Hermes and nothing else that AIGW owns changed. It returns the adopted Route,
// or an empty string when Hermes still shows the bound selection.
func (hermesAdapter) AdoptSelection(deps adapter.Dependencies, cfg *domain.Config) (string, error) {
	binding := cfg.Clients[ID]
	if !binding.Enabled || len(binding.Targets) != 1 {
		return "", nil
	}
	selected, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		return "", nil
	}
	current, err := hermesDesired(deps, *cfg, selected)
	if err != nil {
		return "", err
	}
	native, changed, err := ObserveSelection(binding.Targets[0], current)
	if err != nil || !changed {
		return "", err
	}
	routeID, protocol, err := hermesModelFor(*cfg, native)
	if err != nil {
		return "", err
	}
	if routeID == binding.Model && protocol == selected.Protocol {
		// The binding already names the native choice; only the record is stale.
		return "", nil
	}
	cfg.SetSelectedModel(ID, routeID, protocol)
	return routeID, nil
}

// hermesModelFor maps a projected provider and model to the one catalog
// Route with that provider's Account, protocol and upstream model.
func hermesModelFor(cfg domain.Config, native Selection) (string, domain.EndpointProtocol, error) {
	catalog, err := hermesCatalog(cfg)
	if err != nil {
		return "", "", err
	}
	index := slices.IndexFunc(catalog, func(item hermesCatalogProvider) bool { return item.ID == native.Provider })
	if index < 0 {
		return "", "", fmt.Errorf("Hermes selects provider %q, which AIGW no longer projects; run `aigw use hermes <provider/model>`", native.Provider)
	}
	provider := catalog[index]
	spec := hermesAdapter{}.Spec()
	var matches []string
	for _, routeID := range cfg.CatalogModelIDs() {
		route := cfg.Models[routeID]
		if route.Provider != provider.Provider || route.UpstreamModelID() != native.Model {
			continue
		}
		if slices.Contains(spec.CompatibleModelProtocols(cfg.Providers[route.Provider], route), provider.Protocol) {
			matches = append(matches, routeID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], provider.Protocol, nil
	case 0:
		return "", "", fmt.Errorf("Hermes selects model %q, which provider %q has not added; run `aigw use hermes %s/%s --protocol %s`",
			native.Model, provider.Provider, provider.Provider, native.Model, provider.Protocol.Name())
	default:
		// Only a configuration from before models were unique per provider
		// can hold several, which no selection tells apart.
		return "", "", fmt.Errorf("Hermes selects model %q, which %d models of provider %q carry; correct the configuration, whose file `aigw config path` names", native.Model, len(matches), provider.Provider)
	}
}

func (hermesAdapter) Plan(deps adapter.Dependencies, before, after domain.Config) ([]adapter.ProjectionPlan, error) {
	plans, targets, err := hermesPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	result := make([]adapter.ProjectionPlan, 0, len(plans))
	for index, plan := range plans {
		result = append(result, adapter.ProjectionPlan{Client: ID, Target: targets[index], Action: plan.Action, Surface: hermesSurface, Effects: plan.Effects(), ChangesState: plan.ChangesState()})
	}
	return result, nil
}

func (hermesAdapter) Apply(_ context.Context, deps adapter.Dependencies, before, after domain.Config) (adapter.ProjectionReceipt, error) {
	plans, _, err := hermesPlans(deps, before, after)
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

func (hermesAdapter) ProjectionChanged(before, after domain.Config) bool {
	previous, current := before.Clients[ID], after.Clients[ID]
	if previous.Enabled != current.Enabled {
		return true
	}
	if !current.Enabled {
		return false
	}
	if previous.CredentialCommand != current.CredentialCommand || !slices.Equal(previous.Targets, current.Targets) {
		return true
	}
	left, leftErr := before.ResolveRuntime(ID, "")
	right, rightErr := after.ResolveRuntime(ID, "")
	if leftErr != nil || rightErr != nil || left != right {
		return true
	}
	oldCatalog, oldErr := hermesCatalog(before)
	newCatalog, newErr := hermesCatalog(after)
	if oldErr != nil || newErr != nil {
		return true
	}
	return !slices.EqualFunc(oldCatalog, newCatalog, func(left, right hermesCatalogProvider) bool {
		return left.ID == right.ID && left.Name == right.Name && left.Provider == right.Provider && left.Protocol == right.Protocol && left.Endpoint == right.Endpoint && slices.Equal(left.Models, right.Models)
	})
}

func (hermesAdapter) Inspect(ctx context.Context, deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime) adapter.Status {
	if err := ctx.Err(); err != nil {
		return adapter.Status{Issue: err.Error()}
	}
	clientAdapter := cfg.Clients[ID]
	available, err := discovery.ExecutableAvailable(clientAdapter.Executable)
	if err != nil || !clientAdapter.Enabled || !available || len(clientAdapter.Targets) != 1 {
		return adapter.Status{Issue: "Hermes executable or configuration home is unavailable", RepairAction: "aigw sync"}
	}
	plan, err := hermesInspectionPlan(deps, cfg, selected, clientAdapter.Targets[0])
	if err != nil {
		return uninspectable(deps, cfg, selected, clientAdapter.Targets[0], err)
	}
	if plan.Action != "unchanged" {
		return adapter.Status{Issue: "Hermes configuration projection differs from the selected Route", RepairAction: "aigw sync"}
	}
	return adapter.Status{Ready: true}
}

// uninspectable is the status of a Hermes file that inspection refused with
// err. A refusal that synchronization shares names the action that resolves
// it; inspection also refuses a model chosen in Hermes itself, which
// synchronization adopts.
func uninspectable(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime, target string, err error) adapter.Status {
	if status, refused := adapter.ConflictStatus(err); refused && !adoptsSelection(deps, cfg, selected, target) {
		return status
	}
	return adapter.Status{Issue: "Hermes configuration projection cannot be inspected", RepairAction: "aigw sync"}
}

// adoptsSelection reports whether synchronization would adopt the model that
// Hermes' file selects, so the file's only difference is the operator's choice.
func adoptsSelection(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime, target string) bool {
	desired, err := hermesDesired(deps, cfg, selected)
	if err != nil {
		return false
	}
	_, adopted, err := ObserveSelection(target, desired)
	return err == nil && adopted
}

func hermesInspectionPlan(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime, target string) (Plan, error) {
	desired, err := hermesDesired(deps, cfg, selected)
	if err != nil {
		return Plan{}, err
	}
	plan, err := Prepare(target, &desired)
	if err != nil || plan.Action == "unchanged" || !selected.UsesAIGWCredentialStore() {
		return plan, err
	}
	if _, owned := plan.ObservedCredentialCommand(desired.SelectedProvider); !owned {
		return plan, nil
	}
	reader, err := hermesRetainedReader(cfg, desired, plan, deps.AIGWExecutable)
	if err != nil || reader == deps.AIGWExecutable {
		return plan, err
	}
	retained := deps
	retained.AIGWExecutable = reader
	desired, err = hermesDesired(retained, cfg, selected)
	if err != nil {
		return Plan{}, err
	}
	return Prepare(target, &desired)
}

func hermesRetainedReader(cfg domain.Config, desired Desired, plan Plan, current string) (string, error) {
	return adapter.RetainedDefaultReader(current, false, func() (string, error) {
		catalog, err := hermesCatalog(cfg)
		if err != nil {
			return "", err
		}
		var observedReader string
		for _, item := range catalog {
			if !slices.ContainsFunc(desired.Providers, func(provider Provider) bool { return provider.ID == item.ID }) {
				continue
			}
			command, ok := plan.ObservedCredentialCommand(item.ID)
			if !ok {
				return "", errors.New("managed Hermes credential command is missing")
			}
			reader, err := secret.ExecutableFromCommand(command, ID, item.Runtime.CredentialProjectionFingerprint(ID), runtime.GOOS)
			if err != nil {
				return "", err
			}
			if observedReader != "" && observedReader != reader {
				return "", errors.New("managed Hermes providers use different credential readers")
			}
			observedReader = reader
		}
		return observedReader, nil
	})
}

func (hermesAdapter) Withdraw(cfg *domain.Config) {
	delete(cfg.Clients, ID)
}

func (clientAdapter hermesAdapter) Verify(ctx context.Context, deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime, _ string) (adapter.Verification, error) {
	if deps.Runner == nil {
		return adapter.Verification{}, errors.New("hermes verification requires a process runner")
	}
	configured := cfg.Clients[ID]
	if !configured.Enabled {
		return adapter.Verification{}, errors.New("hermes adapter is disabled; run aigw sync")
	}
	if len(configured.Targets) != 1 {
		return adapter.Verification{}, errors.New("hermes verification requires one configured home; run aigw sync")
	}
	plan, err := hermesInspectionPlan(deps, cfg, selected, configured.Targets[0])
	if err != nil {
		return adapter.Verification{}, err
	}
	data, err := plan.PrepareVerification()
	if err != nil {
		return adapter.Verification{}, err
	}
	return verifyIsolated(ctx, deps, configured, data)
}
