package domain

import (
	"fmt"
	"maps"
	"slices"
)

// TeamMerge is what applying a team file did: what it added and changed,
// what stayed local and why, the capabilities that it dropped, the protocols
// that it left unusable, and what pruning removed. It names models by
// reference.
type TeamMerge struct {
	AddedProviders, AddedFamilies     []string
	AddedModels                       []ModelRef
	AddedRecommendations              []string
	ChangedProviders, ChangedFamilies []string
	ChangedModels                     []ModelRef
	ChangedRecommendations            []string
	KeptEndpoints                     []KeptEndpoint
	KeptProbes                        []KeptProbe
	KeptModels                        []KeptModel
	DroppedCapabilities               []DroppedCapabilities
	Unusable                          []UnusableProtocol
	PrunedModels                      []ModelRef
	PrunedFamilies                    []string
}

// KeptEndpoint is an existing provider's endpoint that stays local, because
// a team file must not redirect the provider's Token. Local is empty when the
// provider has no endpoint for the protocol, which the file does not add.
type KeptEndpoint struct {
	Provider    string
	Protocol    EndpointProtocol
	Local, File string
}

// KeptProbe is an existing provider's balance probe that stays local, as an
// endpoint does.
type KeptProbe struct {
	Provider    string
	Local, File BalanceProbe
}

// KeptModel is a model that keeps its local definition because a client
// selects it on a protocol that the file's definition omits.
type KeptModel struct {
	Model    ModelRef
	Client   string
	Protocol EndpointProtocol
}

// DroppedCapabilities are the capabilities that a team file declared for a
// model on a protocol whose endpoint stays local: they were qualified on the
// file's endpoint, so the model keeps its local capabilities there.
type DroppedCapabilities struct {
	Model        ModelRef
	Protocol     EndpointProtocol
	Capabilities []Capability
}

// UnusableProtocol is a protocol of a model for which its provider has no
// endpoint. No client selects the model on it until the provider has one.
type UnusableProtocol struct {
	Model    ModelRef
	Protocol EndpointProtocol
}

// MergeTeamFile applies a team file, which is a configuration without
// clients, to cfg. The file's models, families and recommendations replace
// the local ones of the same name; local-only ones stay, unless prune removes
// the models that no client selects and no recommendation names, and then the
// families that no model uses. A new provider is added; an existing one takes
// the file's label, and a balance probe when it has none, but keeps its
// endpoints, its probe and its Token. No client's selection changes. The
// file's order comes first in display order, then the local-only entries in
// theirs. The merge returns a valid configuration or an error, and never
// changes cfg.
func MergeTeamFile(cfg Config, incoming Config, prune bool) (Config, TeamMerge, error) {
	if incoming.Version != ConfigVersion {
		return Config{}, TeamMerge{}, &UnsupportedTeamFileVersionError{Version: incoming.Version}
	}
	before := cfg.Clone()
	merged := cfg.Clone()
	var report TeamMerge
	mergeProviders(&merged, incoming, &report)
	maps.Copy(merged.Families, incoming.Families)
	for _, ref := range incoming.ModelRefs() {
		if err := mergeModel(&merged, &before, incoming, ref, &report); err != nil {
			return Config{}, TeamMerge{}, err
		}
	}
	for _, client := range slices.Sorted(maps.Keys(incoming.Recommendations)) {
		recommendation := incoming.Recommendations[client]
		for _, choice := range recommendation.Selections() {
			if _, defined := merged.Models[choice.Model]; !defined {
				return Config{}, TeamMerge{}, NewEntryError("the `"+client+"` recommendation", fmt.Errorf("the team file's %s recommendation names %s, which neither the team file nor the configuration defines", client, choice.Model))
			}
		}
		merged.Recommendations[client] = recommendation
	}
	merged.order = displayOrder{
		providers: fileFirst(incoming.ProviderIDs(), before.ProviderIDs()),
		families:  fileFirst(incoming.FamilyIDs(), before.FamilyIDs()),
		models:    fileFirst(incoming.ModelRefs(), before.ModelRefs()),
	}
	if prune {
		pruneLocalOnly(&merged, incoming)
	}
	merged.Normalize()
	if err := merged.Validate(); err != nil {
		return Config{}, TeamMerge{}, fmt.Errorf("apply the team file: %w", err)
	}
	report.compare(before, merged)
	return merged, report, nil
}

// fileFirst is the display order of a merge: the file's entries in its
// order, then the local-only ones in theirs.
func fileFirst[K comparable](file, local []K) []K {
	return append(slices.Clone(file), slices.DeleteFunc(slices.Clone(local), func(key K) bool { return slices.Contains(file, key) })...)
}

// mergeProviders adds the file's new providers and gives each existing one
// the file's label and, when it has none, the file's balance probe.
func mergeProviders(merged *Config, incoming Config, report *TeamMerge) {
	for _, id := range incoming.ProviderIDs() {
		file := incoming.Providers[id]
		local, exists := merged.Providers[id]
		if !exists {
			merged.Providers[id] = file
			continue
		}
		for _, protocol := range knownProtocols {
			wanted, kept := file.Endpoints.For(protocol), local.Endpoints.For(protocol)
			if wanted != "" && normalizeEndpoint(wanted) != normalizeEndpoint(kept) {
				report.KeptEndpoints = append(report.KeptEndpoints, KeptEndpoint{Provider: id, Protocol: protocol, Local: kept, File: wanted})
			}
		}
		switch {
		case local.BalanceProbe == nil:
			local.BalanceProbe = file.BalanceProbe
		case file.BalanceProbe != nil && !equivalentProbe(local.BalanceProbe, file.BalanceProbe):
			report.KeptProbes = append(report.KeptProbes, KeptProbe{Provider: id, Local: *local.BalanceProbe, File: *file.BalanceProbe})
		}
		local.Label = file.Label
		merged.Providers[id] = local
	}
}

// mergeModel takes the file's definition of the model that ref names, unless
// it would take the protocol of a client's selection of it. A protocol whose
// endpoint the provider kept local keeps the model's local capabilities,
// because the file's were qualified on the file's endpoint, and a protocol
// for which the kept provider has no endpoint is reported as unusable. A
// provider that the file adds has only the file's endpoints, so a protocol
// that it has none for is the file's error, which is refused.
func mergeModel(merged, before *Config, incoming Config, ref ModelRef, report *TeamMerge) error {
	model := incoming.Models[ref]
	local, exists := before.Models[ref]
	for _, client := range before.SelectedClientsForModel(ref) {
		selected, err := before.ResolveRuntime(client, ModelRef{})
		if err != nil {
			continue
		}
		if _, keeps := model.Protocols[selected.Protocol]; !keeps {
			report.KeptModels = append(report.KeptModels, KeptModel{Model: ref, Client: client, Protocol: selected.Protocol})
			return nil
		}
	}
	_, existing := before.Providers[model.Provider]
	provider, filed := merged.Providers[model.Provider], incoming.Providers[model.Provider]
	protocols := make(map[EndpointProtocol][]Capability, len(model.Protocols))
	for _, protocol := range knownProtocols {
		capabilities, declared := model.Protocols[protocol]
		if !declared {
			continue
		}
		kept := provider.Endpoints.For(protocol)
		switch {
		case kept == "" && !existing:
			return NewEntryError("model `"+ref.String()+"`", fmt.Errorf("the team file declares %s on %s, and provider %s, which the file adds, has no %s endpoint", ref, ProtocolLabel(protocol), model.Provider, ProtocolLabel(protocol)))
		case kept == "":
			report.Unusable = append(report.Unusable, UnusableProtocol{Model: ref, Protocol: protocol})
		}
		if normalizeEndpoint(filed.Endpoints.For(protocol)) != normalizeEndpoint(kept) {
			var localCapabilities []Capability
			if exists {
				localCapabilities = local.Protocols[protocol]
			}
			if dropped := slices.DeleteFunc(slices.Clone(capabilities), func(capability Capability) bool {
				return slices.Contains(localCapabilities, capability)
			}); len(dropped) > 0 {
				report.DroppedCapabilities = append(report.DroppedCapabilities, DroppedCapabilities{Model: ref, Protocol: protocol, Capabilities: dropped})
			}
			capabilities = append([]Capability{}, localCapabilities...)
		}
		protocols[protocol] = capabilities
	}
	model.Protocols = protocols
	merged.Models[ref] = model
	return nil
}

// pruneLocalOnly removes the local-only models that no client selects and no
// recommendation names, then the families that neither a remaining model nor
// the file names. Providers stay.
func pruneLocalOnly(merged *Config, incoming Config) {
	named := map[ModelRef]bool{}
	for _, recommendation := range merged.Recommendations {
		for _, choice := range recommendation.Selections() {
			named[choice.Model] = true
		}
	}
	for _, ref := range merged.ModelRefs() {
		if _, filed := incoming.Models[ref]; filed || named[ref] || len(merged.SelectedClientsForModel(ref)) > 0 {
			continue
		}
		delete(merged.Models, ref)
	}
	used := map[string]bool{}
	for _, model := range merged.Models {
		used[model.Family] = true
	}
	for _, family := range merged.FamilyIDs() {
		if _, filed := incoming.Families[family]; !filed && !used[family] {
			delete(merged.Families, family)
		}
	}
}

// compare records what the merge added, changed and pruned.
func (report *TeamMerge) compare(before, after Config) {
	for _, id := range slices.Sorted(maps.Keys(after.Providers)) {
		if previous, existed := before.Providers[id]; !existed {
			report.AddedProviders = append(report.AddedProviders, id)
		} else if !equivalentProvider(previous, after.Providers[id]) {
			report.ChangedProviders = append(report.ChangedProviders, id)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(after.Families)) {
		if previous, existed := before.Families[id]; !existed {
			report.AddedFamilies = append(report.AddedFamilies, id)
		} else if previous != after.Families[id] {
			report.ChangedFamilies = append(report.ChangedFamilies, id)
		}
	}
	for _, ref := range after.ModelRefs() {
		if previous, existed := before.Models[ref]; !existed {
			report.AddedModels = append(report.AddedModels, ref)
		} else if model := after.Models[ref]; !equivalentModel(previous, model) || previous.Label != model.Label {
			report.ChangedModels = append(report.ChangedModels, ref)
		}
	}
	for _, client := range slices.Sorted(maps.Keys(after.Recommendations)) {
		previous, existed := before.Recommendations[client]
		if current := after.Recommendations[client]; !existed {
			report.AddedRecommendations = append(report.AddedRecommendations, client)
		} else if previous.Primary != current.Primary || !slices.Equal(previous.Alternatives, current.Alternatives) {
			report.ChangedRecommendations = append(report.ChangedRecommendations, client)
		}
	}
	for _, ref := range before.ModelRefs() {
		if _, remains := after.Models[ref]; !remains {
			report.PrunedModels = append(report.PrunedModels, ref)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(before.Families)) {
		if _, remains := after.Families[id]; !remains {
			report.PrunedFamilies = append(report.PrunedFamilies, id)
		}
	}
}
