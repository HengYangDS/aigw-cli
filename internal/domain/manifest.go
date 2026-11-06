package domain

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ManifestVersion is the only team manifest schema version that this build
// accepts and exports.
const ManifestVersion = 7

// Manifest is the credential-free team capability document accepted by setup and export.
type Manifest struct {
	Version         int                             `toml:"version"`
	Recommendations map[string]ClientRecommendation `toml:"recommendations,omitempty"`
	Providers       map[string]Provider             `toml:"accounts,omitempty"`
	Families        map[string]Family               `toml:"models"`
	Models          map[string]Model                `toml:"routes"`
}

// MergeOptions makes local identity replacement and Route retirement explicit.
// Configuration manifests are token-free and must not silently redirect an
// existing Account and its system-held Token to a different endpoint.
type MergeOptions struct {
	KeepProviders    map[string]bool
	ReplaceProviders map[string]bool
	ReplaceFamilies  map[string]bool
	ReplaceModels    map[string]bool
	RetireModels     map[string]bool
}

// UnsupportedManifestVersionError reports a team manifest of another schema
// version, which AIGW never reinterprets.
type UnsupportedManifestVersionError struct{ Version int }

// Error formats the version refusal and the supported way to convert the manifest.
func (err *UnsupportedManifestVersionError) Error() string {
	return fmt.Sprintf(
		"unsupported configuration manifest version %d; expected %d; AIGW does not reinterpret schema versions; export the manifest with the matching AIGW release, then review and import that canonical output",
		err.Version,
		ManifestVersion,
	)
}

// ManifestProviderIDs returns every credential owner referenced by a
// configuration manifest. Credential ownership is part of the manifest model,
// so all consumers share this definition instead of depending on a CLI command
// package.
func ManifestProviderIDs(incoming Manifest) []string {
	return slices.Sorted(maps.Keys(incoming.Providers))
}

// Admit validates a decoded team manifest for the admitted clients and returns
// its normalized form: it declares at least one Route, recommends Routes only
// for admitted clients and declared Routes, and validates as a configuration.
func (m Manifest) Admit(admission Admission) (Manifest, error) {
	if m.Providers == nil {
		m.Providers = map[string]Provider{}
	}
	if m.Recommendations == nil {
		m.Recommendations = map[string]ClientRecommendation{}
	}
	if len(m.Models) == 0 {
		return Manifest{}, fmt.Errorf("configuration manifest must define at least one route")
	}
	check := NewConfig(admission)
	check.Providers = m.Providers
	check.Families = m.Families
	check.Models = m.Models
	check.Normalize()
	m.Families = check.Families
	m.Models = check.Models
	for client, recommendation := range m.Recommendations {
		if !admission.Admits(client) {
			return Manifest{}, fmt.Errorf("recommendation uses unsupported client %q", client)
		}
		for _, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
			if _, ok := m.Models[selection.Model]; !ok {
				return Manifest{}, fmt.Errorf("%s recommendation references unknown route %q", client, selection.Model)
			}
		}
	}
	check.Recommendations = m.Recommendations
	if err := check.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("invalid configuration manifest: %w", err)
	}
	return m, nil
}

// ExportManifest projects configuration into the canonical credential-free team
// manifest. Recommendations are the declared team preference and stay as
// declared; Client Bindings are the operator's own choices and stay local; a
// model label that its provider and family would derive is omitted.
func ExportManifest(cfg Config) (Manifest, error) {
	cfg = cfg.Clone()
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Manifest{}, err
	}
	for routeID, route := range cfg.Models {
		derived := modelLabel(Model{Provider: route.Provider, Family: route.Family}, cfg.Providers[route.Provider], cfg.Families[route.Family])
		if route.Label == derived {
			route.Label = ""
			cfg.Models[routeID] = route
		}
	}
	return Manifest{Version: ManifestVersion, Recommendations: cfg.Recommendations, Providers: cfg.Providers, Families: cfg.Families, Models: cfg.Models}, nil
}

// Merge combines a manifest with existing configuration while preserving conflicting owned entries.
func Merge(cfg Config, incoming Manifest) (Config, error) {
	return MergeWithOptions(cfg, incoming, MergeOptions{})
}

// MergeWithOptions combines a manifest under explicit conflict-replacement policy.
func MergeWithOptions(cfg Config, incoming Manifest, options MergeOptions) (Config, error) {
	if incoming.Version != ManifestVersion {
		return Config{}, &UnsupportedManifestVersionError{Version: incoming.Version}
	}
	merged := cfg.Clone()
	if err := validateMergeSelectors(cfg, incoming, options); err != nil {
		return Config{}, err
	}
	retiredFamilies := make(map[string]bool, len(options.RetireModels))
	for name, retire := range options.RetireModels {
		if !retire {
			continue
		}
		retiredFamilies[merged.Models[name].Family] = true
		delete(merged.Models, name)
	}
	for name, account := range incoming.Providers {
		if existing, exists := merged.Providers[name]; exists {
			if equivalentProvider(existing, account) || options.KeepProviders[name] {
				continue
			}
			if !options.ReplaceProviders[name] {
				return Config{}, fmt.Errorf("account %q conflicts with local configuration; inspect it with `aigw config export`, then use --keep-account %s to retain local metadata or --replace-account %s to use incoming metadata; its Token is unchanged", name, name, name)
			}
		}
		merged.Providers[name] = account
	}
	for name, model := range incoming.Families {
		if existing, exists := merged.Families[name]; exists {
			if existing == model {
				continue
			}
			if !options.ReplaceFamilies[name] {
				return Config{}, fmt.Errorf("model %q conflicts with local configuration; re-run with `aigw config import <toml> --replace-model %s` to explicitly replace it", name, name)
			}
		}
		merged.Families[name] = model
	}
	for name, route := range incoming.Models {
		if existing, exists := merged.Models[name]; exists {
			if equivalentModel(existing, route) && modelLabel(existing, merged.Providers[route.Provider], merged.Families[route.Family]) == modelLabel(route, merged.Providers[route.Provider], merged.Families[route.Family]) {
				continue
			}
			if !options.ReplaceModels[name] {
				return Config{}, fmt.Errorf("route %q conflicts with local configuration; re-run with `aigw config import <toml> --replace-route %s` to explicitly replace it", name, name)
			}
		}
		merged.Models[name] = route
	}
	maps.Copy(merged.Families, incoming.Families)
	maps.Copy(merged.Recommendations, incoming.Recommendations)
	for family := range retiredFamilies {
		if _, declared := incoming.Families[family]; declared {
			continue
		}
		referenced := false
		for _, route := range merged.Models {
			if route.Family == family {
				referenced = true
				break
			}
		}
		if !referenced {
			delete(merged.Families, family)
		}
	}
	merged.Normalize()
	if err := merged.Validate(); err != nil {
		return Config{}, fmt.Errorf("merge configuration manifest: %w", err)
	}
	return merged, nil
}

func validateMergeSelectors(cfg Config, incoming Manifest, options MergeOptions) error {
	for _, name := range slices.Sorted(maps.Keys(options.KeepProviders)) {
		if !options.KeepProviders[name] {
			continue
		}
		if options.ReplaceProviders[name] {
			return fmt.Errorf("--keep-account %q conflicts with --replace-account for the same Account", name)
		}
		if _, exists := cfg.Providers[name]; !exists {
			return fmt.Errorf("--keep-account %q does not name an existing local Account", name)
		}
		if _, exists := incoming.Providers[name]; !exists {
			return fmt.Errorf("--keep-account %q does not name an Account in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceProviders {
		if _, exists := incoming.Providers[name]; !exists {
			return fmt.Errorf("--replace-account %q does not name an Account in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceFamilies {
		if _, exists := incoming.Families[name]; !exists {
			return fmt.Errorf("--replace-model %q does not name a Model in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceModels {
		if _, exists := incoming.Models[name]; !exists {
			return fmt.Errorf("--replace-route %q does not name a Route in the imported configuration manifest", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(options.RetireModels)) {
		if !options.RetireModels[name] {
			continue
		}
		if _, exists := cfg.Models[name]; !exists {
			return fmt.Errorf("--retire-route %q does not name an existing Route", name)
		}
		if _, exists := incoming.Models[name]; exists {
			return fmt.Errorf("--retire-route %q is also declared in the imported configuration manifest", name)
		}
		for _, client := range slices.Sorted(maps.Keys(cfg.Clients)) {
			if cfg.Clients[client].Model == name {
				return fmt.Errorf("--retire-route %q is selected by client %q", name, client)
			}
		}
	}
	return nil
}

func equivalentProvider(left, right Provider) bool {
	return left.Label == right.Label && left.Order == right.Order &&
		normalizeEndpoint(left.Endpoints.OpenAIResponses) == normalizeEndpoint(right.Endpoints.OpenAIResponses) &&
		normalizeEndpoint(left.Endpoints.OpenAIChatCompletions) == normalizeEndpoint(right.Endpoints.OpenAIChatCompletions) &&
		normalizeEndpoint(left.Endpoints.Anthropic) == normalizeEndpoint(right.Endpoints.Anthropic) &&
		equivalentProbe(left.BalanceProbe, right.BalanceProbe)
}

func equivalentProbe(left, right *BalanceProbe) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Kind == right.Kind && normalizeEndpoint(left.BaseURL) == normalizeEndpoint(right.BaseURL)
}

func normalizeEndpoint(value string) string { return strings.TrimRight(strings.TrimSpace(value), "/") }

func equivalentModel(left, right Model) bool {
	return left.Purpose == right.Purpose &&
		left.Provider == right.Provider &&
		left.Family == right.Family &&
		left.UpstreamModelID() == right.UpstreamModelID() &&
		left.LifecycleState() == right.LifecycleState() &&
		equalInterfaces(left.Protocols, right.Protocols)
}

func equalInterfaces(left, right map[EndpointProtocol][]Capability) bool {
	if len(left) != len(right) {
		return false
	}
	for protocol, capabilities := range left {
		other, present := right[protocol]
		if !present || !slices.Equal(capabilities, other) {
			return false
		}
	}
	return true
}
