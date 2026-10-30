// Package store persists AIGW's configuration: the canonical file and its
// components, verified checkpoints, backups, schema migration, and the TOML form
// of the team manifest.
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
	Config             platform.FileSnapshot
	Backup             platform.FileSnapshot
	Forwarding         platform.FileSnapshot
	forwardingBackup   platform.FileSnapshot
	retries            platform.FileSnapshot
	retriesBackup      platform.FileSnapshot
	Catalog            platform.FileSnapshot
	catalogBackup      platform.FileSnapshot
	capabilities       platform.FileSnapshot
	capabilitiesBackup platform.FileSnapshot
	Verified           platform.FileSnapshot
}

func (snapshot Snapshot) equal(other Snapshot) bool {
	return snapshot.Config.Equal(other.Config) && snapshot.Backup.Equal(other.Backup) &&
		snapshot.Forwarding.Equal(other.Forwarding) && snapshot.forwardingBackup.Equal(other.forwardingBackup) &&
		snapshot.Catalog.Equal(other.Catalog) && snapshot.catalogBackup.Equal(other.catalogBackup) &&
		snapshot.capabilities.Equal(other.capabilities) && snapshot.capabilitiesBackup.Equal(other.capabilitiesBackup) &&
		snapshot.retries.Equal(other.retries) && snapshot.retriesBackup.Equal(other.retriesBackup) &&
		snapshot.Verified.Equal(other.Verified)
}

// BackupConverged reports whether every rollback component equals the current
// generation, including intentionally absent optional components.
func (snapshot Snapshot) BackupConverged(mode os.FileMode) bool {
	return snapshot.Backup.Exists && snapshot.Backup.Mode == mode &&
		bytes.Equal(snapshot.Backup.Data, snapshot.Config.Data) &&
		backupComponentConverged(snapshot.Forwarding, snapshot.forwardingBackup, mode) &&
		backupComponentConverged(snapshot.retries, snapshot.retriesBackup, mode) &&
		backupComponentConverged(snapshot.Catalog, snapshot.catalogBackup, mode) &&
		backupComponentConverged(snapshot.capabilities, snapshot.capabilitiesBackup, mode)
}

// VerifiedBackupState pairs current backup bytes with client evidence when any client is enabled.
type VerifiedBackupState struct {
	Snapshot   Snapshot
	Current    domain.Config
	Checkpoint VerifiedCheckpoint
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
	forwarding, err := platform.CaptureFileSnapshot(s.forwardingPath())
	if err != nil {
		return Snapshot{}, err
	}
	forwardingBackup, err := platform.CaptureFileSnapshot(s.forwardingPath() + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	retries, err := platform.CaptureFileSnapshot(s.retriesPath())
	retriesBackup, retriesErr := platform.CaptureFileSnapshot(s.retriesPath() + ".bak")
	if failure := errors.Join(err, retriesErr); failure != nil {
		return Snapshot{}, failure
	}
	catalog, err := platform.CaptureFileSnapshot(s.catalogPath())
	if err != nil {
		return Snapshot{}, err
	}
	catalogBackup, err := platform.CaptureFileSnapshot(s.catalogPath() + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	capabilities, err := platform.CaptureFileSnapshot(s.capabilitiesPath())
	capabilitiesBackup, backupErr := platform.CaptureFileSnapshot(s.capabilitiesPath() + ".bak")
	if failure := errors.Join(err, backupErr); failure != nil {
		return Snapshot{}, failure
	}
	return Snapshot{Config: configSnapshot, Backup: backupSnapshot, Forwarding: forwarding, forwardingBackup: forwardingBackup, retries: retries, retriesBackup: retriesBackup, Catalog: catalog, catalogBackup: catalogBackup, capabilities: capabilities, capabilitiesBackup: capabilitiesBackup, Verified: verifiedSnapshot}, nil
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
	capabilities, capabilitiesErr := encodeRouteCapabilities(cfg)
	if err = errors.Join(catalogErr, capabilitiesErr); err != nil {
		return Snapshot{}, err
	}
	retries, err := encodeRetryPolicies(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	return s.commitComponents(before, data, forwarding, catalog, capabilities, retries)
}

func (s Store) commitData(before Snapshot, data []byte) (Snapshot, error) {
	return s.commitComponents(before, data, nil, nil, nil, nil)
}

func (s Store) commitComponents(before Snapshot, data, forwarding, catalog, capabilities, retries []byte) (Snapshot, error) {
	if before.Config.Exists && configurationDataEquivalent(before.Config.Data, data, s.admission) && componentUnchanged(before.Forwarding, forwarding) &&
		componentUnchanged(before.Catalog, catalog) && componentUnchanged(before.capabilities, capabilities) && componentUnchanged(before.retries, retries) {
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
	forwardingAfter, err := s.writeOptional(s.forwardingPath(), before.Forwarding, forwarding)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write client forwarding: %w", err), s.RestoreSnapshot(before, after))
	}
	after.Forwarding = forwardingAfter
	retriesAfter, err := s.writeOptional(s.retriesPath(), before.retries, retries)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write client retry policies: %w", err), s.RestoreSnapshot(before, after))
	}
	after.retries = retriesAfter
	catalogAfter, err := s.writeOptional(s.catalogPath(), before.Catalog, catalog)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write catalogue presentation: %w", err), s.RestoreSnapshot(before, after))
	}
	after.Catalog = catalogAfter
	capabilitiesAfter, err := s.writeOptional(s.capabilitiesPath(), before.capabilities, capabilities)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write route capabilities: %w", err), s.RestoreSnapshot(before, after))
	}
	after.capabilities = capabilitiesAfter
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
		if snapshot.Forwarding.Exists || snapshot.Catalog.Exists || snapshot.capabilities.Exists || snapshot.retries.Exists {
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

// CaptureVerifiedBackupState requires a current checkpoint only when clients are enabled.
func (s Store) CaptureVerifiedBackupState() (VerifiedBackupState, error) {
	snapshot, err := s.CaptureSnapshot()
	if err != nil {
		return VerifiedBackupState{}, err
	}
	if !snapshot.Config.Exists {
		return VerifiedBackupState{}, fmt.Errorf("current config is unavailable: %w", os.ErrNotExist)
	}
	current, err := s.loadSnapshot(snapshot)
	if err != nil {
		return VerifiedBackupState{}, fmt.Errorf("decode current config snapshot: %w", err)
	}
	if !snapshot.Verified.Exists && len(current.EnabledClientIDs()) == 0 {
		return VerifiedBackupState{Snapshot: snapshot, Current: current}, nil
	}
	if !snapshot.Verified.Exists {
		return VerifiedBackupState{}, fmt.Errorf("verified checkpoint is unavailable: %w", os.ErrNotExist)
	}
	checkpoint, err := decodeVerifiedCheckpoint(snapshot.Verified.Data, s.admission)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	checkpoint.Config, err = decodeComponents(checkpoint.Config, snapshot.Catalog, snapshot.capabilities)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	equal, err := configurationsEqual(current, checkpoint.Config)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	if !equal {
		return VerifiedBackupState{}, errors.New("verified checkpoint does not match current configuration")
	}
	return VerifiedBackupState{Snapshot: snapshot, Current: current, Checkpoint: checkpoint}, nil
}

// ConvergeVerifiedBackup makes the verified backup match an expected snapshot without altering unrelated files.
func (s Store) ConvergeVerifiedBackup(expected Snapshot) error {
	current, err := s.CaptureSnapshot()
	if err != nil {
		return err
	}
	if !current.Config.Equal(expected.Config) {
		return errors.New("config preimage changed; refusing to converge backup")
	}
	if !current.Forwarding.Equal(expected.Forwarding) || !current.forwardingBackup.Equal(expected.forwardingBackup) {
		return errors.New("client forwarding preimage changed; refusing to converge backup")
	}
	if !current.Catalog.Equal(expected.Catalog) || !current.catalogBackup.Equal(expected.catalogBackup) {
		return errors.New("catalogue presentation preimage changed; refusing to converge backup")
	}
	if !current.capabilities.Equal(expected.capabilities) || !current.capabilitiesBackup.Equal(expected.capabilitiesBackup) {
		return errors.New("route capability preimage changed; refusing to converge backup")
	}
	if !current.retries.Equal(expected.retries) || !current.retriesBackup.Equal(expected.retriesBackup) {
		return errors.New("client retry policy preimage changed; refusing to converge backup")
	}
	if !current.Verified.Equal(expected.Verified) {
		return errors.New("verified checkpoint preimage changed; refusing to converge backup")
	}
	preserve, err := s.preserveMigrationRollbackInput(expected.Backup)
	if err != nil {
		return err
	}
	if preserve {
		return nil
	}
	after := expected
	after.Backup, err = writeConfigurationFileIfUnchanged(s.path+".bak", expected.Backup, expected.Config.Data, 0o600)
	if err != nil {
		return fmt.Errorf("converge verified config backup: %w", err)
	}
	forwardingBackup, err := s.writeOptional(s.forwardingPath()+".bak", expected.forwardingBackup, expected.Forwarding.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified forwarding backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.forwardingBackup = forwardingBackup
	retriesBackup, err := s.writeOptional(s.retriesPath()+".bak", expected.retriesBackup, expected.retries.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified retry policy backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.retriesBackup = retriesBackup
	catalogBackup, err := s.writeOptional(s.catalogPath()+".bak", expected.catalogBackup, expected.Catalog.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified catalogue backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.catalogBackup = catalogBackup
	capabilitiesBackup, err := s.writeOptional(s.capabilitiesPath()+".bak", expected.capabilitiesBackup, expected.capabilities.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified route capability backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.capabilitiesBackup = capabilitiesBackup
	current, err = s.CaptureSnapshot()
	if err != nil || !current.equal(after) {
		return errors.Join(errors.New("configuration changed during backup convergence"), err, s.RestoreSnapshot(expected, after))
	}
	return nil
}

func (s Store) preserveMigrationRollbackInput(expected platform.FileSnapshot) (bool, error) {
	if !expected.Exists {
		return false, nil
	}
	version, err := configurationVersion(expected.Data)
	if err != nil || (version != domain.PublishedConfigVersion && version != domain.LegacyConfigVersion) {
		return false, nil
	}
	if _, err := migratePredecessorConfig(expected.Data, version, s.admission); err != nil {
		return false, fmt.Errorf("validate migration rollback input: %w", err)
	}
	current, err := platform.CaptureFileSnapshot(s.path + ".bak")
	if err != nil {
		return false, err
	}
	if !current.Equal(expected) {
		return false, errors.New("migration rollback input preimage changed; refusing to converge backup")
	}
	return true, nil
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
	if err := platform.RestoreFileAtomicIfPostimage(s.forwardingPath(), before.Forwarding, after.Forwarding); err != nil {
		failures = append(failures, fmt.Errorf("restore client forwarding snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.forwardingPath()+".bak", before.forwardingBackup, after.forwardingBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore client forwarding backup snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.catalogPath(), before.Catalog, after.Catalog); err != nil {
		failures = append(failures, fmt.Errorf("restore catalogue presentation snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.catalogPath()+".bak", before.catalogBackup, after.catalogBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore catalogue presentation backup: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.capabilitiesPath(), before.capabilities, after.capabilities); err != nil {
		failures = append(failures, fmt.Errorf("restore route capability snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.capabilitiesPath()+".bak", before.capabilitiesBackup, after.capabilitiesBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore route capability backup: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.retriesPath(), before.retries, after.retries); err != nil {
		failures = append(failures, fmt.Errorf("restore client retry policy snapshot: %w", err))
	}
	if err := platform.RestoreFileAtomicIfPostimage(s.retriesPath()+".bak", before.retriesBackup, after.retriesBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore client retry policy backup: %w", err))
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
	if errors.Is(err, os.ErrNotExist) {
		forwarding, forwardingErr := platform.CaptureFileSnapshot(s.forwardingPath())
		if forwardingErr != nil || forwarding.Exists {
			return domain.Config{}, newLoadError(LoadPhaseRead, errors.Join(errors.New("client forwarding has no canonical configuration"), forwardingErr))
		}
		return domain.NewConfig(s.admission), nil
	}
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	forwarding, err := platform.CaptureFileSnapshot(s.forwardingPath())
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
	for id, account := range cfg.Accounts {
		account.Order = 0
		cfg.Accounts[id] = account
	}
	for id, model := range cfg.Models {
		model.Order = 0
		cfg.Models[id] = model
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
	forwarding, err := platform.CaptureFileSnapshot(s.forwardingPath() + ".bak")
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
