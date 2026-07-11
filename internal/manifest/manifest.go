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

const (
	legacyVersion  = domain.LegacyConfigVersion
	currentVersion = domain.CurrentConfigVersion
)

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
	if result.Version != legacyVersion && result.Version != currentVersion {
		return Manifest{}, fmt.Errorf("unsupported team manifest version %d", result.Version)
	}
	if result.Accounts == nil {
		result.Accounts = map[string]domain.Account{}
	}
	if len(result.Profiles) == 0 {
		return Manifest{}, fmt.Errorf("team manifest must define at least one profile")
	}
	if result.Version == legacyVersion {
		for name, profile := range result.Profiles {
			if strings.TrimSpace(profile.Purpose) != "" {
				return Manifest{}, fmt.Errorf("team manifest profile %q purpose requires version %d", name, currentVersion)
			}
		}
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
	cfg.Normalize()
	if team.Version > cfg.Version {
		return domain.Config{}, fmt.Errorf("team manifest version %d requires local config version %d; run `aigw config upgrade`", team.Version, team.Version)
	}
	for name, account := range team.Accounts {
		cfg.Accounts[name] = account
	}
	for name, profile := range team.Profiles {
		cfg.Profiles[name] = profile
	}
	if cfg.Routes.Default == "" {
		cfg.Routes.Default = team.RecommendedDefault
		if cfg.Routes.Default == "" {
			for name := range team.Profiles {
				cfg.Routes.Default = name
				break
			}
		}
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("merge team manifest: %w", err)
	}
	return cfg, nil
}

func Export(cfg domain.Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return toml.Marshal(Manifest{Version: cfg.Version, RecommendedDefault: cfg.Routes.Default, Accounts: cfg.Accounts, Profiles: cfg.Profiles})
}
