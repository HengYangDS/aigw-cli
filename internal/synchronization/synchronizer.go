// Package synchronization owns configuration and client projection changes
// with guarded compensation for writes that remain owned.
package synchronization

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

// ClientIDs returns admitted clients in stable execution order.
func (s Synchronizer) ClientIDs() []string {
	return s.registry().IDs()
}

// Inspect observes one admitted client through its operational adapter.
func (s Synchronizer) Inspect(ctx context.Context, cfg configuration.Config, clientID string, runtime configuration.Runtime) client.Status {
	return s.registry().Inspect(ctx, s.clientDependencies(), cfg, clientID, runtime)
}

// Verify runs one explicit live request through the admitted client adapter.
func (s Synchronizer) Verify(ctx context.Context, cfg configuration.Config, clientID string, runtime configuration.Runtime, explicitRoute string) (client.Verification, error) {
	return s.registry().Verify(ctx, s.clientDependencies(), cfg, clientID, runtime, explicitRoute)
}

// ReconcileClient projects one unchanged Route without saving configuration or
// touching other clients. Converged settings remain a byte-exact no-op.
func (s Synchronizer) ReconcileClient(ctx context.Context, cfg configuration.Config, clientID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.registry().Plan(s.clientDependencies(), cfg, cfg, clientID); err != nil {
		return err
	}
	undoEntrypoint, err := s.prepareCredentialEntrypoint(cfg, clientID)
	if err != nil {
		return err
	}
	err = s.registry().Apply(ctx, s.clientDependencies(), cfg, cfg, clientID)
	if err == nil || errors.Is(err, client.ErrProjectionRollbackFailed) || undoEntrypoint == nil {
		return err
	}
	return errors.Join(err, undoEntrypoint())
}

func (s Synchronizer) prepareCredentialEntrypoint(cfg configuration.Config, clientIDs ...string) (func() error, error) {
	required, err := s.needsCredentialEntrypoint(cfg, clientIDs...)
	if err != nil || !required {
		return nil, err
	}
	return credential.EnsureEntrypoint(s.AIGWExecutable, s.CredentialPath)
}

// ConfigStore is the exact persistence capability needed by a synchronization
// transaction.
type ConfigStore interface {
	CaptureSnapshot() (configuration.Snapshot, error)
	Commit(before configuration.Snapshot, cfg configuration.Config) (configuration.Snapshot, error)
	RestoreSnapshot(before, after configuration.Snapshot) error
}

// Synchronizer carries the explicit dependencies required for one convergence
// transaction. Client behavior is selected only through Registry.
type Synchronizer struct {
	Config                       ConfigStore
	Secrets                      secrets.Store
	Runner                       process.CaptureRunner
	Discovery                    discovery.Discoverer
	Registry                     client.Registry
	ClaudeSettingsPath           string
	AIGWExecutable               string
	CredentialPath               string
	AuthorizeCodexRouteSelection bool
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
	executable := s.AIGWExecutable
	if s.CredentialPath != "" {
		executable = s.CredentialPath
	}
	return client.Dependencies{
		Secrets:                      s.Secrets,
		Runner:                       s.Runner,
		Discovery:                    s.Discovery,
		ClaudeSettingsPath:           s.ClaudeSettingsPath,
		AIGWExecutable:               executable,
		AuthorizeCodexRouteSelection: s.AuthorizeCodexRouteSelection,
	}
}

func (s Synchronizer) needsCredentialEntrypoint(cfg configuration.Config, clientIDs ...string) (bool, error) {
	if s.CredentialPath == "" {
		return false, nil
	}
	if len(clientIDs) == 0 {
		clientIDs = s.ClientIDs()
	}
	for _, clientID := range clientIDs {
		if !cfg.Clients[clientID].Enabled {
			continue
		}
		runtime, err := cfg.ResolveRuntime(clientID, "")
		if err != nil {
			return false, err
		}
		if runtime.RequiresAccountToken() && runtime.CredentialCommand == "" {
			return true, nil
		}
	}
	return false, nil
}

// CredentialEntrypointPlan observes the one shared helper prerequisite without
// creating it. A false result means no default Account-Token client needs work.
func (s Synchronizer) CredentialEntrypointPlan(cfg configuration.Config, clientIDs ...string) (bool, error) {
	required, err := s.needsCredentialEntrypoint(cfg, clientIDs...)
	if err != nil || !required {
		return false, err
	}
	return credential.EntrypointNeeded(s.CredentialPath)
}

func (s Synchronizer) discoveredResult() (discovery.Result, error) {
	if s.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return s.Discovery.Discover(), nil
}
