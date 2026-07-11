package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type Store struct{ path string }

// legacyProfile exists only at the configuration decode boundary.  It accepts
// the pre-Account TOML shape long enough to promote it into the canonical
// Account -> Profile model; it is never exposed by domain types or re-emitted.
type legacyProfile struct {
	Label        string               `toml:"label" json:"label"`
	Account      string               `toml:"account" json:"account"`
	Client       string               `toml:"client" json:"client"`
	Models       domain.Models        `toml:"models" json:"models"`
	Endpoints    domain.Endpoints     `toml:"endpoints" json:"endpoints"`
	AccountProbe *domain.AccountProbe `toml:"account_probe" json:"account_probe"`
}

type legacyTOMLConfig struct {
	Profiles map[string]legacyProfile `toml:"profiles"`
}

type legacyJSONConfig struct {
	Profiles map[string]legacyProfile `json:"profiles"`
}

// VerifiedCheckpoint is a secret-free record written only after all requested
// client protocol verifications succeed. It is suitable for rollback, not for
// credential recovery.
type VerifiedCheckpoint struct {
	Config     domain.Config `json:"config"`
	Clients    []string      `json:"clients"`
	VerifiedAt time.Time     `json:"verified_at"`
}

func NewStore(path string) Store { return Store{path: path} }
func (s Store) Path() string     { return s.path }

func (s Store) Lock(ctx context.Context) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, fmt.Errorf("create config directory for lock: %w", err)
	}
	guard := flock.New(s.path + ".lock")
	locked, err := guard.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("acquire config lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("config is busy; another AIGW mutation is running")
	}
	return guard.Unlock, nil
}

func (s Store) Load() (domain.Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.NewConfig(), nil
	}
	if err != nil {
		return domain.Config{}, fmt.Errorf("read config: %w", err)
	}
	return decodeTOMLConfig(data)
}

func (s Store) Save(cfg domain.Config) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("refuse invalid config: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if current, err := os.ReadFile(s.path); err == nil {
		if err := writeAtomic(s.path+".bak", current, 0o600); err != nil {
			return fmt.Errorf("back up current config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read current config for backup: %w", err)
	}
	return writeAtomic(s.path, data, 0o600)
}

func (s Store) SaveVerifiedCheckpoint(cfg domain.Config, clients []string) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("refuse invalid verified checkpoint: %w", err)
	}
	checkpoint := VerifiedCheckpoint{
		Config:     cfg,
		Clients:    append([]string(nil), clients...),
		VerifiedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verified checkpoint: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory for verified checkpoint: %w", err)
	}
	return writeAtomic(s.path+".verified.json", append(data, '\n'), 0o600)
}

func (s Store) LoadVerifiedCheckpoint() (VerifiedCheckpoint, error) {
	data, err := os.ReadFile(s.path + ".verified.json")
	if err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("read verified checkpoint: %w", err)
	}
	var checkpoint VerifiedCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("parse verified checkpoint: %w", err)
	}
	var legacy struct {
		Config legacyJSONConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("parse legacy verified checkpoint fields: %w", err)
	}
	canonical, err := promoteLegacyProfiles(checkpoint.Config, legacy.Config.Profiles)
	if err != nil {
		return VerifiedCheckpoint{}, err
	}
	checkpoint.Config = canonical
	if checkpoint.VerifiedAt.IsZero() || len(checkpoint.Clients) == 0 {
		return VerifiedCheckpoint{}, errors.New("verified checkpoint is incomplete")
	}
	return checkpoint, nil
}

func (s Store) LoadBackup() (domain.Config, error) {
	data, err := os.ReadFile(s.path + ".bak")
	if err != nil {
		return domain.Config{}, fmt.Errorf("read previous config backup: %w", err)
	}
	return decodeTOMLConfig(data)
}

func decodeTOMLConfig(data []byte) (domain.Config, error) {
	var cfg domain.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return domain.Config{}, fmt.Errorf("parse config: %w", err)
	}
	var legacy legacyTOMLConfig
	if err := toml.Unmarshal(data, &legacy); err != nil {
		return domain.Config{}, fmt.Errorf("parse legacy config fields: %w", err)
	}
	return promoteLegacyProfiles(cfg, legacy.Profiles)
}

func promoteLegacyProfiles(cfg domain.Config, legacy map[string]legacyProfile) (domain.Config, error) {
	cfg.Normalize()
	for name, source := range legacy {
		profile, exists := cfg.Profiles[name]
		if !exists {
			continue
		}
		if profile.Account != "" || (source.Endpoints.OpenAIResponses == "" && source.Endpoints.Anthropic == "" && source.AccountProbe == nil) {
			continue
		}
		account := cfg.Accounts[name]
		if account.Label == "" {
			account.Label = profile.Label
		}
		account.Endpoints = source.Endpoints
		account.AccountProbe = source.AccountProbe
		cfg.Accounts[name] = account
		profile.Account = name
		cfg.Profiles[name] = profile
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.toml.*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("secure temporary config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config atomically: %w", err)
	}
	return os.Chmod(path, mode)
}
