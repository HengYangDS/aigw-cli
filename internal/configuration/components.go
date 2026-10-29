package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/transaction"
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
	forwardingBackup, err := s.writeOptional(s.forwardingPath()+".bak", before.ForwardingBackup, before.Forwarding.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up client forwarding: %w", err), s.RestoreSnapshot(before, after))
	}
	after.ForwardingBackup = forwardingBackup
	retriesBackup, err := s.writeOptional(s.retriesPath()+".bak", before.RetriesBackup, before.Retries.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up client retry policies: %w", err), s.RestoreSnapshot(before, after))
	}
	after.RetriesBackup = retriesBackup
	catalogBackup, err := s.writeOptional(s.catalogPath()+".bak", before.CatalogBackup, before.Catalog.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up catalogue presentation: %w", err), s.RestoreSnapshot(before, after))
	}
	after.CatalogBackup = catalogBackup
	capabilitiesBackup, err := s.writeOptional(s.capabilitiesPath()+".bak", before.CapabilitiesBackup, before.Capabilities.Data)
	if err != nil {
		return Snapshot{}, errors.Join(fmt.Errorf("back up route capabilities: %w", err), s.RestoreSnapshot(before, after))
	}
	after.CapabilitiesBackup = capabilitiesBackup
	return after, nil
}

// captureCurrent reads the canonical file and every optional component, but no
// recovery file.
func (s Store) captureCurrent() (Snapshot, error) {
	config, err := transaction.CaptureFileSnapshot(s.path)
	forwarding, forwardingErr := transaction.CaptureFileSnapshot(s.forwardingPath())
	retries, retriesErr := transaction.CaptureFileSnapshot(s.retriesPath())
	catalog, catalogErr := transaction.CaptureFileSnapshot(s.catalogPath())
	capabilities, capabilitiesErr := transaction.CaptureFileSnapshot(s.capabilitiesPath())
	if failure := errors.Join(err, forwardingErr, retriesErr, catalogErr, capabilitiesErr); failure != nil {
		return Snapshot{}, failure
	}
	return Snapshot{Config: config, Forwarding: forwarding, Retries: retries, Catalog: catalog, Capabilities: capabilities}, nil
}

func (snapshot Snapshot) sameCurrent(other Snapshot) bool {
	return snapshot.Config.Equal(other.Config) && snapshot.Forwarding.Equal(other.Forwarding) &&
		snapshot.Catalog.Equal(other.Catalog) && snapshot.Capabilities.Equal(other.Capabilities) && snapshot.Retries.Equal(other.Retries)
}

// withComponents reads the optional components of one generation, the
// current one or ".bak", and merges them into its canonical configuration.
func (s Store) withComponents(cfg domain.Config, suffix string) (domain.Config, error) {
	catalog, err := transaction.CaptureFileSnapshot(s.catalogPath() + suffix)
	capabilities, capabilitiesErr := transaction.CaptureFileSnapshot(s.capabilitiesPath() + suffix)
	if failure := errors.Join(err, capabilitiesErr); failure != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, failure)
	}
	cfg, err = decodeComponents(cfg, catalog, capabilities)
	if err != nil {
		return domain.Config{}, err
	}
	retries, err := transaction.CaptureFileSnapshot(s.retriesPath() + suffix)
	if err != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, err)
	}
	return decodeRetryPolicies(cfg, retries)
}

// decodeComponents merges catalogue presentation and recorded Route
// capabilities into a decoded canonical configuration.
func decodeComponents(cfg domain.Config, catalog, capabilities transaction.FileSnapshot) (domain.Config, error) {
	cfg, err := decodeCatalogOrder(cfg, catalog)
	if err != nil {
		return domain.Config{}, err
	}
	return decodeRouteCapabilities(cfg, capabilities)
}

func (Store) writeOptional(path string, before transaction.FileSnapshot, data []byte) (transaction.FileSnapshot, error) {
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
func componentUnchanged(before transaction.FileSnapshot, data []byte) bool {
	return before.Exists == (len(data) != 0) && bytes.Equal(before.Data, data)
}

func backupComponentConverged(current, backup transaction.FileSnapshot, mode os.FileMode) bool {
	return backup.Exists == current.Exists && bytes.Equal(backup.Data, current.Data) && (!current.Exists || backup.Mode == mode)
}
