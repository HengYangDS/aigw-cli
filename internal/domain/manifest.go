package domain

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// ManifestVersion is the only team manifest schema version that this build
// accepts and exports.
const ManifestVersion = 7

// Manifest is the credential-free team capability document accepted by setup and export.
type Manifest struct {
	Version         int                             `toml:"version"`
	Recommendations map[string]ClientRecommendation `toml:"recommendations,omitempty"`
	Accounts        map[string]Account              `toml:"accounts,omitempty"`
	Models          map[string]Model                `toml:"models"`
	Routes          map[string]Route                `toml:"routes"`
}

// MergeOptions makes local identity replacement and Route retirement explicit.
// Configuration manifests are token-free and must not silently redirect an
// existing Account and its system-held Token to a different endpoint.
type MergeOptions struct {
	KeepAccounts    map[string]bool
	ReplaceAccounts map[string]bool
	ReplaceModels   map[string]bool
	ReplaceRoutes   map[string]bool
	RetireRoutes    map[string]bool
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

// ManifestAccountNames returns every credential owner referenced by a
// configuration manifest. Credential ownership is part of the manifest model,
// so all consumers share this definition instead of depending on a CLI command
// package.
func ManifestAccountNames(incoming Manifest) []string {
	names := make([]string, 0, len(incoming.Accounts))
	for name := range incoming.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Admit validates a decoded team manifest for the admitted clients and returns
// its normalized form: it declares at least one Route, recommends Routes only
// for admitted clients and declared Routes, and validates as a configuration.
func (m Manifest) Admit(admission Admission) (Manifest, error) {
	if m.Accounts == nil {
		m.Accounts = map[string]Account{}
	}
	if m.Recommendations == nil {
		m.Recommendations = map[string]ClientRecommendation{}
	}
	if len(m.Routes) == 0 {
		return Manifest{}, fmt.Errorf("configuration manifest must define at least one route")
	}
	check := NewConfig(admission)
	check.Accounts = m.Accounts
	check.Models = m.Models
	check.Routes = m.Routes
	check.Normalize()
	m.Models = check.Models
	m.Routes = check.Routes
	for client, recommendation := range m.Recommendations {
		if !admission.Admits(client) {
			return Manifest{}, fmt.Errorf("recommendation uses unsupported client %q", client)
		}
		for _, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
			if _, ok := m.Routes[selection.Route]; !ok {
				return Manifest{}, fmt.Errorf("%s recommendation references unknown route %q", client, selection.Route)
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
// Route label that its Account and Model would derive is omitted.
func ExportManifest(cfg Config) (Manifest, error) {
	cfg = cfg.Clone()
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Manifest{}, err
	}
	for routeID, route := range cfg.Routes {
		derived := routeLabel(Route{Account: route.Account, Model: route.Model}, cfg.Accounts[route.Account], cfg.Models[route.Model])
		if route.Label == derived {
			route.Label = ""
			cfg.Routes[routeID] = route
		}
	}
	return Manifest{Version: ManifestVersion, Recommendations: cfg.Recommendations, Accounts: cfg.Accounts, Models: cfg.Models, Routes: cfg.Routes}, nil
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
	retiredModels := make(map[string]bool, len(options.RetireRoutes))
	for name, retire := range options.RetireRoutes {
		if !retire {
			continue
		}
		retiredModels[merged.Routes[name].Model] = true
		delete(merged.Routes, name)
	}
	for name, account := range incoming.Accounts {
		if existing, exists := merged.Accounts[name]; exists {
			if equivalentAccount(existing, account) || options.KeepAccounts[name] {
				continue
			}
			if !options.ReplaceAccounts[name] {
				return Config{}, fmt.Errorf("account %q conflicts with local configuration; inspect it with `aigw config export`, then use --keep-account %s to retain local metadata or --replace-account %s to use incoming metadata; its Token is unchanged", name, name, name)
			}
		}
		merged.Accounts[name] = account
	}
	for name, model := range incoming.Models {
		if existing, exists := merged.Models[name]; exists {
			if existing == model {
				continue
			}
			if !options.ReplaceModels[name] {
				return Config{}, fmt.Errorf("model %q conflicts with local configuration; re-run with `aigw config import <toml> --replace-model %s` to explicitly replace it", name, name)
			}
		}
		merged.Models[name] = model
	}
	for name, route := range incoming.Routes {
		if existing, exists := merged.Routes[name]; exists {
			if equivalentRoute(existing, route) && routeLabel(existing, merged.Accounts[route.Account], merged.Models[route.Model]) == routeLabel(route, merged.Accounts[route.Account], merged.Models[route.Model]) {
				continue
			}
			if !options.ReplaceRoutes[name] {
				return Config{}, fmt.Errorf("route %q conflicts with local configuration; re-run with `aigw config import <toml> --replace-route %s` to explicitly replace it", name, name)
			}
		}
		merged.Routes[name] = route
	}
	maps.Copy(merged.Models, incoming.Models)
	maps.Copy(merged.Recommendations, incoming.Recommendations)
	for model := range retiredModels {
		if _, declared := incoming.Models[model]; declared {
			continue
		}
		referenced := false
		for _, route := range merged.Routes {
			if route.Model == model {
				referenced = true
				break
			}
		}
		if !referenced {
			delete(merged.Models, model)
		}
	}
	merged.Normalize()
	if err := merged.Validate(); err != nil {
		return Config{}, fmt.Errorf("merge configuration manifest: %w", err)
	}
	return merged, nil
}

func validateMergeSelectors(cfg Config, incoming Manifest, options MergeOptions) error {
	for _, name := range slices.Sorted(maps.Keys(options.KeepAccounts)) {
		if !options.KeepAccounts[name] {
			continue
		}
		if options.ReplaceAccounts[name] {
			return fmt.Errorf("--keep-account %q conflicts with --replace-account for the same Account", name)
		}
		if _, exists := cfg.Accounts[name]; !exists {
			return fmt.Errorf("--keep-account %q does not name an existing local Account", name)
		}
		if _, exists := incoming.Accounts[name]; !exists {
			return fmt.Errorf("--keep-account %q does not name an Account in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceAccounts {
		if _, exists := incoming.Accounts[name]; !exists {
			return fmt.Errorf("--replace-account %q does not name an Account in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceModels {
		if _, exists := incoming.Models[name]; !exists {
			return fmt.Errorf("--replace-model %q does not name a Model in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceRoutes {
		if _, exists := incoming.Routes[name]; !exists {
			return fmt.Errorf("--replace-route %q does not name a Route in the imported configuration manifest", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(options.RetireRoutes)) {
		if !options.RetireRoutes[name] {
			continue
		}
		if _, exists := cfg.Routes[name]; !exists {
			return fmt.Errorf("--retire-route %q does not name an existing Route", name)
		}
		if _, exists := incoming.Routes[name]; exists {
			return fmt.Errorf("--retire-route %q is also declared in the imported configuration manifest", name)
		}
		for _, client := range slices.Sorted(maps.Keys(cfg.Clients)) {
			if cfg.Clients[client].Route == name {
				return fmt.Errorf("--retire-route %q is selected by client %q", name, client)
			}
		}
	}
	return nil
}

func equivalentAccount(left, right Account) bool {
	return left.Label == right.Label && left.Order == right.Order &&
		normalizeEndpoint(left.Endpoints.OpenAIResponses) == normalizeEndpoint(right.Endpoints.OpenAIResponses) &&
		normalizeEndpoint(left.Endpoints.OpenAIChatCompletions) == normalizeEndpoint(right.Endpoints.OpenAIChatCompletions) &&
		normalizeEndpoint(left.Endpoints.Anthropic) == normalizeEndpoint(right.Endpoints.Anthropic) &&
		equivalentProbe(left.AccountProbe, right.AccountProbe)
}

func equivalentProbe(left, right *AccountProbe) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Kind == right.Kind && normalizeEndpoint(left.BaseURL) == normalizeEndpoint(right.BaseURL)
}

func normalizeEndpoint(value string) string { return strings.TrimRight(strings.TrimSpace(value), "/") }

func equivalentRoute(left, right Route) bool {
	return left.Purpose == right.Purpose &&
		left.Account == right.Account &&
		left.Model == right.Model &&
		left.UpstreamModelID() == right.UpstreamModelID() &&
		left.LifecycleState() == right.LifecycleState() &&
		equalInterfaces(left.Interfaces, right.Interfaces)
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
