package configuration

import (
	"fmt"
	"regexp"
	"strings"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

var credentialKey = regexp.MustCompile(`(?i)(^|[_-])(token|secret|password|api[_-]?key|auth|authorization(?:[_-]?header)?|credential)($|[_-])`)

// Parse decodes and validates one complete team manifest for the admitted
// clients without applying it.
func Parse(data []byte, admission domain.Admission) (domain.Manifest, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return domain.Manifest{}, fmt.Errorf("parse configuration manifest: %w", err)
	}
	if key := findCredentialKey(raw, ""); key != "" {
		return domain.Manifest{}, fmt.Errorf("configuration manifest contains forbidden credential field %q", key)
	}
	var header struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &header); err != nil {
		return domain.Manifest{}, fmt.Errorf("parse configuration manifest: %w", err)
	}
	if header.Version != domain.ManifestVersion {
		return domain.Manifest{}, &domain.UnsupportedManifestVersionError{Version: header.Version}
	}
	var result domain.Manifest
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return domain.Manifest{}, fmt.Errorf("validate configuration manifest shape: %w", err)
	}
	return result.Admit(admission)
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

// Export encodes configuration as the canonical credential-free team manifest
// and proves that the encoding parses back for the configuration's clients.
func Export(cfg domain.Config) ([]byte, error) {
	manifest, err := domain.ExportManifest(cfg)
	if err != nil {
		return nil, err
	}
	data, err := toml.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	if _, err := Parse(data, cfg.Admission()); err != nil {
		return nil, fmt.Errorf("export configuration manifest: %w", err)
	}
	return data, nil
}
