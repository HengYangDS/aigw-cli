package domain

import (
	"fmt"
	"slices"
	"strings"
)

// UnsupportedTeamFileVersionError reports a team file of another schema
// version, which AIGW never reinterprets.
type UnsupportedTeamFileVersionError struct{ Version int }

// Error names the file's version and the one that this AIGW reads. It names
// no conversion, because no AIGW command converts a team file.
func (err *UnsupportedTeamFileVersionError) Error() string {
	return fmt.Sprintf("the team file has version %d, and this AIGW reads version %d", err.Version, ConfigVersion)
}

// AdmitTeamFile validates a decoded team file, which is a configuration
// without clients, and returns its normalized form: it recommends models only
// for admitted clients and validates as a configuration. A recommendation may
// name a model that only the configuration defines, which the merge checks.
// Like a configuration, it may declare providers without models, which is
// what `aigw config export` writes for a configuration without models.
func (c *Config) AdmitTeamFile() (Config, error) {
	file := c.Clone()
	file.Normalize()
	check := file.Clone()
	check.Recommendations = make(map[string]ClientRecommendation, len(file.Recommendations))
	for client, recommendation := range file.Recommendations {
		if !file.admission.Admits(client) {
			return Config{}, fmt.Errorf("recommendation uses unsupported client %q", client)
		}
		if recommendation.Primary.Model.IsZero() {
			return Config{}, fmt.Errorf("client recommendation %q has no primary model", client)
		}
		// A choice of a model that only the configuration defines is checked
		// when the file is merged into it.
		var defined []ClientSelection
		for _, selection := range recommendation.Selections() {
			if _, ok := file.Models[selection.Model]; ok {
				defined = append(defined, selection)
			}
		}
		if len(defined) > 0 {
			check.Recommendations[client] = ClientRecommendation{Primary: defined[0], Alternatives: defined[1:]}
		}
	}
	if err := check.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid team file: %w", err)
	}
	return file, nil
}

// ExportTeamFile returns the team file of cfg: its providers, models,
// families and recommendations in its display order, and no clients.
// Recommendations are the declared team preference and stay as declared;
// client selections are the operator's own and stay local; a model label that
// its provider and family would derive is omitted.
func ExportTeamFile(cfg Config) (Config, error) {
	cfg = cfg.Clone()
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	for ref, model := range cfg.Models {
		if model.Label == modelLabel(Model{Provider: model.Provider, ID: model.ID, Family: model.Family}, cfg.Providers[model.Provider], cfg.Families[model.Family]) {
			model.Label = ""
			cfg.Models[ref] = model
		}
	}
	cfg.Clients = map[string]ClientBinding{}
	return cfg, nil
}

func equivalentProvider(left, right Provider) bool {
	return left.Label == right.Label &&
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
	return left.Provider == right.Provider &&
		left.ID == right.ID &&
		left.Family == right.Family &&
		left.Deprecated == right.Deprecated &&
		equalProtocols(left.Protocols, right.Protocols)
}

func equalProtocols(left, right map[EndpointProtocol][]Capability) bool {
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
