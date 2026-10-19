package configuration

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
	Config             transaction.FileSnapshot
	Backup             transaction.FileSnapshot
	Forwarding         transaction.FileSnapshot
	ForwardingBackup   transaction.FileSnapshot
	Catalog            transaction.FileSnapshot
	CatalogBackup      transaction.FileSnapshot
	Capabilities       transaction.FileSnapshot
	CapabilitiesBackup transaction.FileSnapshot
	Verified           transaction.FileSnapshot
}

func (snapshot Snapshot) equal(other Snapshot) bool {
	return snapshot.Config.Equal(other.Config) && snapshot.Backup.Equal(other.Backup) &&
		snapshot.Forwarding.Equal(other.Forwarding) && snapshot.ForwardingBackup.Equal(other.ForwardingBackup) &&
		snapshot.Catalog.Equal(other.Catalog) && snapshot.CatalogBackup.Equal(other.CatalogBackup) &&
		snapshot.Capabilities.Equal(other.Capabilities) && snapshot.CapabilitiesBackup.Equal(other.CapabilitiesBackup) &&
		snapshot.Verified.Equal(other.Verified)
}

// BackupConverged reports whether every rollback component equals the current
// generation, including intentionally absent optional components.
func (snapshot Snapshot) BackupConverged(mode os.FileMode) bool {
	return snapshot.Backup.Exists && snapshot.Backup.Mode == mode &&
		bytes.Equal(snapshot.Backup.Data, snapshot.Config.Data) &&
		backupComponentConverged(snapshot.Forwarding, snapshot.ForwardingBackup, mode) &&
		backupComponentConverged(snapshot.Catalog, snapshot.CatalogBackup, mode) &&
		backupComponentConverged(snapshot.Capabilities, snapshot.CapabilitiesBackup, mode)
}

// VerifiedBackupState pairs current backup bytes with client evidence when any client is enabled.
type VerifiedBackupState struct {
	Snapshot   Snapshot
	Current    Config
	Checkpoint VerifiedCheckpoint
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
	forwarding, err := transaction.CaptureFileSnapshot(s.forwardingPath())
	if err != nil {
		return Snapshot{}, err
	}
	forwardingBackup, err := transaction.CaptureFileSnapshot(s.forwardingPath() + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	catalog, err := transaction.CaptureFileSnapshot(s.catalogPath())
	if err != nil {
		return Snapshot{}, err
	}
	catalogBackup, err := transaction.CaptureFileSnapshot(s.catalogPath() + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	capabilities, err := transaction.CaptureFileSnapshot(s.capabilitiesPath())
	if err != nil {
		return Snapshot{}, err
	}
	capabilitiesBackup, err := transaction.CaptureFileSnapshot(s.capabilitiesPath() + ".bak")
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Config: configSnapshot, Backup: backupSnapshot, Forwarding: forwarding, ForwardingBackup: forwardingBackup, Catalog: catalog, CatalogBackup: catalogBackup, Capabilities: capabilities, CapabilitiesBackup: capabilitiesBackup, Verified: verifiedSnapshot}, nil
}

// Commit saves one configuration and returns the exact postimage needed for a
// guarded rollback. If postimage observation fails, it restores the prepared
// preimage before returning the error.
func (s Store) Commit(before Snapshot, cfg Config) (Snapshot, error) {
	data, err := encodeConfig(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	forwarding, err := encodeForwarding(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	catalog, err := encodeCatalogOrder(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	capabilities, err := encodeRouteCapabilities(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	return s.commitComponents(before, data, forwarding, catalog, capabilities)
}

func (s Store) commitData(before Snapshot, data []byte) (Snapshot, error) {
	return s.commitComponents(before, data, nil, nil, nil)
}

func (s Store) commitComponents(before Snapshot, data, forwarding, catalog, capabilities []byte) (Snapshot, error) {
	if before.Config.Exists && configurationDataEquivalent(before.Config.Data, data) && componentUnchanged(before.Forwarding, forwarding) &&
		componentUnchanged(before.Catalog, catalog) && componentUnchanged(before.Capabilities, capabilities) {
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
	if !before.Config.Exists || !configurationDataEquivalent(before.Config.Data, data) {
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
	catalogAfter, err := s.writeOptional(s.catalogPath(), before.Catalog, catalog)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write catalogue presentation: %w", err), s.RestoreSnapshot(before, after))
	}
	after.Catalog = catalogAfter
	capabilitiesAfter, err := s.writeOptional(s.capabilitiesPath(), before.Capabilities, capabilities)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("write route capabilities: %w", err), s.RestoreSnapshot(before, after))
	}
	after.Capabilities = capabilitiesAfter
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
func (snapshot Snapshot) MatchesConfiguration(expected Config) (bool, error) {
	if !snapshot.Config.Exists {
		if snapshot.Forwarding.Exists || snapshot.Catalog.Exists || snapshot.Capabilities.Exists {
			return false, errors.New("configuration component has no canonical configuration")
		}
		expected = expected.Clone()
		expected.Normalize()
		return reflect.DeepEqual(expected, NewConfig()), nil
	}
	current, err := (Store{}).loadSnapshot(snapshot)
	if err != nil {
		return false, err
	}
	return configurationsEqual(current, expected)
}

func configurationDataEquivalent(before, after []byte) bool {
	if bytes.Equal(before, after) {
		return true
	}
	previous, previousErr := decodeTOMLConfig(before)
	next, nextErr := decodeTOMLConfig(after)
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
	checkpoint, err := decodeVerifiedCheckpoint(snapshot.Verified.Data)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	checkpoint.Config, err = decodeCatalogOrder(checkpoint.Config, snapshot.Catalog)
	if err != nil {
		return VerifiedBackupState{}, err
	}
	checkpoint.Config, err = decodeRouteCapabilities(checkpoint.Config, snapshot.Capabilities)
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
	if !current.Forwarding.Equal(expected.Forwarding) || !current.ForwardingBackup.Equal(expected.ForwardingBackup) {
		return errors.New("client forwarding preimage changed; refusing to converge backup")
	}
	if !current.Catalog.Equal(expected.Catalog) || !current.CatalogBackup.Equal(expected.CatalogBackup) {
		return errors.New("catalogue presentation preimage changed; refusing to converge backup")
	}
	if !current.Capabilities.Equal(expected.Capabilities) || !current.CapabilitiesBackup.Equal(expected.CapabilitiesBackup) {
		return errors.New("route capability preimage changed; refusing to converge backup")
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
	forwardingBackup, err := s.writeOptional(s.forwardingPath()+".bak", expected.ForwardingBackup, expected.Forwarding.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified forwarding backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.ForwardingBackup = forwardingBackup
	catalogBackup, err := s.writeOptional(s.catalogPath()+".bak", expected.CatalogBackup, expected.Catalog.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified catalogue backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.CatalogBackup = catalogBackup
	capabilitiesBackup, err := s.writeOptional(s.capabilitiesPath()+".bak", expected.CapabilitiesBackup, expected.Capabilities.Data)
	if err != nil {
		return errors.Join(fmt.Errorf("converge verified route capability backup: %w", err), s.RestoreSnapshot(expected, after))
	}
	after.CapabilitiesBackup = capabilitiesBackup
	current, err = s.CaptureSnapshot()
	if err != nil || !current.equal(after) {
		return errors.Join(errors.New("configuration changed during backup convergence"), err, s.RestoreSnapshot(expected, after))
	}
	return nil
}

func (s Store) preserveMigrationRollbackInput(expected transaction.FileSnapshot) (bool, error) {
	if !expected.Exists {
		return false, nil
	}
	version, err := configurationVersion(expected.Data)
	if err != nil || (version != PublishedConfigVersion && version != LegacyConfigVersion) {
		return false, nil
	}
	if _, err := migratePredecessorConfig(expected.Data, version); err != nil {
		return false, fmt.Errorf("validate migration rollback input: %w", err)
	}
	current, err := transaction.CaptureFileSnapshot(s.path + ".bak")
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
	if err := transaction.RestoreFileAtomicIfPostimage(s.path, before.Config, after.Config); err != nil {
		failures = append(failures, fmt.Errorf("restore config snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.path+".bak", before.Backup, after.Backup); err != nil {
		failures = append(failures, fmt.Errorf("restore config backup snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.path+".verified.json", before.Verified, after.Verified); err != nil {
		failures = append(failures, fmt.Errorf("restore verified checkpoint snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.forwardingPath(), before.Forwarding, after.Forwarding); err != nil {
		failures = append(failures, fmt.Errorf("restore client forwarding snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.forwardingPath()+".bak", before.ForwardingBackup, after.ForwardingBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore client forwarding backup snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.catalogPath(), before.Catalog, after.Catalog); err != nil {
		failures = append(failures, fmt.Errorf("restore catalogue presentation snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.catalogPath()+".bak", before.CatalogBackup, after.CatalogBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore catalogue presentation backup: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.capabilitiesPath(), before.Capabilities, after.Capabilities); err != nil {
		failures = append(failures, fmt.Errorf("restore route capability snapshot: %w", err))
	}
	if err := transaction.RestoreFileAtomicIfPostimage(s.capabilitiesPath()+".bak", before.CapabilitiesBackup, after.CapabilitiesBackup); err != nil {
		failures = append(failures, fmt.Errorf("restore route capability backup: %w", err))
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
	cfg, err := s.LoadCredentialConfig()
	if err != nil {
		return Config{}, err
	}
	catalog, err := transaction.CaptureFileSnapshot(s.catalogPath())
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	cfg, err = decodeCatalogOrder(cfg, catalog)
	if err != nil {
		return Config{}, err
	}
	capabilities, err := transaction.CaptureFileSnapshot(s.capabilitiesPath())
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeRouteCapabilities(cfg, capabilities)
}

// LoadCredentialConfig reads only the routing and credential authority. Display
// metadata and later Route capabilities cannot interrupt a cached native
// credential reader.
func (s Store) LoadCredentialConfig() (Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		forwarding, forwardingErr := transaction.CaptureFileSnapshot(s.forwardingPath())
		if forwardingErr != nil || forwarding.Exists {
			return Config{}, newLoadError(LoadPhaseRead, errors.Join(errors.New("client forwarding has no canonical configuration"), forwardingErr))
		}
		return NewConfig(), nil
	}
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	forwarding, err := transaction.CaptureFileSnapshot(s.forwardingPath())
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeStoreConfig(data, forwarding)
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
func (s Store) LoadBackup() (Config, error) {
	data, err := os.ReadFile(s.path + ".bak")
	if err != nil {
		return Config{}, fmt.Errorf("read previous config backup: %w", err)
	}
	forwarding, err := transaction.CaptureFileSnapshot(s.forwardingPath() + ".bak")
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	cfg, err := decodeStoreConfig(data, forwarding)
	if err != nil {
		return Config{}, err
	}
	catalog, err := transaction.CaptureFileSnapshot(s.catalogPath() + ".bak")
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	cfg, err = decodeCatalogOrder(cfg, catalog)
	if err != nil {
		return Config{}, err
	}
	capabilities, err := transaction.CaptureFileSnapshot(s.capabilitiesPath() + ".bak")
	if err != nil {
		return Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeRouteCapabilities(cfg, capabilities)
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
