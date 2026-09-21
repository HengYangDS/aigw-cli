package configuration

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var credentialKey = regexp.MustCompile(`(?i)(^|[_-])(token|secret|password|api[_-]?key|auth|authorization(?:[_-]?header)?|credential)($|[_-])`)

const currentVersion = 6

// Manifest is the credential-free team capability document accepted by setup and export.
type Manifest struct {
	Version         int                        `toml:"version"`
	Recommendations map[string]ClientSelection `toml:"recommendations,omitempty"`
	Accounts        map[string]Account         `toml:"accounts,omitempty"`
	Profiles        map[string]Profile         `toml:"profiles"`
}

// MergeOptions makes every local-identity replacement explicit. Configuration
// manifests are intentionally token-free; they must not silently redirect an
// existing local Account and its system-held Token to a different endpoint.
type MergeOptions struct {
	ReplaceAccounts map[string]bool
	ReplaceProfiles map[string]bool
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

// Parse decodes and validates one complete team manifest without applying it.
func Parse(data []byte) (Manifest, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Manifest{}, fmt.Errorf("parse configuration manifest: %w", err)
	}
	if key := findCredentialKey(raw, ""); key != "" {
		return Manifest{}, fmt.Errorf("configuration manifest contains forbidden credential field %q", key)
	}
	var result Manifest
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Manifest{}, fmt.Errorf("validate configuration manifest shape: %w", err)
	}
	if result.Version != currentVersion {
		return Manifest{}, unsupportedManifestVersionError(result.Version)
	}
	if result.Accounts == nil {
		result.Accounts = map[string]Account{}
	}
	if result.Recommendations == nil {
		result.Recommendations = map[string]ClientSelection{}
	}
	if len(result.Profiles) == 0 {
		return Manifest{}, fmt.Errorf("configuration manifest must define at least one profile")
	}
	check := NewConfig()
	check.Accounts = result.Accounts
	check.Profiles = result.Profiles
	for client, selection := range result.Recommendations {
		if !IsAdmittedClient(client) {
			return Manifest{}, fmt.Errorf("recommendation uses unsupported client %q", client)
		}
		if _, ok := result.Profiles[selection.Profile]; !ok {
			return Manifest{}, fmt.Errorf("%s recommendation references unknown profile %q", client, selection.Profile)
		}
	}
	check.Recommendations = result.Recommendations
	if err := check.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("invalid configuration manifest: %w", err)
	}
	return result, nil
}

func findCredentialKey(value any, prefix string) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if credentialKey.MatchString(key) {
				return path
			}
			if found := findCredentialKey(child, path); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findCredentialKey(child, prefix); found != "" {
				return found
			}
		}
	}
	return ""
}

// Merge combines a manifest with existing configuration while preserving conflicting owned entries.
func Merge(cfg Config, incoming Manifest) (Config, error) {
	return MergeWithOptions(cfg, incoming, MergeOptions{})
}

// MergeWithOptions combines a manifest under explicit conflict-replacement policy.
func MergeWithOptions(cfg Config, incoming Manifest, options MergeOptions) (Config, error) {
	if incoming.Version != currentVersion {
		return Config{}, unsupportedManifestVersionError(incoming.Version)
	}
	merged := cfg.Clone()
	if err := validateReplacementSelectors(incoming, options); err != nil {
		return Config{}, err
	}
	for name, account := range incoming.Accounts {
		if existing, exists := merged.Accounts[name]; exists {
			if equivalentAccount(existing, account) {
				continue
			}
			if !options.ReplaceAccounts[name] {
				return Config{}, fmt.Errorf("account %q conflicts with local configuration; inspect it with `aigw config export` and re-run with `aigw config import <toml> --replace-account %s` to explicitly replace the Account metadata while preserving its Token", name, name)
			}
		}
		merged.Accounts[name] = account
	}
	for name, profile := range incoming.Profiles {
		if existing, exists := merged.Profiles[name]; exists {
			if equivalentProfile(existing, profile) {
				continue
			}
			if !options.ReplaceProfiles[name] {
				return Config{}, fmt.Errorf("profile %q conflicts with local configuration; re-run with `aigw config import <toml> --replace-profile %s` to explicitly replace it", name, name)
			}
		}
		merged.Profiles[name] = profile
	}
	maps.Copy(merged.Recommendations, incoming.Recommendations)
	if err := merged.Validate(); err != nil {
		return Config{}, fmt.Errorf("merge configuration manifest: %w", err)
	}
	return merged, nil
}

func unsupportedManifestVersionError(version int) error {
	return fmt.Errorf(
		"unsupported configuration manifest version %d; expected %d; AIGW does not reinterpret schema versions; export the manifest with the matching AIGW release, then review and import that canonical output",
		version,
		currentVersion,
	)
}

func validateReplacementSelectors(incoming Manifest, options MergeOptions) error {
	for name := range options.ReplaceAccounts {
		if _, exists := incoming.Accounts[name]; !exists {
			return fmt.Errorf("--replace-account %q does not name an Account in the imported configuration manifest", name)
		}
	}
	for name := range options.ReplaceProfiles {
		if _, exists := incoming.Profiles[name]; !exists {
			return fmt.Errorf("--replace-profile %q does not name a Profile in the imported configuration manifest", name)
		}
	}
	return nil
}

func equivalentAccount(left, right Account) bool {
	return left.Label == right.Label &&
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

func equivalentProfile(left, right Profile) bool {
	return left.Label == right.Label &&
		left.Purpose == right.Purpose &&
		left.Account == right.Account &&
		left.Model == right.Model &&
		left.Tier == right.Tier &&
		slices.Equal(left.Protocols, right.Protocols)
}

// Export projects configuration into the canonical credential-free team manifest form.
func Export(cfg Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	recommendations := make(map[string]ClientSelection, len(cfg.Recommendations)+len(cfg.Clients))
	maps.Copy(recommendations, cfg.Recommendations)
	for client, binding := range cfg.Clients {
		if binding.Profile != "" {
			recommendations[client] = binding.selection()
		}
	}
	data, err := toml.Marshal(Manifest{Version: currentVersion, Recommendations: recommendations, Accounts: cfg.Accounts, Profiles: cfg.Profiles})
	if err != nil {
		return nil, err
	}
	if _, err := Parse(data); err != nil {
		return nil, fmt.Errorf("export configuration manifest: %w", err)
	}
	return data, nil
}
