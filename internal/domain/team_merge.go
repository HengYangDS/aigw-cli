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
// selects it and the file's definition would change that selection: by
// naming another provider's model or another model ID, or by omitting the
// selection's protocol.
type KeptModel struct {
	Model    ModelRef
	Client   string
	Protocol EndpointProtocol
	// File is the model that the file's definition names.
	File ModelRef
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

// MergeTeamFile applies a team file to cfg. The file's models, families and
// recommendations replace the local ones of the same name; local-only ones
// stay, unless prune removes the models that no client selects and no
// recommendation names, and then the families that no model uses. A new
// provider is added; an existing one takes the file's label, and a balance
// probe when it has none, but keeps its endpoints, its probe and its Token. No
// client's selection changes. The merge returns a valid configuration or an
// error, and never changes cfg.
func MergeTeamFile(cfg Config, incoming Manifest, prune bool) (Config, TeamMerge, error) {
	if incoming.Version != ManifestVersion {
		return Config{}, TeamMerge{}, &UnsupportedManifestVersionError{Version: incoming.Version}
	}
	before := cfg.Clone()
	merged := cfg.Clone()
	var report TeamMerge
	mergeProviders(&merged, incoming, &report)
	maps.Copy(merged.Families, incoming.Families)
	for _, key := range slices.Sorted(maps.Keys(incoming.Models)) {
		mergeModel(&merged, &before, incoming, key, &report)
	}
	for _, client := range slices.Sorted(maps.Keys(incoming.Recommendations)) {
		recommendation := incoming.Recommendations[client]
		for _, choice := range recommendation.Selections() {
			if _, defined := merged.Models[choice.Model]; !defined {
				return Config{}, TeamMerge{}, fmt.Errorf("the team file's %s recommendation names %q, which neither the team file nor the configuration defines", client, choice.Model)
			}
		}
		merged.Recommendations[client] = recommendation
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

// mergeProviders adds the file's new providers and gives each existing one
// the file's label and display order and, when it has none, the file's
// balance probe.
func mergeProviders(merged *Config, incoming Manifest, report *TeamMerge) {
	for _, id := range slices.Sorted(maps.Keys(incoming.Providers)) {
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
		local.Label, local.Order = file.Label, file.Order
		merged.Providers[id] = local
	}
}

// knownProtocols are the protocols in the order in which reports name them.
var knownProtocols = []EndpointProtocol{ProtocolAnthropic, ProtocolOpenAIResponses, ProtocolOpenAIChatCompletions}

// mergeModel takes the file's definition of the model stored under key,
// unless it would change a client's selection of it. A protocol whose
// endpoint the provider kept local keeps the model's local capabilities,
// because the file's were qualified on the file's endpoint, and a protocol
// for which the kept provider has no endpoint is reported as unusable.
func mergeModel(merged, before *Config, incoming Manifest, key string, report *TeamMerge) {
	model := incoming.Models[key]
	local, exists := before.Models[key]
	for _, client := range before.SelectedClientsForModel(key) {
		selected, err := before.ResolveRuntime(client, "")
		if err != nil {
			continue
		}
		if _, keeps := model.Protocols[selected.Protocol]; !keeps || model.Ref() != local.Ref() {
			kept := KeptModel{Model: local.Ref(), Client: client, File: model.Ref()}
			if !keeps {
				kept.Protocol = selected.Protocol
			}
			report.KeptModels = append(report.KeptModels, kept)
			return
		}
	}
	provider, filed := merged.Providers[model.Provider], incoming.Providers[model.Provider]
	protocols := make(map[EndpointProtocol][]Capability, len(model.Protocols))
	for _, protocol := range knownProtocols {
		capabilities, declared := model.Protocols[protocol]
		if !declared {
			continue
		}
		kept := provider.Endpoints.For(protocol)
		if kept == "" {
			report.Unusable = append(report.Unusable, UnusableProtocol{Model: model.Ref(), Protocol: protocol})
		}
		if normalizeEndpoint(filed.Endpoints.For(protocol)) != normalizeEndpoint(kept) {
			var localCapabilities []Capability
			if exists {
				localCapabilities = local.Protocols[protocol]
			}
			if dropped := slices.DeleteFunc(slices.Clone(capabilities), func(capability Capability) bool {
				return slices.Contains(localCapabilities, capability)
			}); len(dropped) > 0 {
				report.DroppedCapabilities = append(report.DroppedCapabilities, DroppedCapabilities{Model: model.Ref(), Protocol: protocol, Capabilities: dropped})
			}
			capabilities = append([]Capability{}, localCapabilities...)
		}
		protocols[protocol] = capabilities
	}
	model.Protocols = protocols
	merged.Models[key] = model
}

// pruneLocalOnly removes the local-only models that no client selects and no
// recommendation names, then the families that neither a remaining model nor
// the file names. Providers stay.
func pruneLocalOnly(merged *Config, incoming Manifest) {
	named := map[string]bool{}
	for _, recommendation := range merged.Recommendations {
		for _, choice := range recommendation.Selections() {
			named[choice.Model] = true
		}
	}
	for _, key := range merged.ModelIDs() {
		if _, filed := incoming.Models[key]; filed || named[key] || len(merged.SelectedClientsForModel(key)) > 0 {
			continue
		}
		delete(merged.Models, key)
	}
	used := map[string]bool{}
	for _, model := range merged.Models {
		used[model.Family] = true
	}
	for _, family := range slices.Sorted(maps.Keys(merged.Families)) {
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
	for _, key := range after.ModelIDs() {
		if previous, existed := before.Models[key]; !existed {
			report.AddedModels = append(report.AddedModels, after.Models[key].Ref())
		} else if model := after.Models[key]; !equivalentModel(previous, model) || previous.Label != model.Label {
			report.ChangedModels = append(report.ChangedModels, model.Ref())
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
	for _, key := range before.ModelIDs() {
		if _, remains := after.Models[key]; !remains {
			report.PrunedModels = append(report.PrunedModels, before.Models[key].Ref())
		}
	}
	for _, id := range slices.Sorted(maps.Keys(before.Families)) {
		if _, remains := after.Families[id]; !remains {
			report.PrunedFamilies = append(report.PrunedFamilies, id)
		}
	}
}
