package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

var credentialKey = regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key|auth(?:orization)?(?:[_-]?header)?|credential)`)

type Manifest struct {
	Version            int                       `toml:"version"`
	RecommendedDefault string                    `toml:"recommended_default"`
	Accounts           map[string]domain.Account `toml:"accounts,omitempty"`
	Profiles           map[string]domain.Profile `toml:"profiles"`
}

// MergeOptions makes every local-identity replacement explicit. Team manifests
// are intentionally token-free; they must not silently redirect an existing
// local Account and its system-held Token to a different endpoint.
type MergeOptions struct {
	ReplaceAccounts map[string]bool
	ReplaceProfiles map[string]bool
}

func Parse(data []byte) (Manifest, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Manifest{}, fmt.Errorf("parse team manifest: %w", err)
	}
	if key := findCredentialKey(raw, ""); key != "" {
		return Manifest{}, fmt.Errorf("team manifest contains forbidden credential field %q", key)
	}
	var result Manifest
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Manifest{}, fmt.Errorf("validate team manifest shape: %w", err)
	}
	if result.Version != domain.ConfigVersion {
		return Manifest{}, fmt.Errorf("unsupported team manifest version %d; expected %d", result.Version, domain.ConfigVersion)
	}
	if result.Accounts == nil {
		result.Accounts = map[string]domain.Account{}
	}
	if len(result.Profiles) == 0 {
		return Manifest{}, fmt.Errorf("team manifest must define at least one profile")
	}
	if result.RecommendedDefault != "" {
		if _, ok := result.Profiles[result.RecommendedDefault]; !ok {
			return Manifest{}, fmt.Errorf("recommended default references unknown profile %q", result.RecommendedDefault)
		}
	}
	check := domain.NewConfig()
	check.Accounts = result.Accounts
	check.Profiles = result.Profiles
	if result.RecommendedDefault != "" {
		check.Routes.Default = result.RecommendedDefault
	} else {
		for name := range result.Profiles {
			check.Routes.Default = name
			break
		}
	}
	if err := check.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("invalid team manifest: %w", err)
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

func Merge(cfg domain.Config, team Manifest) (domain.Config, error) {
	return MergeWithOptions(cfg, team, MergeOptions{})
}

func MergeWithOptions(cfg domain.Config, team Manifest, options MergeOptions) (domain.Config, error) {
	if team.Version != domain.ConfigVersion {
		return domain.Config{}, fmt.Errorf("unsupported team manifest version %d; expected %d", team.Version, domain.ConfigVersion)
	}
	merged := cloneConfig(cfg)
	if err := validateReplacementSelectors(team, options); err != nil {
		return domain.Config{}, err
	}
	for name, account := range team.Accounts {
		if existing, exists := merged.Accounts[name]; exists {
			if equivalentAccount(existing, account) {
				continue
			}
			if !options.ReplaceAccounts[name] {
				return domain.Config{}, fmt.Errorf("account %q conflicts with local configuration; inspect it with `aigw account list` and re-run with `aigw config import <team-profiles.toml> --replace-account %s` to explicitly replace the Account metadata while preserving its Token", name, name)
			}
		}
		merged.Accounts[name] = account
	}
	for name, profile := range team.Profiles {
		if existing, exists := merged.Profiles[name]; exists {
			if equivalentProfile(existing, profile) {
				continue
			}
			if !options.ReplaceProfiles[name] {
				return domain.Config{}, fmt.Errorf("profile %q conflicts with local configuration; re-run with `aigw config import <team-profiles.toml> --replace-profile %s` to explicitly replace it", name, name)
			}
		}
		merged.Profiles[name] = profile
	}
	if merged.Routes.Default == "" {
		merged.Routes.Default = team.RecommendedDefault
		if merged.Routes.Default == "" {
			for name := range team.Profiles {
				merged.Routes.Default = name
				break
			}
		}
	}
	if err := merged.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("merge team manifest: %w", err)
	}
	return merged, nil
}

func validateReplacementSelectors(team Manifest, options MergeOptions) error {
	for name := range options.ReplaceAccounts {
		if _, exists := team.Accounts[name]; !exists {
			return fmt.Errorf("--replace-account %q does not name an Account in the imported team manifest", name)
		}
	}
	for name := range options.ReplaceProfiles {
		if _, exists := team.Profiles[name]; !exists {
			return fmt.Errorf("--replace-profile %q does not name a Profile in the imported team manifest", name)
		}
	}
	return nil
}

func cloneConfig(cfg domain.Config) domain.Config {
	copy := cfg
	copy.Accounts = make(map[string]domain.Account, len(cfg.Accounts))
	for name, account := range cfg.Accounts {
		copy.Accounts[name] = account
	}
	copy.Profiles = make(map[string]domain.Profile, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		copy.Profiles[name] = profile
	}
	copy.Routes.Overrides = make(map[string]string, len(cfg.Routes.Overrides))
	for client, profile := range cfg.Routes.Overrides {
		copy.Routes.Overrides[client] = profile
	}
	copy.Adapters = make(map[string]domain.AdapterConfig, len(cfg.Adapters))
	for client, adapter := range cfg.Adapters {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		copy.Adapters[client] = adapter
	}
	copy.Normalize()
	return copy
}

func equivalentAccount(left, right domain.Account) bool {
	return left.Label == right.Label &&
		normalizeEndpoint(left.Endpoints.OpenAIResponses) == normalizeEndpoint(right.Endpoints.OpenAIResponses) &&
		normalizeEndpoint(left.Endpoints.Anthropic) == normalizeEndpoint(right.Endpoints.Anthropic) &&
		equivalentProbe(left.AccountProbe, right.AccountProbe)
}

func equivalentProbe(left, right *domain.AccountProbe) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Kind == right.Kind && normalizeEndpoint(left.BaseURL) == normalizeEndpoint(right.BaseURL)
}

func normalizeEndpoint(value string) string { return strings.TrimRight(strings.TrimSpace(value), "/") }

func equivalentProfile(left, right domain.Profile) bool {
	if left.Label != right.Label || left.Purpose != right.Purpose || left.Account != right.Account || left.Client != right.Client || len(left.Models) != len(right.Models) {
		return false
	}
	for client, model := range left.Models {
		if right.Models[client] != model {
			return false
		}
	}
	return true
}

func Export(cfg domain.Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return toml.Marshal(Manifest{Version: domain.ConfigVersion, RecommendedDefault: cfg.Routes.Default, Accounts: cfg.Accounts, Profiles: cfg.Profiles})
}
