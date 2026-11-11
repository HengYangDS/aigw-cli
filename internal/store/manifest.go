package store

import (
	"errors"
	"fmt"
	"regexp"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

// ErrTeamFileClients refuses a team file with a [clients] section: each
// machine keeps its own client selections, which a team file never sets.
var ErrTeamFileClients = errors.New("the team file has a [clients] section, and a team file never sets a client's selection")

var credentialKey = regexp.MustCompile(`(?i)(^|[_-])(token|secret|password|api[_-]?key|auth|authorization(?:[_-]?header)?|credential)($|[_-])`)

// ParseTeamFile decodes and validates one complete team file for the admitted
// clients without applying it. A team file has the configuration's form
// without clients, in the same version; a credential field, a `[clients]`
// section, another version and a field that the version does not define are
// refused, in that order.
func ParseTeamFile(data []byte, admission domain.Admission) (domain.Config, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return domain.Config{}, fmt.Errorf("parse the team file: %w", err)
	}
	if key := findCredentialKey(raw, ""); key != "" {
		return domain.Config{}, domain.NewEntryError("the field `"+key+"`", fmt.Errorf("the team file has the credential field %q, and a team file never holds a credential", key))
	}
	if _, clients := raw["clients"]; clients {
		return domain.Config{}, ErrTeamFileClients
	}
	version, err := readVersion(data)
	if err != nil {
		return domain.Config{}, fmt.Errorf("parse the team file: %w", err)
	}
	if version != domain.ConfigVersion {
		return domain.Config{}, &domain.UnsupportedTeamFileVersionError{Version: version}
	}
	decoded, err := decodeDocument(data)
	if err != nil {
		return domain.Config{}, fmt.Errorf("parse the team file: %w", err)
	}
	file, err := decoded.config(admission)
	if err != nil {
		return domain.Config{}, fmt.Errorf("invalid team file: %w", err)
	}
	return file.AdmitTeamFile()
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

// ExportTeamFile encodes configuration as a credential-free team file, in its
// display order and without clients, and proves that the encoding parses
// back for the configuration's clients.
func ExportTeamFile(cfg domain.Config) ([]byte, error) {
	file, err := domain.ExportTeamFile(cfg)
	if err != nil {
		return nil, err
	}
	data, err := encodeDocument(newDocument(file))
	if err != nil {
		return nil, err
	}
	if _, err := ParseTeamFile(data, cfg.Admission()); err != nil {
		return nil, fmt.Errorf("export the team file: %w", err)
	}
	return data, nil
}
