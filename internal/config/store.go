package config

import (
	"context"
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
	var cfg domain.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return domain.Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
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
