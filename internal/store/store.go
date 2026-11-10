// Package store persists AIGW's configuration: the canonical file and its
// components, verified checkpoints, backups, and the TOML form of the team
// manifest.
package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
)

// Store owns the canonical configuration file, lock, verified checkpoint, and
// rollback backup at one path, and decodes them for the clients it admits.
type Store struct {
	path      string
	admission domain.Admission
}

// Snapshot captures the exact config and one-version backup state around a
// configuration mutation. It is intentionally file-level because an empty
// pre-setup state is not a valid  Config and cannot be restored through
// Save.
type Snapshot struct {
	Config   platform.FileSnapshot
	Backup   platform.FileSnapshot
	Verified platform.FileSnapshot
	// components and backups are the optional components' two generations.
	components componentSnapshots
	backups    componentSnapshots
}

// NewStore binds configuration persistence to one canonical path and the
// clients that its configurations admit.
func NewStore(path string, admission domain.Admission) Store {
	return Store{path: path, admission: admission}
}

// Path returns the canonical configuration path owned by the Store.
func (s Store) Path() string { return s.path }

// Admission returns the clients that the Store's configurations admit.
func (s Store) Admission() domain.Admission { return s.admission }

// CaptureSnapshot reads the current configuration and owned recovery files without mutation.
func (s Store) CaptureSnapshot() (Snapshot, error) {
	configSnapshot, err := platform.CaptureFileSnapshot(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	backupSnapshot, err := platform.CaptureFileSnapshot(s.path + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	verifiedSnapshot, err := platform.CaptureFileSnapshot(s.path + ".verified.json")
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Config: configSnapshot, Backup: backupSnapshot, Verified: verifiedSnapshot}
	for index, component := range components {
		current, err := platform.CaptureFileSnapshot(s.componentPath(index))
		if err != nil && !component.bothGenerations {
			return Snapshot{}, err
		}
		backup, backupErr := platform.CaptureFileSnapshot(s.componentPath(index) + ".bak")
		if failure := errors.Join(err, backupErr); failure != nil {
			return Snapshot{}, failure
		}
		snapshot.components[index], snapshot.backups[index] = current, backup
	}
	return snapshot, nil
}

// Commit saves one configuration and returns the exact postimage needed for a
// guarded rollback. If postimage observation fails, it restores the prepared
// preimage before returning the error.
func (s Store) Commit(before Snapshot, cfg domain.Config) (Snapshot, error) {
	data, err := encodeConfig(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	forwarding, err := encodeForwarding(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	catalog, catalogErr := encodeCatalogOrder(cfg)
	capabilities, capabilitiesErr := encodeModelCapabilities(cfg)
	if err = errors.Join(catalogErr, capabilitiesErr); err != nil {
		return Snapshot{}, err
	}
	retries, err := encodeRetryPolicies(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	return s.commitComponents(before, data, [componentCount][]byte{
		forwardingComponent: forwarding, retriesComponent: retries, catalogComponent: catalog, capabilitiesComponent: capabilities,
	})
}

func (s Store) commitComponents(before Snapshot, data []byte, next [componentCount][]byte) (Snapshot, error) {
	if before.Config.Exists && configurationDataEquivalent(before.Config.Data, data, s.admission) && componentsUnchanged(before.components, next) {
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
		backedUp, err := s.backUp(before)
		if err != nil {
			return Snapshot{}, err
		}
		after = backedUp
	}
	if !before.Config.Exists || !configurationDataEquivalent(before.Config.Data, data, s.admission) {
		config, err := writeConfigurationFileIfUnchanged(s.path, before.Config, data, 0o600)
		if err != nil {
			return Snapshot{}, errors.Join(fmt.Errorf("write config: %w", err), s.RestoreSnapshot(before, after))
		}
		after.Config = config
	}
	for index, component := range components {
		written, err := s.writeOptional(s.componentPath(index), before.components[index], next[index])
		if err != nil {
			return Snapshot{}, errors.Join(fmt.Errorf("write %s: %w", component.whole, err), s.RestoreSnapshot(before, after))
		}
		after.components[index] = written
	}
	verified, err := removeConfigurationFileIfUnchanged(s.path+".verified.json", before.Verified)
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

// MatchesConfiguration binds an earlier assembled selection to these exact
// canonical and component preimages, including the empty pre-setup state.
func (snapshot Snapshot) MatchesConfiguration(expected domain.Config) (bool, error) {
	if !snapshot.Config.Exists {
		if slices.ContainsFunc(snapshot.components[:], func(component platform.FileSnapshot) bool { return component.Exists }) {
			return false, errors.New("configuration component has no canonical configuration")
		}
		expected = expected.Clone()
		expected.Normalize()
		return reflect.DeepEqual(expected, domain.NewConfig(expected.Admission())), nil
	}
	current, err := (Store{admission: expected.Admission()}).loadSnapshot(snapshot)
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

// RestoreSnapshot independently restores configuration, backup and checkpoint
// while each still matches its written postimage. Conflicts preserve newer
// state without suppressing other restoration attempts or their errors.
func (s Store) RestoreSnapshot(before, after Snapshot) error {
	var failures []error
	if err := platform.RestoreFileAtomicIfPostimage(s.path, before.Config, after.Config); err != nil {
		failures = append(failures, fmt.Errorf("restore config snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.path+".bak", before.Backup, after.Backup); err != nil {
		failures = append(failures, fmt.Errorf("restore config backup snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.path+".verified.json", before.Verified, after.Verified); err != nil {
		failures = append(failures, fmt.Errorf("restore verified checkpoint snapshot: %w", err))
	}
	for _, index := range restoreOrder {
		if err := platform.RestoreFileAtomicIfPostimage(s.componentPath(index), before.components[index], after.components[index]); err != nil {
			failures = append(failures, fmt.Errorf("restore %s snapshot: %w", components[index].record, err))
		}
		if err := platform.RestoreFileAtomicIfPostimage(s.componentPath(index)+".bak", before.backups[index], after.backups[index]); err != nil {
			failures = append(failures, fmt.Errorf("restore %s: %w", components[index].restoredBackup, err))
		}
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

// Load reads, parses, normalizes, and validates the current configuration.
func (s Store) Load() (domain.Config, error) {
	cfg, err := s.LoadCredentialConfig()
	if err != nil {
		return domain.Config{}, err
	}
	return s.withComponents(cfg, "")
}

// LoadCredentialConfig reads only the routing and credential authority. Display
// metadata and later Route capabilities cannot interrupt a cached native
// credential reader.
func (s Store) LoadCredentialConfig() (domain.Config, error) {
	data, err := os.ReadFile(s.path)
	// A file in the way or a link that leads nowhere, at the configuration or
	// at its folder, is no first run: a setup would write through it, or
	// replace the link with a new file.
	if failure := platform.ClassifyPath(s.path, platform.FileExpected, err); failure.Blocked() || failure.Absent() {
		if failure.Blocked() {
			return domain.Config{}, newLoadError(LoadPhaseRead, failure)
		}
		forwarding, forwardingErr := platform.CaptureFileSnapshot(s.componentPath(forwardingComponent))
		if forwardingErr != nil || forwarding.Exists {
			return domain.Config{}, newLoadError(LoadPhaseRead, errors.Join(errors.New("client forwarding has no canonical configuration"), forwardingErr))
		}
		return domain.NewConfig(s.admission), nil
	}
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	forwarding, err := platform.CaptureFileSnapshot(s.componentPath(forwardingComponent))
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeStoreConfig(data, forwarding, s.admission)
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

var writeConfigurationFileIfUnchanged = platform.WriteFileAtomicExactModeIfUnchanged
var removeConfigurationFileIfUnchanged = platform.RemoveFileIfUnchanged

func encodeConfig(cfg domain.Config) ([]byte, error) {
	cfg.Normalize()
	cfg = cfg.Clone()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("refuse invalid config: %w", err)
	}
	for id, account := range cfg.Providers {
		account.Order = 0
		cfg.Providers[id] = account
	}
	for id, model := range cfg.Families {
		model.Order = 0
		cfg.Families[id] = model
	}
	keepPublishedCapabilities(cfg)
	for client, binding := range cfg.Clients {
		binding.RetryPolicy = nil
		cfg.Clients[client] = binding
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

// LoadBackup returns the validated rollback configuration retained by the Store.
func (s Store) LoadBackup() (domain.Config, error) {
	data, err := os.ReadFile(s.path + ".bak")
	if err != nil {
		return domain.Config{}, fmt.Errorf("read previous config backup: %w", err)
	}
	forwarding, err := platform.CaptureFileSnapshot(s.componentPath(forwardingComponent) + ".bak")
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	cfg, err := decodeStoreConfig(data, forwarding, s.admission)
	if err != nil {
		return domain.Config{}, err
	}
	return s.withComponents(cfg, ".bak")
}

func decodeTOMLConfig(data []byte, admission domain.Admission) (domain.Config, error) {
	var header struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &header); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseParse, err)
	}
	if header.Version != domain.ConfigVersion {
		return domain.Config{}, newLoadError(LoadPhaseValidate, &domain.UnsupportedConfigVersionError{
			Version:         header.Version,
			ExpectedVersion: domain.ConfigVersion,
		})
	}

	var cfg domain.Config
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseParse, err)
	}
	cfg.SetAdmission(admission)
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, newLoadError(LoadPhaseValidate, err)
	}
	return cfg, nil
}
