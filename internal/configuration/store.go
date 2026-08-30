package configuration

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

	"aigw-cli/internal/transaction"
	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
)

type Store struct{ path string }

// Snapshot captures the exact config and one-version backup state around a
// configuration mutation. It is intentionally file-level because an empty
// pre-setup state is not a valid  Config and cannot be restored through
// Save.
type Snapshot struct {
	Config transaction.FileSnapshot
	Backup transaction.FileSnapshot
}

type VerifiedBackupSnapshot struct {
	Config   transaction.FileSnapshot
	Backup   transaction.FileSnapshot
	Verified transaction.FileSnapshot
}

type VerifiedBackupState struct {
	Snapshot   VerifiedBackupSnapshot
	Current    Config
	Checkpoint VerifiedCheckpoint
}

// VerifiedCheckpoint is a secret-free record written only after all requested
// client protocol verifications succeed. It is suitable for rollback, not for
// credential recovery.
type VerifiedCheckpoint struct {
	Config     Config    `json:"config"`
	Clients    []string  `json:"clients"`
	VerifiedAt time.Time `json:"verified_at"`
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

func (s Store) CaptureVerifiedBackupState() (VerifiedBackupState, error) {
	configSnapshot, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	if !configSnapshot.Exists {
		return VerifiedBackupState{}, fmt.Errorf("current config is unavailable: %w", os.ErrNotExist)
	}
	backupSnapshot, err := transaction.CaptureFileSnapshot(s.path + ".bak")
	if err != nil {
		return VerifiedBackupState{}, err
	}
	verifiedSnapshot, err := transaction.CaptureFileSnapshot(s.path + ".verified.json")
	if err != nil {
		return VerifiedBackupState{}, err
	}
	if !verifiedSnapshot.Exists {
		return VerifiedBackupState{}, fmt.Errorf("verified checkpoint is unavailable: %w", os.ErrNotExist)
	}
	current, err := decodeTOMLConfig(configSnapshot.Data)
	if err != nil {
		return VerifiedBackupState{}, fmt.Errorf("decode current config snapshot: %w", err)
	}
	checkpoint, err := decodeVerifiedCheckpoint(verifiedSnapshot.Data)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	return VerifiedBackupState{
		Snapshot: VerifiedBackupSnapshot{
			Config:   configSnapshot,
			Backup:   backupSnapshot,
			Verified: verifiedSnapshot,
		},
		Current:    current,
		Checkpoint: checkpoint,
	}, nil
}

func (s Store) ConvergeVerifiedBackup(expected VerifiedBackupSnapshot) (VerifiedBackupSnapshot, error) {
	currentConfig, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return VerifiedBackupSnapshot{}, err
	}
	if !sameFileSnapshot(currentConfig, expected.Config) {
		return VerifiedBackupSnapshot{}, errors.New("config preimage changed; refusing to converge backup")
	}
	currentVerified, err := transaction.CaptureFileSnapshot(s.path + ".verified.json")
	if err != nil {
		return VerifiedBackupSnapshot{}, err
	}
	if !sameFileSnapshot(currentVerified, expected.Verified) {
		return VerifiedBackupSnapshot{}, errors.New("verified checkpoint preimage changed; refusing to converge backup")
	}
	if _, err := transaction.WriteFileAtomicIfUnchanged(s.path+".bak", expected.Backup, expected.Config.Data, 0o600); err != nil {
		return VerifiedBackupSnapshot{}, fmt.Errorf("converge verified config backup: %w", err)
	}
	if err := os.Chmod(s.path+".bak", 0o600); err != nil {
		return VerifiedBackupSnapshot{}, fmt.Errorf("secure verified config backup: %w", err)
	}
	currentBackup, err := transaction.CaptureFileSnapshot(s.path + ".bak")
	if err != nil {
		return VerifiedBackupSnapshot{}, err
	}
	return VerifiedBackupSnapshot{Config: currentConfig, Backup: currentBackup, Verified: currentVerified}, nil
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

func (s Store) Load() (Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return NewConfig(), nil
	}
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeTOMLConfig(data)
}

func (s Store) Save(cfg Config) error {
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
		if err := transaction.WriteFileAtomicExactMode(s.path+".bak", current, 0o600); err != nil {
			return fmt.Errorf("back up current config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read current config for backup: %w", err)
	}
	return transaction.WriteFileAtomicExactMode(s.path, data, 0o600)
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

func (s Store) SaveVerifiedCheckpoint(cfg Config, clients []string) error {
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
	return transaction.WriteFileAtomicExactMode(s.path+".verified.json", append(data, '\n'), 0o600)
}

func (s Store) LoadVerifiedCheckpoint() (VerifiedCheckpoint, error) {
	data, err := os.ReadFile(s.path + ".verified.json")
	if err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("read verified checkpoint: %w", err)
	}
	return decodeVerifiedCheckpoint(data)
}

func decodeVerifiedCheckpoint(data []byte) (VerifiedCheckpoint, error) {
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

func (s Store) LoadBackup() (Config, error) {
	data, err := os.ReadFile(s.path + ".bak")
	if err != nil {
		return Config{}, fmt.Errorf("read previous config backup: %w", err)
	}
	return decodeTOMLConfig(data)
}

func decodeTOMLConfig(data []byte) (Config, error) {
	var header struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &header); err != nil {
		return Config{}, newLoadError(LoadPhaseParse, err)
	}
	if header.Version == 2 {
		var legacy versionTwoConfig
		decoder := toml.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&legacy); err != nil {
			return Config{}, newLoadError(LoadPhaseParse, err)
		}
		cfg, err := migrateVersionTwoConfig(legacy)
		if err != nil {
			return Config{}, newLoadError(LoadPhaseValidate, err)
		}
		return cfg, nil
	}

	var cfg Config
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, newLoadError(LoadPhaseParse, err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, newLoadError(LoadPhaseValidate, err)
	}
	return cfg, nil
}

type versionTwoConfig struct {
	Version  int                          `toml:"version"`
	Accounts map[string]Account           `toml:"accounts,omitempty"`
	Profiles map[string]versionTwoProfile `toml:"profiles"`
	Routes   versionTwoRoutes             `toml:"routes"`
	Adapters map[string]AdapterConfig     `toml:"adapters,omitempty"`
}

type versionTwoProfile struct {
	Label         string            `toml:"label"`
	Purpose       string            `toml:"purpose,omitempty"`
	Account       string            `toml:"account"`
	Client        string            `toml:"client,omitempty"`
	ModelProvider string            `toml:"model_provider,omitempty"`
	Models        map[string]string `toml:"models,omitempty"`
}

type versionTwoRoutes struct {
	Default   string            `toml:"default"`
	Overrides map[string]string `toml:"overrides,omitempty"`
}

func migrateVersionTwoConfig(legacy versionTwoConfig) (Config, error) {
	cfg := NewConfig()
	cfg.Accounts = legacy.Accounts
	cfg.Adapters = legacy.Adapters
	for profileID, oldProfile := range legacy.Profiles {
		if !IsAdmittedClient(oldProfile.Client) || len(oldProfile.Models) != 1 || strings.TrimSpace(oldProfile.Models[oldProfile.Client]) == "" {
			return Config{}, fmt.Errorf("cannot migrate profile %q because it does not declare exactly one client and model", profileID)
		}
		cfg.Profiles[profileID] = Profile{
			Label:         oldProfile.Label,
			Purpose:       oldProfile.Purpose,
			Account:       oldProfile.Account,
			Client:        oldProfile.Client,
			Model:         oldProfile.Models[oldProfile.Client],
			ModelProvider: oldProfile.ModelProvider,
		}
	}
	for client, profileID := range legacy.Routes.Overrides {
		cfg.Routes[client] = profileID
	}
	if legacy.Routes.Default != "" {
		profile, ok := cfg.Profiles[legacy.Routes.Default]
		if !ok {
			return Config{}, fmt.Errorf("cannot migrate default route because it references unknown profile %q", legacy.Routes.Default)
		}
		if cfg.Routes[profile.Client] == "" {
			cfg.Routes[profile.Client] = legacy.Routes.Default
		}
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func sameFileSnapshot(left, right transaction.FileSnapshot) bool {
	return left.Exists == right.Exists &&
		left.SHA256 == right.SHA256 &&
		left.Mode == right.Mode &&
		bytes.Equal(left.Data, right.Data)
}
