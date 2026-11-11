// Package store persists AIGW's configuration in one file, aigw.toml, with
// its backup, its verified snapshot and its lock, and reads and writes team
// files, which have the configuration's form without clients.
package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/gofrs/flock"
)

// Store owns the configuration file, its lock, verified snapshot and
// rollback backup at one path, and decodes them for the clients it admits.
type Store struct {
	path      string
	admission domain.Admission
}

// Snapshot captures the exact configuration, backup and verified snapshot
// around a configuration mutation. It is file-level because an empty
// pre-setup state is not a valid Config and cannot be restored through a
// save.
type Snapshot struct {
	Config   platform.FileSnapshot
	Backup   platform.FileSnapshot
	Verified platform.FileSnapshot
}

// NewStore binds configuration persistence to one configuration file and the
// clients that its configurations admit.
func NewStore(path string, admission domain.Admission) Store {
	return Store{path: path, admission: admission}
}

// Path returns the configuration file that the Store owns.
func (s Store) Path() string { return s.path }

// Admission returns the clients that the Store's configurations admit.
func (s Store) Admission() domain.Admission { return s.admission }

// backupPath is the backup that each change keeps of the configuration it
// replaces, aigw.toml.bak.
func (s Store) backupPath() string { return s.path + ".bak" }

// verifiedPath is the verified snapshot beside the configuration,
// aigw.verified.json.
func (s Store) verifiedPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".verified.json"
}

// CaptureSnapshot reads the current configuration and owned recovery files without mutation.
func (s Store) CaptureSnapshot() (Snapshot, error) {
	configSnapshot, err := platform.CaptureFileSnapshot(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	backupSnapshot, err := platform.CaptureFileSnapshot(s.backupPath())
	if err != nil {
		return Snapshot{}, err
	}
	verifiedSnapshot, err := platform.CaptureFileSnapshot(s.verifiedPath())
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Config: configSnapshot, Backup: backupSnapshot, Verified: verifiedSnapshot}, nil
}

// Commit saves one configuration and returns the exact postimage needed for a
// guarded rollback. If postimage observation fails, it restores the prepared
// preimage before returning the error.
func (s Store) Commit(before Snapshot, cfg domain.Config) (Snapshot, error) {
	data, err := encodeConfig(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	if before.Config.Exists && configurationDataEquivalent(before.Config.Data, data, s.admission) {
		current, err := s.CaptureSnapshot()
		if err != nil {
			return Snapshot{}, err
		}
		if !current.equal(before) {
			return Snapshot{}, errors.New("configuration preimage changed; refusing unchanged commit")
		}
		return before, nil
	}
	after := before
	if before.Config.Exists {
		backup, err := writeConfigurationFileIfUnchanged(s.backupPath(), before.Backup, before.Config.Data, 0o600)
		if err != nil {
			return Snapshot{}, fmt.Errorf("back up current config: %w", err)
		}
		after.Backup = backup
	}
	config, err := writeConfigurationFileIfUnchanged(s.path, before.Config, data, 0o600)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write config: %w", err), s.RestoreSnapshot(before, after))
	}
	after.Config = config
	verified, err := removeConfigurationFileIfUnchanged(s.verifiedPath(), before.Verified)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("invalidate verified checkpoint: %w", err), s.RestoreSnapshot(before, after))
	}
	after.Verified = verified
	current, err := s.CaptureSnapshot()
	if err != nil || !current.equal(after) {
		return Snapshot{}, errors.Join(errors.New("configuration changed during commit"), err, s.RestoreSnapshot(before, after))
	}
	return after, nil
}

func (snapshot Snapshot) equal(other Snapshot) bool {
	return snapshot.Config.Equal(other.Config) && snapshot.Backup.Equal(other.Backup) && snapshot.Verified.Equal(other.Verified)
}

// MatchesConfiguration binds an earlier assembled selection to this exact
// configuration preimage, including the empty pre-setup state.
func (snapshot Snapshot) MatchesConfiguration(expected domain.Config) (bool, error) {
	if !snapshot.Config.Exists {
		expected = expected.Clone()
		expected.Normalize()
		return reflect.DeepEqual(expected, domain.NewConfig(expected.Admission())), nil
	}
	current, err := decodeTOMLConfig(snapshot.Config.Data, expected.Admission())
	if err != nil {
		return false, err
	}
	return configurationsEqual(current, expected)
}

func configurationDataEquivalent(before, after []byte, admission domain.Admission) bool {
	if bytes.Equal(before, after) {
		return true
	}
	previous, previousErr := decodeTOMLConfig(before, admission)
	next, nextErr := decodeTOMLConfig(after, admission)
	return previousErr == nil && nextErr == nil && reflect.DeepEqual(previous, next)
}

// configurationsEqual reports whether two configurations are the same file.
func configurationsEqual(left, right domain.Config) (bool, error) {
	leftData, leftErr := encodeConfig(left)
	rightData, rightErr := encodeConfig(right)
	if err := errors.Join(leftErr, rightErr); err != nil {
		return false, err
	}
	return bytes.Equal(leftData, rightData), nil
}

// RestoreSnapshot independently restores configuration, backup and checkpoint
// while each still matches its written postimage. Conflicts preserve newer
// state without suppressing other restoration attempts or their errors.
func (s Store) RestoreSnapshot(before, after Snapshot) error {
	var failures []error
	if err := platform.RestoreFileAtomicIfPostimage(s.path, before.Config, after.Config); err != nil {
		failures = append(failures, fmt.Errorf("restore config snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.backupPath(), before.Backup, after.Backup); err != nil {
		failures = append(failures, fmt.Errorf("restore config backup snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.verifiedPath(), before.Verified, after.Verified); err != nil {
		failures = append(failures, fmt.Errorf("restore verified checkpoint snapshot: %w", err))
	}
	return errors.Join(failures...)
}

// ErrConfigurationBusy reports that another AIGW command held the
// configuration lock for the whole bounded wait.
var ErrConfigurationBusy = errors.New("config is busy; another AIGW mutation is running")

// Lock acquires the bounded configuration mutation lock and returns its release function.
func (s Store) Lock(ctx context.Context) (func() error, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, fmt.Errorf("create config directory for lock: %w", err)
	}
	guard := flock.New(s.path + ".lock")
	locked, err := guard.TryLockContext(ctx, 50*time.Millisecond)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("%w: %w", ErrConfigurationBusy, err)
	}
	if err != nil {
		return nil, fmt.Errorf("acquire config lock: %w", err)
	}
	if !locked {
		return nil, ErrConfigurationBusy
	}
	return guard.Unlock, nil
}

// Load reads, parses, normalizes, and validates the current configuration. A
// home without aigw.toml has no configuration, whatever else it holds.
func (s Store) Load() (domain.Config, error) {
	data, err := os.ReadFile(s.path)
	// A file in the way or a link that leads nowhere, at the configuration or
	// at its folder, is no first run: a setup would write through it, or
	// replace the link with a new file.
	if failure := platform.ClassifyPath(s.path, platform.FileExpected, err); failure.Blocked() || failure.Absent() {
		if failure.Blocked() {
			return domain.Config{}, newLoadError(LoadPhaseRead, failure)
		}
		return domain.NewConfig(s.admission), nil
	}
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeTOMLConfig(data, s.admission)
}

// Save validates and atomically persists configuration while maintaining its recovery boundary.
func (s Store) Save(cfg domain.Config) error {
	before, err := s.CaptureSnapshot()
	if err != nil {
		return fmt.Errorf("capture current config: %w", err)
	}
	_, err = s.Commit(before, cfg)
	return err
}

// RollbackConfiguration returns the configuration file that a program
// rollback gives the retained program to read, or nothing without one.
func (s Store) RollbackConfiguration() ([]byte, error) {
	snapshot, err := platform.CaptureFileSnapshot(s.path)
	if err != nil || !snapshot.Exists {
		return nil, err
	}
	return snapshot.Data, nil
}

var writeConfigurationFileIfUnchanged = platform.WriteFileAtomicExactModeIfUnchanged
var removeConfigurationFileIfUnchanged = platform.RemoveFileIfUnchanged

func encodeConfig(cfg domain.Config) ([]byte, error) {
	cfg = cfg.Clone()
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("refuse invalid config: %w", err)
	}
	data, err := encodeDocument(newDocument(cfg))
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return data, nil
}

// LoadBackup returns the validated rollback configuration retained by the Store.
func (s Store) LoadBackup() (domain.Config, error) {
	data, err := os.ReadFile(s.backupPath())
	if err != nil {
		return domain.Config{}, fmt.Errorf("read previous config backup: %w", err)
	}
	return decodeTOMLConfig(data, s.admission)
}

// decodeTOMLConfig decodes a schema 8 configuration: another version is
// refused by its number before any other field is read, and a field that
// schema 8 does not define by its name.
func decodeTOMLConfig(data []byte, admission domain.Admission) (domain.Config, error) {
	version, err := readVersion(data)
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseParse, err)
	}
	if version != domain.ConfigVersion {
		return domain.Config{}, newLoadError(LoadPhaseValidate, &domain.UnsupportedConfigVersionError{
			Version:         version,
			ExpectedVersion: domain.ConfigVersion,
		})
	}
	decoded, err := decodeDocument(data)
	if _, undefined := errors.AsType[*UndefinedFieldError](err); undefined {
		return domain.Config{}, newLoadError(LoadPhaseValidate, err)
	}
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseParse, err)
	}
	cfg, err := decoded.config(admission)
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseValidate, err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseValidate, err)
	}
	return cfg, nil
}
