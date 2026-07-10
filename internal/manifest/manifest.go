package manifest

import (
	"encoding/json"
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
	Profiles           map[string]domain.Profile `toml:"profiles"`
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
	if result.Version != 1 {
		return Manifest{}, fmt.Errorf("unsupported team manifest version %d", result.Version)
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
	return toml.Marshal(Manifest{Version: 1, RecommendedDefault: cfg.Routes.Default, Profiles: cfg.Profiles})
}

type legacyConfig struct {
	Version  int                      `json:"version"`
	Profiles map[string]legacyProfile `json:"profiles"`
	Routes   map[string]string        `json:"routes"`
}

type legacyProfile struct {
	Label    string                       `json:"label"`
	BaseURL  string                       `json:"base_url"`
	Adapters map[string]map[string]string `json:"adapters"`
	Proxy    map[string]json.RawMessage   `json:"proxy"`
}

func MigrateLegacyV2(data []byte) (domain.Config, error) {
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return domain.Config{}, fmt.Errorf("parse legacy config: %w", err)
	}
	if legacy.Version != 2 {
		return domain.Config{}, fmt.Errorf("unsupported legacy config version %d", legacy.Version)
	}
	cfg := domain.NewConfig()
	for name, source := range legacy.Profiles {
		profile := domain.Profile{Label: source.Label}
		profile.Endpoints.OpenAIResponses = source.BaseURL
		if codex := source.Adapters[domain.ClientCodex]; codex != nil && codex["base_url"] != "" {
			profile.Endpoints.OpenAIResponses = codex["base_url"]
		}
		if claude := source.Adapters[domain.ClientClaude]; claude != nil {
			profile.Endpoints.Anthropic = claude["base_url"]
		}
		if enabledRaw, ok := source.Proxy["codex_responses"]; ok {
			var enabled bool
			_ = json.Unmarshal(enabledRaw, &enabled)
			if enabled {
				var proxyURL string
				_ = json.Unmarshal(source.Proxy["url"], &proxyURL)
				if proxyURL != "" {
					profile.Endpoints.OpenAIResponses = proxyURL
				}
			}
		}
		cfg.Profiles[name] = profile
	}
	cfg.Routes.Default = legacy.Routes["default"]
	for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
		if route := legacy.Routes[client]; route != "" && route != cfg.Routes.Default {
			cfg.Routes.Overrides[client] = route
		}
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("migrated config is invalid: %w", err)
	}
	return cfg, nil
}
