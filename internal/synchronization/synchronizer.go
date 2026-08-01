// Package synchronization owns atomic convergence between AIGW configuration,
// Codex projections, and Codex native authentication.
package synchronization

import (
	"context"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

// Runner executes the bounded native authentication plan emitted by Codex.
type Runner interface {
	Run(context.Context, process.Plan) error
}

// ConfigStore is the exact persistence capability needed by a synchronization
// transaction. Keeping this boundary narrow makes failure and rollback
// semantics independently testable without substituting host filesystem
// behavior.
type ConfigStore interface {
	CaptureSnapshot() (configuration.Snapshot, error)
	Save(configuration.Config) error
	RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error
}

// Synchronizer carries the explicit dependencies required for one convergence
// transaction. It does not discover credentials, paths, or executables on its
// own, which keeps host and test policy outside the synchronization domain.
type Synchronizer struct {
	Config    ConfigStore
	Secrets   secrets.Store
	Runner    Runner
	Discovery discovery.Discoverer
}
