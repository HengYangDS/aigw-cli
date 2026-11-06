package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// Optional components hold configuration that published schema-six readers
// cannot accept. Each is absent when empty and follows the canonical file
// through backup, compensation and comparison.

// The optional components, in the order that a commit writes them.
const (
	forwardingComponent = iota
	retriesComponent
	catalogComponent
	capabilitiesComponent
	componentCount
)

// component is one optional component's file, after the configuration's stem,
// and the words that name it in a failure: as a whole when a write or backup
// fails, as one record when a preimage or restoration fails, and its backup
// in verified convergence and in restoration.
type component struct {
	suffix         string
	whole          string
	record         string
	verifiedBackup string
	restoredBackup string
	// bothGenerations reads the backup even when the current file failed, and
	// reports both failures.
	bothGenerations bool
}

var components = [componentCount]component{
	forwardingComponent: {
		suffix: ".forwarding.toml", whole: "client forwarding", record: "client forwarding",
		verifiedBackup: "forwarding", restoredBackup: "client forwarding backup snapshot",
	},
	retriesComponent: {
		suffix: ".retries.toml", whole: "client retry policies", record: "client retry policy",
		verifiedBackup: "retry policy", restoredBackup: "client retry policy backup", bothGenerations: true,
	},
	catalogComponent: {
		suffix: ".catalog.toml", whole: "catalog presentation", record: "catalog presentation",
		verifiedBackup: "catalog", restoredBackup: "catalog presentation backup",
	},
	capabilitiesComponent: {
		suffix: ".capabilities.toml", whole: "route capabilities", record: "route capability",
		verifiedBackup: "route capability", restoredBackup: "route capability backup", bothGenerations: true,
	},
}

// restoreOrder is the order in which restoration and verified convergence
// visit the components, which differs from the order that a commit writes them.
var restoreOrder = [componentCount]int{forwardingComponent, catalogComponent, capabilitiesComponent, retriesComponent}

// componentSnapshots holds one generation of every optional component.
type componentSnapshots [componentCount]platform.FileSnapshot

func (snapshots componentSnapshots) equal(other componentSnapshots) bool {
	return slices.EqualFunc(snapshots[:], other[:], platform.FileSnapshot.Equal)
}

// componentPath returns one optional component's file beside the canonical file.
func (s Store) componentPath(index int) string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + components[index].suffix
}

// backUp retains the current generation of the canonical file and every
// optional component before a commit replaces them.
func (s Store) backUp(before Snapshot) (Snapshot, error) {
	after := before
	backup, err := writeConfigurationFileIfUnchanged(s.path+".bak", before.Backup, before.Config.Data, 0o600)
	if err != nil {
		return Snapshot{}, fmt.Errorf("back up current config: %w", err)
	}
	after.Backup = backup
	for index, component := range components {
		written, err := s.writeOptional(s.componentPath(index)+".bak", before.backups[index], before.components[index].Data)
		if err != nil {
			return Snapshot{}, errors.Join(fmt.Errorf("back up %s: %w", component.whole, err), s.RestoreSnapshot(before, after))
		}
		after.backups[index] = written
	}
	return after, nil
}

// captureCurrent reads the canonical file and every optional component, but no
// recovery file.
func (s Store) captureCurrent() (Snapshot, error) {
	config, err := platform.CaptureFileSnapshot(s.path)
	snapshot, failures := Snapshot{Config: config}, []error{err}
	for index := range components {
		captured, err := platform.CaptureFileSnapshot(s.componentPath(index))
		snapshot.components[index], failures = captured, append(failures, err)
	}
	if failure := errors.Join(failures...); failure != nil {
		return Snapshot{}, failure
	}
	return snapshot, nil
}

// Forwarding returns the client forwarding component.
func (snapshot Snapshot) Forwarding() platform.FileSnapshot {
	return snapshot.components[forwardingComponent]
}

// Catalog returns the catalog presentation component.
func (snapshot Snapshot) Catalog() platform.FileSnapshot {
	return snapshot.components[catalogComponent]
}

func (snapshot Snapshot) equal(other Snapshot) bool {
	return snapshot.sameCurrent(other) && snapshot.Backup.Equal(other.Backup) &&
		snapshot.backups.equal(other.backups) && snapshot.Verified.Equal(other.Verified)
}

func (snapshot Snapshot) sameCurrent(other Snapshot) bool {
	return snapshot.Config.Equal(other.Config) && snapshot.components.equal(other.components)
}

// withComponents reads the optional components of one generation, the
// current one or ".bak", and merges them into its canonical configuration.
func (s Store) withComponents(cfg domain.Config, suffix string) (domain.Config, error) {
	catalog, err := platform.CaptureFileSnapshot(s.componentPath(catalogComponent) + suffix)
	capabilities, capabilitiesErr := platform.CaptureFileSnapshot(s.componentPath(capabilitiesComponent) + suffix)
	if failure := errors.Join(err, capabilitiesErr); failure != nil {
		return domain.Config{}, newLoadError(LoadPhaseRead, failure)
	}
	cfg, err = decodeComponents(cfg, catalog, capabilities)
	if err != nil {
		return domain.Config{}, err
	}
	retries, err := platform.CaptureFileSnapshot(s.componentPath(retriesComponent) + suffix)
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

// componentsUnchanged reports whether every optional component already holds
// its data, where empty data means that the component is absent.
func componentsUnchanged(before componentSnapshots, data [componentCount][]byte) bool {
	for index := range before {
		if before[index].Exists != (len(data[index]) != 0) || !bytes.Equal(before[index].Data, data[index]) {
			return false
		}
	}
	return true
}

func backupComponentConverged(current, backup platform.FileSnapshot, mode os.FileMode) bool {
	return backup.Exists == current.Exists && bytes.Equal(backup.Data, current.Data) && (!current.Exists || backup.Mode == mode)
}
