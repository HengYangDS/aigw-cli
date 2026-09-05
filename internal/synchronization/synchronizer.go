// Package synchronization owns atomic convergence between AIGW configuration
// and every admitted client adapter.
package synchronization

import (
	"context"
	"fmt"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

// ClientIDs returns admitted clients in stable execution order.
func (s Synchronizer) ClientIDs() []string {
	return s.registry().IDs()
}

// Inspect observes one admitted client through its operational adapter.
func (s Synchronizer) Inspect(ctx context.Context, cfg configuration.Config, clientID string, runtime configuration.Runtime, options client.InspectionOptions) client.Status {
	return s.registry().Inspect(ctx, s.clientDependencies(), cfg, clientID, runtime, options)
}

// Verify runs one explicit live request through the admitted client adapter.
func (s Synchronizer) Verify(ctx context.Context, cfg configuration.Config, clientID string, runtime configuration.Runtime) (client.Verification, error) {
	return s.registry().Verify(ctx, s.clientDependencies(), cfg, clientID, runtime)
}

// ConfigStore is the exact persistence capability needed by a synchronization
// transaction.
type ConfigStore interface {
	CaptureSnapshot() (configuration.Snapshot, error)
	Commit(configuration.Snapshot, configuration.Config) (configuration.Snapshot, error)
	RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error
}

// Synchronizer carries the explicit dependencies required for one convergence
// transaction. Client behavior is selected only through Registry.
type Synchronizer struct {
	Config             ConfigStore
	Secrets            secrets.Store
	Runner             client.Runner
	Discovery          discovery.Discoverer
	Registry           client.Registry
	ClaudeSettingsPath string
	AIGWExecutable     string
}

// DesiredClientConfiguration discovers the requested clients and derives the
// configuration that can be activated now. An empty list means every admitted
// client; unrequested adapters remain unchanged.
func (s Synchronizer) DesiredClientConfiguration(before configuration.Config, clientIDs ...string) (configuration.Config, discovery.Result, error) {
	discovered, err := s.discoveredResult()
	if err != nil {
		return configuration.Config{}, discovery.Result{}, err
	}
	after, err := s.registry().Converge(s.clientDependencies(), before, discovered, clientIDs...)
	return after, discovered, err
}

// Withdraw removes selected adapters from desired configuration. With no
// explicit IDs it withdraws every admitted adapter for portable uninstall.
func (s Synchronizer) Withdraw(cfg *configuration.Config, clientIDs ...string) error {
	if len(clientIDs) == 0 {
		clientIDs = s.registry().IDs()
	}
	for _, clientID := range clientIDs {
		if err := s.registry().Withdraw(cfg, clientID); err != nil {
			return err
		}
	}
	return nil
}

func (s Synchronizer) registry() client.Registry {
	if s.Registry.Empty() {
		return client.DefaultRegistry()
	}
	return s.Registry
}

func (s Synchronizer) clientDependencies() client.Dependencies {
	return client.Dependencies{
		Secrets:            s.Secrets,
		Runner:             s.Runner,
		Discovery:          s.Discovery,
		ClaudeSettingsPath: s.ClaudeSettingsPath,
		AIGWExecutable:     s.AIGWExecutable,
	}
}

func (s Synchronizer) discoveredResult() (discovery.Result, error) {
	if s.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return s.Discovery.Discover(), nil
}
