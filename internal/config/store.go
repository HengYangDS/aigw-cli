package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

type Store struct{ path string }

// Snapshot captures the exact config and one-version backup state around a
// configuration mutation. It is intentionally file-level because an empty
// pre-setup state is not a valid domain.Config and cannot be restored through
// Save.
type Snapshot struct {
	Config transaction.FileSnapshot
	Backup transaction.FileSnapshot
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

func (s Store) CaptureSnapshot() (Snapshot, error) {
	configSnapshot, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	backupSnapshot, err := transaction.CaptureFileSnapshot(s.path + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Config: configSnapshot, Backup: backupSnapshot}, nil
}

// RestoreSnapshot restores a captured config and backup only if both files
// still equal the postimages produced by the current mutation. This protects a
// newer external writer from being overwritten during compensation.
func (s Store) RestoreSnapshot(before, after Snapshot) error {
	if err := transaction.RestoreFileAtomicIfPostimage(s.path, before.Config, after.Config); err != nil {
		return fmt.Errorf("restore config snapshot: %w", err)
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.path+".bak", before.Backup, after.Backup); err != nil {
		return fmt.Errorf("restore config backup snapshot: %w", err)
	}
	return nil
}

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
	data = separateTOMLTableBlocks(data)
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

// separateTOMLTableBlocks inserts exactly one separator before each generated
// table after the first. TOML ignores blank lines, so this is a presentation-
// only normalization of AIGW-owned output.
func separateTOMLTableBlocks(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	formatted := make([]string, 0, len(lines)+8)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && len(formatted) > 0 {
			previous := strings.TrimSpace(formatted[len(formatted)-1])
			if previous != "" && !strings.HasPrefix(previous, "#") {
				formatted = append(formatted, "")
			}
		}
		formatted = append(formatted, line)
	}
	return []byte(strings.Join(formatted, "\n"))
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
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&checkpoint); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("parse verified checkpoint: %w", err)
	}
	if checkpoint.VerifiedAt.IsZero() || len(checkpoint.Clients) == 0 {
		return VerifiedCheckpoint{}, errors.New("verified checkpoint is incomplete")
	}
	checkpoint.Config.Normalize()
	if err := checkpoint.Config.Validate(); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("validate verified checkpoint: %w", err)
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
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return domain.Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.Normalize()
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
