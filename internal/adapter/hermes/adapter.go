package hermes

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

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
			domain.ProtocolResponses, domain.ProtocolAnthropic, domain.ProtocolChat,
		},
		SessionsKeepModel: true,
		OptionalTarget:    "config.yaml",
		ContextWindow:     true,
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
	binding, executable, ready, err := adapter.ConvergeBinding(deps, cfg, discovered, ID)
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
		// A provider whose Token AIGW cannot use, or whose Token the store
		// cannot observe, is left out: its credential command would return no
		// Token.
		if _, observed := connected[item.Provider]; !observed {
			connected[item.Provider] = secret.ObserveToken(deps.Secrets, item.Provider).Usable()
		}
		if !connected[item.Provider] {
			continue
		}
		command, err := secret.Command(item.Runtime.CredentialExecutable(deps.AIGWExecutable), ID, item.Runtime.CredentialProjectionFingerprint(ID))
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
	Models   []Model
}

// hermesCatalog groups the models that Hermes can use under one Hermes
// provider per provider and protocol, in configuration order and then the
// protocols' order.
func hermesCatalog(cfg domain.Config) ([]hermesCatalogProvider, error) {
	spec := hermesAdapter{}.Spec()
	var result []hermesCatalogProvider
	for _, providerID := range cfg.ProviderIDs() {
		provider := cfg.Providers[providerID]
		var items []hermesCatalogProvider
		for _, ref := range cfg.ProviderModelRefs(providerID) {
			model := cfg.Models[ref]
			for _, protocol := range spec.CompatibleModelProtocols(provider, model) {
				index := slices.IndexFunc(items, func(item hermesCatalogProvider) bool { return item.Protocol == protocol })
				if index < 0 {
					selected, err := cfg.ResolveModelProtocol(ID, ref, protocol)
					if err != nil {
						return nil, err
					}
					items = append(items, hermesCatalogProvider{ID: hermesProviderID(providerID, protocol), Name: provider.Label + " · " + domain.ProtocolLabel(protocol),
						Provider: providerID, Protocol: protocol, Endpoint: selected.Endpoint, Runtime: selected})
					index = len(items) - 1
				}
				items[index].Models = append(items[index].Models, Model{ID: model.ID, ContextWindow: model.Window()})
			}
		}
		slices.SortStableFunc(items, func(left, right hermesCatalogProvider) int {
			return domain.CompareProtocols(left.Protocol, right.Protocol)
		})
		result = append(result, items...)
	}
	return result, nil
}

// hermesProviderID joins aigw, the provider ID and the protocol name, so that
// each provider gets one Hermes provider per protocol.
func hermesProviderID(providerID string, protocol domain.EndpointProtocol) string {
	return "aigw-" + providerID + "-" + protocol.Name()
}

func hermesPlans(deps adapter.Dependencies, before, after domain.Config) ([]Plan, []string, error) {
	previous, current := before.Clients[ID], after.Clients[ID]
	dropped := platform.DroppedFiles(previous.Targets, current.Targets)
	// A withdraw-only client restores the files that the transition drops and
	// writes none that it keeps, whose selection waits for its Token.
	if slices.Contains(deps.WithdrawOnly, ID) {
		current.Targets = nil
	}
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired Desired
	if current.Enabled && len(current.Targets) > 0 {
		if len(current.Targets) != 1 {
			return nil, nil, errors.New("hermes requires one configured home")
		}
		selected, err := after.ResolveRuntime(ID, domain.ModelRef{})
		if err != nil {
			return nil, nil, err
		}
		desired, err = hermesDesired(deps, after, selected)
		if err != nil {
			return nil, nil, err
		}
	}
	written := hermesListing(before)
	// The files that the configuration after the transition names come first:
	// they alone are written, while the client is enabled. A file that the
	// transition drops, however either configuration spells it, is restored.
	targets := slices.Concat(current.Targets, dropped)
	// The selection that the transition starts from: a file that shows it
	// holds Hermes' own choice, which the configuration follows.
	var shown *Selection
	if selected, err := before.ResolveRuntime(ID, domain.ModelRef{}); err == nil {
		shown = &Selection{Provider: hermesProviderID(selected.ProviderID, selected.Protocol), Model: selected.UpstreamModel}
	}
	plans := make([]Plan, 0, len(targets))
	for index, target := range targets {
		var projection *Desired
		if current.Enabled && index < len(current.Targets) {
			projection = &desired
		}
		plan, err := PrepareTransition(target, projection, shown, deps.AuthorizedSelectionClient == ID)
		if err == nil && projection != nil {
			err = plan.refuseReorderedModels(written)
		}
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	return plans, targets, nil
}

// hermesListing is what AIGW wrote into Hermes's file for cfg, the
// configuration before a change: each provider entry that cfg projects, with
// its models in AIGW's order, whatever Tokens are stored, and nothing where cfg
// projects no Hermes.
func hermesListing(cfg domain.Config) []Provider {
	if !cfg.Clients[ID].Enabled {
		return nil
	}
	catalog, err := hermesCatalog(cfg)
	if err != nil {
		return nil
	}
	providers := make([]Provider, 0, len(catalog))
	for _, item := range catalog {
		providers = append(providers, Provider{ID: item.ID, Models: item.Models})
	}
	return providers
}

// AdoptSelection records Hermes' own model choice as its Client Binding
// when the operator selected another listed model of a provider entry that
// AIGW wrote and nothing else that AIGW owns changed. It returns the adopted
// model, or the zero reference when Hermes shows the bound selection. A
// selection that AIGW cannot follow is refused with the step that restores
// the recorded selection. It reads Hermes' file and AIGW's record of it, and
// no Token or credential command.
func (hermesAdapter) AdoptSelection(_ adapter.Dependencies, cfg *domain.Config) (domain.ModelRef, error) {
	binding := cfg.Clients[ID]
	if !binding.Enabled || len(binding.Targets) != 1 {
		return domain.ModelRef{}, nil
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return domain.ModelRef{}, nil
	}
	recorded := adapter.Recorded(hermesAdapter{}.Spec(), selected)
	native, changed, err := ObserveSelection(binding.Targets[0])
	if unfollowed, refused := errors.AsType[*unfollowedSelectionError](err); refused {
		return domain.ModelRef{}, hermesChoiceOf(*cfg, unfollowed.Selection).refusal(recorded, unfollowed.Selection)
	}
	if err != nil || !changed {
		return domain.ModelRef{}, err
	}
	choice := hermesChoiceOf(*cfg, native)
	if !choice.listed {
		return domain.ModelRef{}, choice.refusal(recorded, native)
	}
	if choice.ref == binding.Model && choice.protocol == selected.Protocol {
		// The binding already names the native choice; only the record is stale.
		return domain.ModelRef{}, nil
	}
	cfg.SetSelectedModel(ID, choice.ref, choice.protocol)
	return choice.ref, nil
}

// hermesChoice is what a selection in Hermes' file names in a configuration:
// the provider and protocol of its provider entry, when AIGW writes that
// entry, and whether the entry lists the selected model.
type hermesChoice struct {
	ref      domain.ModelRef
	protocol domain.EndpointProtocol
	written  bool
	listed   bool
}

// hermesChoiceOf maps a provider entry and model to the model of that entry's
// provider with that ID, on the entry's protocol.
func hermesChoiceOf(cfg domain.Config, native Selection) hermesChoice {
	catalog, err := hermesCatalog(cfg)
	if err != nil {
		return hermesChoice{}
	}
	index := slices.IndexFunc(catalog, func(item hermesCatalogProvider) bool { return item.ID == native.Provider })
	if index < 0 {
		return hermesChoice{}
	}
	entry := catalog[index]
	return hermesChoice{
		ref: domain.NewModelRef(entry.Provider, native.Model), protocol: entry.Protocol,
		written: true, listed: listsModel(entry.Models, native.Model),
	}
}

// refusal refuses a selection that AIGW does not follow, with the step that
// restores recorded. It names the command that adds and selects the model
// only for a provider entry that AIGW writes.
func (choice hermesChoice) refusal(recorded adapter.RecordedSelection, native Selection) *adapter.SelectionError {
	switch {
	case native.Provider == "":
		return adapter.RefusedSelection(recorded, "selects no model of a provider that AIGW wrote", "")
	case !choice.written:
		return adapter.RefusedSelection(recorded, fmt.Sprintf("selects provider %q, which AIGW does not write", native.Provider), "")
	}
	return adapter.RefusedSelection(recorded,
		fmt.Sprintf("selects model %q, which provider %q has not added on %s", native.Model, choice.ref.Provider(), domain.ProtocolLabel(choice.protocol)),
		adapter.SelectCommand(ID, choice.ref, choice.protocol.Name()))
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
	left, leftErr := before.ResolveRuntime(ID, domain.ModelRef{})
	right, rightErr := after.ResolveRuntime(ID, domain.ModelRef{})
	if leftErr != nil || rightErr != nil || !left.Equal(right) {
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
	// A configured executable that is missing is the registry's to report,
	// with its own fix, before the client's files are inspected.
	clientAdapter := cfg.Clients[ID]
	if !clientAdapter.Enabled || clientAdapter.Executable == "" || len(clientAdapter.Targets) != 1 {
		return adapter.Status{Issue: "Hermes executable or configuration home is unavailable", RepairAction: "aigw sync"}
	}
	plan, err := hermesInspectionPlan(deps, cfg, selected, clientAdapter.Targets[0])
	if err != nil {
		return uninspectable(deps, err)
	}
	// A plan that keeps Hermes' file writes only AIGW's record of it: the
	// file already shows the selection, as after a choice made in Hermes.
	if plan.Action != "unchanged" && !plan.keepConfig {
		return adapter.Status{Issue: "Hermes configuration differs from the selected model", RepairAction: "aigw sync"}
	}
	return adapter.Status{Ready: true}
}

// uninspectable is the status of a Hermes file that inspection refused with
// err. A refusal that synchronization shares names the action that resolves
// it.
func uninspectable(deps adapter.Dependencies, err error) adapter.Status {
	if status, refused := adapter.ConflictStatus(err); refused {
		return status
	}
	if failed, atTarget := errors.AsType[*adapter.TargetError](err); atTarget {
		if status, unreadable := adapter.UnreadableStatus(err, failed.Path, deps.Host.Home); unreadable {
			return status
		}
	}
	return adapter.Status{Issue: "Hermes configuration cannot be inspected", RepairAction: "aigw sync"}
}

func hermesInspectionPlan(deps adapter.Dependencies, cfg domain.Config, selected domain.Runtime, target string) (Plan, error) {
	desired, err := hermesDesired(deps, cfg, selected)
	if err != nil {
		return Plan{}, err
	}
	shown := &Selection{Provider: desired.SelectedProvider, Model: desired.SelectedModel}
	plan, err := PrepareTransition(target, &desired, shown, false)
	if err == nil {
		// An inspection compares the file with the configuration that it
		// inspects, which AIGW last wrote there.
		err = plan.refuseReorderedModels(desired.Providers)
	}
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
	return PrepareTransition(target, &desired, shown, false)
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
			reader, err := secret.ExecutableFromCommand(command, ID, item.Runtime.CredentialProjectionFingerprint(ID))
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
