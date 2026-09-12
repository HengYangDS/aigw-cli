package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"aigw-cli/internal/transaction"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
)

// Store owns the canonical configuration file, lock, verified checkpoint, and rollback backup at one path.
type Store struct{ path string }

// Snapshot captures the exact config and one-version backup state around a
// configuration mutation. It is intentionally file-level because an empty
// pre-setup state is not a valid  Config and cannot be restored through
// Save.
type Snapshot struct {
	Config   transaction.FileSnapshot
	Backup   transaction.FileSnapshot
	Verified transaction.FileSnapshot
}

// VerifiedBackupState pairs verified backup bytes with the currently loaded typed configuration.
type VerifiedBackupState struct {
	Snapshot   Snapshot
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

// NewStore binds configuration persistence to one canonical path.
func NewStore(path string) Store { return Store{path: path} }

// Path returns the canonical configuration path owned by the Store.
func (s Store) Path() string { return s.path }

// CaptureSnapshot reads the current configuration and owned recovery files without mutation.
func (s Store) CaptureSnapshot() (Snapshot, error) {
	configSnapshot, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	backupSnapshot, err := transaction.CaptureFileSnapshot(s.path + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	verifiedSnapshot, err := transaction.CaptureFileSnapshot(s.path + ".verified.json")
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Config: configSnapshot, Backup: backupSnapshot, Verified: verifiedSnapshot}, nil
}

// Commit saves one configuration and returns the exact postimage needed for a
// guarded rollback. If postimage observation fails, it restores the prepared
// preimage before returning the error.
func (s Store) Commit(before Snapshot, cfg Config) (Snapshot, error) {
	data, err := encodeConfig(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	backupAfter := before.Backup
	if before.Config.Exists {
		backupAfter, err = writeConfigurationFileIfUnchanged(s.path+".bak", before.Backup, before.Config.Data, 0o600)
		if err != nil {
			return Snapshot{}, fmt.Errorf("back up current config: %w", err)
		}
	}
	configAfter, err := writeConfigurationFileIfUnchanged(s.path, before.Config, data, 0o600)
	if err != nil {
		if before.Config.Exists {
			if restoreErr := transaction.RestoreFileAtomicIfPostimage(s.path+".bak", before.Backup, backupAfter); restoreErr != nil {
				return Snapshot{}, fmt.Errorf("write config: %w; restore config backup: %w", err, restoreErr)
			}
		}
		return Snapshot{}, fmt.Errorf("write config: %w", err)
	}
	verifiedAfter, err := removeConfigurationFileIfUnchanged(s.path+".verified.json", before.Verified)
	if err == nil {
		return Snapshot{Config: configAfter, Backup: backupAfter, Verified: verifiedAfter}, nil
	}
	failures := []error{fmt.Errorf("invalidate verified checkpoint: %w", err)}
	if restoreErr := transaction.RestoreFileAtomicIfPostimage(s.path, before.Config, configAfter); restoreErr != nil {
		failures = append(failures, fmt.Errorf("restore config: %w", restoreErr))
	}
	if before.Config.Exists {
		if restoreErr := transaction.RestoreFileAtomicIfPostimage(s.path+".bak", before.Backup, backupAfter); restoreErr != nil {
			failures = append(failures, fmt.Errorf("restore config backup: %w", restoreErr))
		}
	}
	return Snapshot{}, errors.Join(failures...)
}

// CaptureVerifiedBackupState validates that current configuration still matches its verified checkpoint before returning recovery state.
func (s Store) CaptureVerifiedBackupState() (VerifiedBackupState, error) {
	snapshot, err := s.CaptureSnapshot()
	if err != nil {
		return VerifiedBackupState{}, err
	}
	if !snapshot.Config.Exists {
		return VerifiedBackupState{}, fmt.Errorf("current config is unavailable: %w", os.ErrNotExist)
	}
	if !snapshot.Verified.Exists {
		return VerifiedBackupState{}, fmt.Errorf("verified checkpoint is unavailable: %w", os.ErrNotExist)
	}
	current, err := decodeTOMLConfig(snapshot.Config.Data)
	if err != nil {
		return VerifiedBackupState{}, fmt.Errorf("decode current config snapshot: %w", err)
	}
	checkpoint, err := decodeVerifiedCheckpoint(snapshot.Verified.Data)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	currentData, currentErr := encodeConfig(current)
	verifiedData, verifiedErr := encodeConfig(checkpoint.Config)
	if err := errors.Join(currentErr, verifiedErr); err != nil {
		return VerifiedBackupState{}, err
	}
	if !bytes.Equal(currentData, verifiedData) {
		return VerifiedBackupState{}, errors.New("verified checkpoint does not match current configuration")
	}
	return VerifiedBackupState{Snapshot: snapshot, Current: current, Checkpoint: checkpoint}, nil
}

// ConvergeVerifiedBackup makes the verified backup match an expected snapshot without altering unrelated files.
func (s Store) ConvergeVerifiedBackup(expected Snapshot) error {
	currentConfig, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return err
	}
	if !currentConfig.Equal(expected.Config) {
		return errors.New("config preimage changed; refusing to converge backup")
	}
	currentVerified, err := transaction.CaptureFileSnapshot(s.path + ".verified.json")
	if err != nil {
		return err
	}
	if !currentVerified.Equal(expected.Verified) {
		return errors.New("verified checkpoint preimage changed; refusing to converge backup")
	}
	if _, err := transaction.WriteFileAtomicExactModeIfUnchanged(s.path+".bak", expected.Backup, expected.Config.Data, 0o600); err != nil {
		return fmt.Errorf("converge verified config backup: %w", err)
	}
	return nil
}

// RestoreSnapshot independently restores configuration, backup and checkpoint
// while each still matches its written postimage. Conflicts preserve newer
// state without suppressing other restoration attempts or their errors.
func (s Store) RestoreSnapshot(before, after Snapshot) error {
	var failures []error
	if err := transaction.RestoreFileAtomicIfPostimage(s.path, before.Config, after.Config); err != nil {
		failures = append(failures, fmt.Errorf("restore config snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.path+".bak", before.Backup, after.Backup); err != nil {
		failures = append(failures, fmt.Errorf("restore config backup snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.path+".verified.json", before.Verified, after.Verified); err != nil {
		failures = append(failures, fmt.Errorf("restore verified checkpoint snapshot: %w", err))
	}
	return errors.Join(failures...)
}

// Lock acquires the bounded configuration mutation lock and returns its release function.
func (s Store) Lock(ctx context.Context) (func() error, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
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

// Load reads, parses, normalizes, and validates the current configuration.
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

// Save validates and atomically persists configuration while maintaining its recovery boundary.
func (s Store) Save(cfg Config) error {
	before, err := s.CaptureSnapshot()
	if err != nil {
		return fmt.Errorf("capture current config: %w", err)
	}
	_, err = s.Commit(before, cfg)
	return err
}

var writeConfigurationFileIfUnchanged = transaction.WriteFileAtomicExactModeIfUnchanged
var removeConfigurationFileIfUnchanged = transaction.RemoveFileIfUnchanged

func encodeConfig(cfg Config) ([]byte, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("refuse invalid config: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return separateTOMLTableBlocks(data), nil
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

// SaveVerifiedCheckpoint records a completed verification only while its
// configuration remains current. It serializes with AIGW mutations, preserves
// newer checkpoints, and compensates observed external configuration changes.
func (s Store) SaveVerifiedCheckpoint(ctx context.Context, cfg Config, clients []string) (resultErr error) {
	if err := validateCheckpointClients(clients); err != nil {
		return err
	}
	expected, err := encodeConfig(cfg)
	if err != nil {
		return err
	}
	unlock, err := s.Lock(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := unlock(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("release checkpoint lock: %w", err))
		}
	}()
	configBefore, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return err
	}
	if !configBefore.Exists {
		return errors.New("configuration is unavailable; verification checkpoint was not saved")
	}
	current, err := decodeTOMLConfig(configBefore.Data)
	if err != nil {
		return err
	}
	currentData, err := encodeConfig(current)
	if err != nil {
		return err
	}
	if !bytes.Equal(expected, currentData) {
		return errors.New("configuration changed during verification; run `aigw verify --for all` again")
	}
	checkpointPath := s.path + ".verified.json"
	checkpointBefore, err := transaction.CaptureFileSnapshot(checkpointPath)
	if err != nil {
		return err
	}
	checkpoint := VerifiedCheckpoint{
		Config:     current,
		Clients:    append([]string(nil), clients...),
		VerifiedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verified checkpoint: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	checkpointAfter, err := writeConfigurationFileIfUnchanged(checkpointPath, checkpointBefore, append(data, '\n'), 0o600)
	if err != nil {
		return err
	}
	configAfter, err := transaction.CaptureFileSnapshot(s.path)
	if err == nil && configBefore.Equal(configAfter) {
		return nil
	}
	if err == nil {
		err = errors.New("configuration changed during checkpoint commit; run `aigw verify --for all` again")
	}
	rollbackErr := transaction.RestoreFileAtomicIfPostimage(checkpointPath, checkpointBefore, checkpointAfter)
	return errors.Join(err, rollbackErr)
}

// LoadVerifiedCheckpoint returns the last complete verified configuration checkpoint.
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
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return VerifiedCheckpoint{}, errors.New("parse verified checkpoint: expected one complete JSON document")
	}
	if checkpoint.VerifiedAt.IsZero() {
		return VerifiedCheckpoint{}, errors.New("verified checkpoint is incomplete")
	}
	if err := validateCheckpointClients(checkpoint.Clients); err != nil {
		return VerifiedCheckpoint{}, err
	}
	checkpoint.Config.Normalize()
	if err := checkpoint.Config.Validate(); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("validate verified checkpoint: %w", err)
	}
	return checkpoint, nil
}

func validateCheckpointClients(clients []string) error {
	if len(clients) == 0 {
		return errors.New("verified checkpoint is incomplete: no clients")
	}
	for index, client := range clients {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("verified checkpoint contains unknown client %q", client)
		}
		if slices.Contains(clients[:index], client) {
			return fmt.Errorf("verified checkpoint repeats client %q", client)
		}
	}
	return nil
}

// LoadBackup returns the validated rollback configuration retained by the Store.
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
	if header.Version != ConfigVersion {
		return Config{}, newLoadError(LoadPhaseValidate, &UnsupportedConfigVersionError{
			Version:         header.Version,
			ExpectedVersion: ConfigVersion,
		})
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
