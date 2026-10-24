package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	hermesconfig "aigw-cli/internal/hermes/configuration"
	surfaceidentity "aigw-cli/internal/surface"
	"aigw-cli/internal/transaction"
)

type hermesAdapter struct{}

const hermesSurface = string(surfaceidentity.HermesHomeDefault)

func (hermesAdapter) Spec() configuration.ClientSpec {
	return mustClientSpec(configuration.ClientHermes)
}

func (hermesAdapter) Discover(source DiscoverySource) discovery.Result {
	executable := source.Executable(configuration.ClientHermes)
	path := filepath.Join(source.HermesHomeDirectory(), "config.yaml")
	return discovery.Result{
		Executables: map[string]string{configuration.ClientHermes: executable},
		Surfaces:    []discovery.Surface{{ID: hermesSurface, Product: "Hermes", Authority: "aigw", Executable: executable, ConfigPath: path, Present: source.FilePresent(path)}},
	}
}

func (hermesAdapter) Converge(deps Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
	selected, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if _, absent := errors.AsType[*configuration.RuntimeBindingUnselectedError](err); absent {
		return nil
	}
	if err != nil {
		return err
	}
	adapter, explicitlyConfigured := cfg.Clients[configuration.ClientHermes]
	if !explicitlyConfigured || !adapter.Enabled {
		return nil
	}
	executable, err := resolveExecutable(configuration.ClientHermes, adapter.Executable, discovered.Executable(configuration.ClientHermes))
	if err != nil {
		return err
	}
	available, err := discovery.ExecutableAvailable(executable)
	if err != nil {
		return err
	}
	if !available {
		return nil
	}
	credentialAvailable := !selected.UsesAIGWCredentialStore()
	if selected.UsesAIGWCredentialStore() {
		credentialAvailable, err = secretAvailable(deps.Secrets, selected.AccountID)
		if err != nil {
			return err
		}
	}
	if !credentialAvailable {
		return nil
	}
	if len(adapter.Targets) == 0 {
		surface, found := discovered.Surface(hermesSurface)
		if !found || surface.ConfigPath == "" {
			return nil
		}
		adapter.Targets = []string{surface.ConfigPath}
	}
	cfg.SetClientActivation(configuration.ClientHermes, true, executable, adapter.Targets)
	return nil
}

func hermesDesired(deps Dependencies, cfg configuration.Config, selected configuration.Runtime) (hermesconfig.Desired, error) {
	catalogue, err := hermesCatalogue(cfg)
	if err != nil {
		return hermesconfig.Desired{}, err
	}
	connected := map[string]bool{selected.AccountID: true}
	desired := hermesconfig.Desired{SelectedProvider: hermesProviderID(selected.AccountID, selected.Protocol), SelectedModel: selected.Model}
	for _, item := range catalogue {
		if _, observed := connected[item.Account]; !observed {
			available, err := secretAvailable(deps.Secrets, item.Account)
			if err != nil {
				return hermesconfig.Desired{}, fmt.Errorf("inspect Hermes Account %q credential: %w", item.Account, err)
			}
			connected[item.Account] = available
		}
		if !connected[item.Account] {
			continue
		}
		command, err := credential.Command(item.Runtime.CredentialExecutable(deps.AIGWExecutable), configuration.ClientHermes, item.Runtime.CredentialProjectionFingerprint(configuration.ClientHermes), runtime.GOOS)
		if err != nil {
			return hermesconfig.Desired{}, err
		}
		desired.Providers = append(desired.Providers, hermesconfig.Provider{
			ID: item.ID, Name: item.Name, Endpoint: item.Endpoint, Protocol: string(item.Protocol), CredentialCommand: command, Models: item.Models,
		})
	}
	if err := desired.Validate(); err != nil {
		return hermesconfig.Desired{}, err
	}
	return desired, nil
}

type hermesCatalogueProvider struct {
	ID       string
	Name     string
	Account  string
	Protocol configuration.EndpointProtocol
	Endpoint string
	Runtime  configuration.Runtime
	Models   []string
}

func hermesCatalogue(cfg configuration.Config) ([]hermesCatalogueProvider, error) {
	spec := mustClientSpec(configuration.ClientHermes)
	providers := make(map[string]hermesCatalogueProvider)
	var ids []string
	for _, routeID := range cfg.CatalogRouteIDs() {
		route := cfg.Routes[routeID]
		account := cfg.Accounts[route.Account]
		for _, protocol := range spec.CompatibleRouteProtocols(account, route) {
			id := hermesProviderID(route.Account, protocol)
			item, present := providers[id]
			if !present {
				ids = append(ids, id)
				selected, err := cfg.ResolveRouteProtocol(configuration.ClientHermes, routeID, protocol)
				if err != nil {
					return nil, err
				}
				item = hermesCatalogueProvider{ID: id, Name: account.Label + " · " + configuration.ProtocolLabel(protocol), Account: route.Account, Protocol: protocol, Endpoint: selected.Endpoint, Runtime: selected}
			}
			if !slices.Contains(item.Models, route.UpstreamModelID()) {
				item.Models = append(item.Models, route.UpstreamModelID())
			}
			providers[id] = item
		}
	}
	result := make([]hermesCatalogueProvider, 0, len(providers))
	for _, id := range ids {
		result = append(result, providers[id])
	}
	slices.SortFunc(result, func(left, right hermesCatalogueProvider) int {
		if order := cfg.CompareAccounts(left.Account, right.Account); order != 0 {
			return order
		}
		return configuration.CompareProtocols(left.Protocol, right.Protocol)
	})
	return result, nil
}

func hermesProviderID(accountID string, protocol configuration.EndpointProtocol) string {
	return "aigw-" + accountID + "-" + strings.ReplaceAll(string(protocol), "_", "-")
}

func hermesPlans(deps Dependencies, before, after configuration.Config) ([]hermesconfig.Plan, []string, error) {
	previous, current := before.Clients[configuration.ClientHermes], after.Clients[configuration.ClientHermes]
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired hermesconfig.Desired
	if current.Enabled && len(current.Targets) > 0 {
		if len(current.Targets) != 1 {
			return nil, nil, errors.New("hermes requires one configured home")
		}
		selected, err := after.ResolveRuntime(configuration.ClientHermes, "")
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
	plans := make([]hermesconfig.Plan, 0, len(targets))
	for _, target := range targets {
		var projection *hermesconfig.Desired
		if current.Enabled && len(current.Targets) > 0 && slices.Contains(current.Targets, target) {
			projection = &desired
		}
		plan, err := hermesconfig.Prepare(target, projection)
		if err != nil && projection != nil {
			authorized := deps.AuthorizedSelectionClient == configuration.ClientHermes
			previous, previousErr := hermesPreviousDesired(deps, before)
			switch {
			case previousErr != nil && authorized:
				return nil, nil, previousErr
			case previousErr != nil:
				// Without a previous projection there is no proved selection to adopt.
			case authorized:
				plan, err = hermesconfig.PrepareAuthorizedSelection(target, *projection, previous)
			default:
				plan, err = hermesconfig.PrepareAdoptedSelection(target, *projection, previous)
			}
		}
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	return plans, targets, nil
}

func hermesPreviousDesired(deps Dependencies, before configuration.Config) (hermesconfig.Desired, error) {
	selected, err := before.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		return hermesconfig.Desired{}, err
	}
	return hermesDesired(deps, before, selected)
}

// AdoptHermesSelection records Hermes' own model choice as its Client Binding
// when the operator selected another model of AIGW's projected catalogue in
// Hermes and nothing else that AIGW owns changed. It returns the adopted Route,
// or an empty string when Hermes still shows the bound selection.
func AdoptHermesSelection(deps Dependencies, cfg *configuration.Config) (string, error) {
	binding := cfg.Clients[configuration.ClientHermes]
	if !binding.Enabled || len(binding.Targets) != 1 {
		return "", nil
	}
	selected, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		return "", nil
	}
	current, err := hermesDesired(deps, *cfg, selected)
	if err != nil {
		return "", err
	}
	native, changed, err := hermesconfig.ObserveSelection(binding.Targets[0], current)
	if err != nil || !changed {
		return "", err
	}
	routeID, protocol, err := hermesRouteFor(*cfg, native)
	if err != nil {
		return "", err
	}
	if routeID == binding.Route && protocol == selected.Protocol {
		// The binding already names the native choice; only the record is stale.
		return "", nil
	}
	cfg.SetSelectedRoute(configuration.ClientHermes, routeID, protocol)
	return routeID, nil
}

// hermesRouteFor maps a projected provider and model to the one catalogue
// Route with that provider's Account, protocol and upstream model.
func hermesRouteFor(cfg configuration.Config, native hermesconfig.Selection) (string, configuration.EndpointProtocol, error) {
	catalogue, err := hermesCatalogue(cfg)
	if err != nil {
		return "", "", err
	}
	index := slices.IndexFunc(catalogue, func(item hermesCatalogueProvider) bool { return item.ID == native.Provider })
	if index < 0 {
		return "", "", fmt.Errorf("Hermes selects provider %q, which AIGW no longer projects; run `aigw use --for hermes <route>`", native.Provider)
	}
	provider := catalogue[index]
	spec := mustClientSpec(configuration.ClientHermes)
	var matches []string
	for _, routeID := range cfg.CatalogRouteIDs() {
		route := cfg.Routes[routeID]
		if route.Account != provider.Account || route.UpstreamModelID() != native.Model {
			continue
		}
		if slices.Contains(spec.CompatibleRouteProtocols(cfg.Accounts[route.Account], route), provider.Protocol) {
			matches = append(matches, routeID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], provider.Protocol, nil
	case 0:
		return "", "", fmt.Errorf("Hermes selects model %q, which no AIGW Route provides; run `aigw use --for hermes <route>`", native.Model)
	default:
		commands := make([]string, 0, len(matches))
		for _, routeID := range matches {
			commands = append(commands, fmt.Sprintf("`aigw use --for hermes --protocol %s %s`", provider.Protocol, routeID))
		}
		return "", "", fmt.Errorf("Hermes selects model %q, which several AIGW Routes provide; run one of %s", native.Model, strings.Join(commands, ", "))
	}
}

func (hermesAdapter) Plan(deps Dependencies, before, after configuration.Config) ([]ProjectionPlan, error) {
	plans, targets, err := hermesPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectionPlan, 0, len(plans))
	for index, plan := range plans {
		result = append(result, ProjectionPlan{Client: configuration.ClientHermes, Target: targets[index], Action: plan.Action, Surface: hermesSurface, ChangesState: plan.ChangesState()})
	}
	return result, nil
}

type hermesReceipt []hermesconfig.Receipt

func (receipt hermesReceipt) Rollback() error {
	var result error
	for _, item := range slices.Backward(receipt) {
		result = errors.Join(result, item.Rollback())
	}
	return result
}

func (hermesAdapter) Apply(_ context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	plans, _, err := hermesPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	receipts := hermesReceipt{}
	for _, plan := range plans {
		receipt, err := plan.Apply()
		if err != nil {
			return nil, transaction.CompensationFailed(err, receipts.Rollback())
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (hermesAdapter) ProjectionChanged(before, after configuration.Config) bool {
	previous, current := before.Clients[configuration.ClientHermes], after.Clients[configuration.ClientHermes]
	if previous.Enabled != current.Enabled {
		return true
	}
	if !current.Enabled {
		return false
	}
	if previous.CredentialCommand != current.CredentialCommand || !slices.Equal(previous.Targets, current.Targets) {
		return true
	}
	left, leftErr := before.ResolveRuntime(configuration.ClientHermes, "")
	right, rightErr := after.ResolveRuntime(configuration.ClientHermes, "")
	if leftErr != nil || rightErr != nil || left != right {
		return true
	}
	oldCatalogue, oldErr := hermesCatalogue(before)
	newCatalogue, newErr := hermesCatalogue(after)
	if oldErr != nil || newErr != nil {
		return true
	}
	return !slices.EqualFunc(oldCatalogue, newCatalogue, func(left, right hermesCatalogueProvider) bool {
		return left.ID == right.ID && left.Name == right.Name && left.Account == right.Account && left.Protocol == right.Protocol && left.Endpoint == right.Endpoint && slices.Equal(left.Models, right.Models)
	})
}

func (hermesAdapter) Inspect(ctx context.Context, deps Dependencies, cfg configuration.Config, selected configuration.Runtime) Status {
	if err := ctx.Err(); err != nil {
		return Status{Issue: err.Error()}
	}
	adapter := cfg.Clients[configuration.ClientHermes]
	available, err := discovery.ExecutableAvailable(adapter.Executable)
	if err != nil || !adapter.Enabled || !available || len(adapter.Targets) != 1 {
		return Status{Issue: "Hermes executable or configuration home is unavailable", RepairAction: "aigw sync"}
	}
	plan, err := hermesInspectionPlan(deps, cfg, selected, adapter.Targets[0])
	if err != nil {
		return Status{Issue: "Hermes configuration projection cannot be inspected", RepairAction: "aigw sync"}
	}
	if plan.Action != "unchanged" {
		return Status{Issue: "Hermes configuration projection differs from the selected Route", RepairAction: "aigw sync"}
	}
	return Status{Ready: true}
}

func hermesInspectionPlan(deps Dependencies, cfg configuration.Config, selected configuration.Runtime, target string) (hermesconfig.Plan, error) {
	desired, err := hermesDesired(deps, cfg, selected)
	if err != nil {
		return hermesconfig.Plan{}, err
	}
	plan, err := hermesconfig.Prepare(target, &desired)
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
		return hermesconfig.Plan{}, err
	}
	return hermesconfig.Prepare(target, &desired)
}

func hermesRetainedReader(cfg configuration.Config, desired hermesconfig.Desired, plan hermesconfig.Plan, current string) (string, error) {
	return retainedDefaultReader(current, false, func() (string, error) {
		catalogue, err := hermesCatalogue(cfg)
		if err != nil {
			return "", err
		}
		var observedReader string
		for _, item := range catalogue {
			if !slices.ContainsFunc(desired.Providers, func(provider hermesconfig.Provider) bool { return provider.ID == item.ID }) {
				continue
			}
			command, ok := plan.ObservedCredentialCommand(item.ID)
			if !ok {
				return "", errors.New("managed Hermes credential command is missing")
			}
			reader, err := credential.ExecutableFromCommand(command, configuration.ClientHermes, item.Runtime.CredentialProjectionFingerprint(configuration.ClientHermes), runtime.GOOS)
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

func (hermesAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, configuration.ClientHermes)
}

func (adapter hermesAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, selected configuration.Runtime, _ string) (Verification, error) {
	if deps.Runner == nil {
		return Verification{}, errors.New("hermes verification requires a process runner")
	}
	configured := cfg.Clients[configuration.ClientHermes]
	if !configured.Enabled {
		return Verification{}, errors.New("hermes adapter is disabled; run aigw sync")
	}
	if len(configured.Targets) != 1 {
		return Verification{}, errors.New("hermes verification requires one configured home; run aigw sync")
	}
	plan, err := hermesInspectionPlan(deps, cfg, selected, configured.Targets[0])
	if err != nil {
		return Verification{}, err
	}
	data, err := plan.PrepareVerification()
	if err != nil {
		return Verification{}, err
	}
	return verifyIsolated(ctx, deps, configured, data)
}
