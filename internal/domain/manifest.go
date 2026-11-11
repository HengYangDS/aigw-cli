package domain

import (
	"fmt"
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

// UnsupportedManifestVersionError reports a team manifest of another schema
// version, which AIGW never reinterprets.
type UnsupportedManifestVersionError struct{ Version int }

// Error formats the version refusal. It names no way to convert the
// manifest, because no AIGW command converts one.
func (err *UnsupportedManifestVersionError) Error() string {
	return fmt.Sprintf(
		"unsupported configuration manifest version %d; expected %d; AIGW does not reinterpret schema versions",
		err.Version,
		ManifestVersion,
	)
}

// Admit validates a decoded team manifest for the admitted clients and returns
// its normalized form: it recommends models only for admitted clients, and
// validates as a configuration. A recommendation may name a model that only
// the configuration defines, which the merge checks. Like a configuration, it
// may declare providers without Routes, which is what `aigw config export`
// writes for a configuration without models.
func (m Manifest) Admit(admission Admission) (Manifest, error) {
	if m.Providers == nil {
		m.Providers = map[string]Provider{}
	}
	if m.Recommendations == nil {
		m.Recommendations = map[string]ClientRecommendation{}
	}
	check := NewConfig(admission)
	check.Providers = m.Providers
	check.Families = m.Families
	check.Models = m.Models
	check.Normalize()
	m.Families = check.Families
	m.Models = check.Models
	check.Recommendations = make(map[string]ClientRecommendation, len(m.Recommendations))
	for client, recommendation := range m.Recommendations {
		if !admission.Admits(client) {
			return Manifest{}, fmt.Errorf("recommendation uses unsupported client %q", client)
		}
		if recommendation.Primary.Model == "" {
			return Manifest{}, fmt.Errorf("client recommendation %q has no primary route", client)
		}
		// A choice of a model that only the configuration defines is checked
		// when the file is merged into it.
		var defined []ClientSelection
		for _, selection := range recommendation.Selections() {
			if _, ok := m.Models[selection.Model]; ok {
				defined = append(defined, selection)
			}
		}
		if len(defined) > 0 {
			check.Recommendations[client] = ClientRecommendation{Primary: defined[0], Alternatives: defined[1:]}
		}
	}
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

func equivalentProvider(left, right Provider) bool {
	return left.Label == right.Label && left.Order == right.Order &&
		normalizeEndpoint(left.Endpoints.Responses) == normalizeEndpoint(right.Endpoints.Responses) &&
		normalizeEndpoint(left.Endpoints.Chat) == normalizeEndpoint(right.Endpoints.Chat) &&
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
