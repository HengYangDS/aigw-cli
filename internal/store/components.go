package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// Optional components hold configuration that published schema-six readers
// cannot accept. Each is absent when empty and follows the canonical file
// through backup, compensation and comparison.

// backUp retains the current generation of the canonical file and every
// optional component before a commit replaces them.
func (s Store) backUp(before Snapshot) (Snapshot, error) {
	after := before
	backup, err := writeConfigurationFileIfUnchanged(s.path+".bak", before.Backup, before.Config.Data, 0o600)
	if err != nil {
		return Snapshot{}, fmt.Errorf("back up current config: %w", err)
	}
	after.Backup = backup
	forwardingBackup, err := s.writeOptional(s.forwardingPath()+".bak", before.forwardingBackup, before.Forwarding.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up client forwarding: %w", err), s.RestoreSnapshot(before, after))
	}
	after.forwardingBackup = forwardingBackup
	retriesBackup, err := s.writeOptional(s.retriesPath()+".bak", before.retriesBackup, before.retries.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up client retry policies: %w", err), s.RestoreSnapshot(before, after))
	}
	after.retriesBackup = retriesBackup
	catalogBackup, err := s.writeOptional(s.catalogPath()+".bak", before.catalogBackup, before.Catalog.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up catalog presentation: %w", err), s.RestoreSnapshot(before, after))
	}
	after.catalogBackup = catalogBackup
	capabilitiesBackup, err := s.writeOptional(s.capabilitiesPath()+".bak", before.capabilitiesBackup, before.capabilities.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up route capabilities: %w", err), s.RestoreSnapshot(before, after))
	}
	after.capabilitiesBackup = capabilitiesBackup
	return after, nil
}

// captureCurrent reads the canonical file and every optional component, but no
// recovery file.
func (s Store) captureCurrent() (Snapshot, error) {
	config, err := platform.CaptureFileSnapshot(s.path)
	forwarding, forwardingErr := platform.CaptureFileSnapshot(s.forwardingPath())
	retries, retriesErr := platform.CaptureFileSnapshot(s.retriesPath())
	catalog, catalogErr := platform.CaptureFileSnapshot(s.catalogPath())
	capabilities, capabilitiesErr := platform.CaptureFileSnapshot(s.capabilitiesPath())
	if failure := errors.Join(err, forwardingErr, retriesErr, catalogErr, capabilitiesErr); failure != nil {
		return Snapshot{}, failure
	}
	return Snapshot{Config: config, Forwarding: forwarding, retries: retries, Catalog: catalog, capabilities: capabilities}, nil
}

func (snapshot Snapshot) sameCurrent(other Snapshot) bool {
	return snapshot.Config.Equal(other.Config) && snapshot.Forwarding.Equal(other.Forwarding) &&
		snapshot.Catalog.Equal(other.Catalog) && snapshot.capabilities.Equal(other.capabilities) && snapshot.retries.Equal(other.retries)
}

// withComponents reads the optional components of one generation, the
// current one or ".bak", and merges them into its canonical configuration.
func (s Store) withComponents(cfg domain.Config, suffix string) (domain.Config, error) {
	catalog, err := platform.CaptureFileSnapshot(s.catalogPath() + suffix)
	capabilities, capabilitiesErr := platform.CaptureFileSnapshot(s.capabilitiesPath() + suffix)
	if failure := errors.Join(err, capabilitiesErr); failure != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, failure)
	}
	cfg, err = decodeComponents(cfg, catalog, capabilities)
	if err != nil {
		return domain.Config{}, err
	}
	retries, err := platform.CaptureFileSnapshot(s.retriesPath() + suffix)
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeRetryPolicies(cfg, retries)
}

// decodeComponents merges catalog presentation and recorded Route
// capabilities into a decoded canonical configuration.
func decodeComponents(cfg domain.Config, catalog, capabilities platform.FileSnapshot) (domain.Config, error) {
	cfg, err := decodeCatalogOrder(cfg, catalog)
	if err != nil {
		return domain.Config{}, err
	}
	return decodeRouteCapabilities(cfg, capabilities)
}

func (Store) writeOptional(path string, before platform.FileSnapshot, data []byte) (platform.FileSnapshot, error) {
	if len(data) == 0 {
		if !before.Exists {
			return before, nil
		}
		return removeConfigurationFileIfUnchanged(path, before)
	}
	return writeConfigurationFileIfUnchanged(path, before, data, 0o600)
}

// componentUnchanged reports whether an optional component already holds data,
// where empty data means that the component is absent.
func componentUnchanged(before platform.FileSnapshot, data []byte) bool {
	return before.Exists == (len(data) != 0) && bytes.Equal(before.Data, data)
}

func backupComponentConverged(current, backup platform.FileSnapshot, mode os.FileMode) bool {
	return backup.Exists == current.Exists && bytes.Equal(backup.Data, current.Data) && (!current.Exists || backup.Mode == mode)
}
