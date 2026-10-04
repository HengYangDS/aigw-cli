// Package synchronization owns configuration and client projection changes
// with guarded compensation for writes that remain owned.
package synchronization

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

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
	dependencies, err := s.clientDependencies([]string{clientID}, cfg)
	if err != nil {
		return client.Status{Issue: "AIGW credential reader identity is unavailable", RepairAction: "aigw doctor"}
	}
	return s.registry().Inspect(ctx, dependencies, cfg, clientID, runtime)
}

// Verify runs one explicit live request through the admitted client adapter.
func (s Synchronizer) Verify(ctx context.Context, cfg configuration.Config, clientID string, runtime configuration.Runtime, explicitRoute string) (client.Verification, error) {
	selected := cfg.Clone()
	selected.SetSelectedRoute(clientID, runtime.RouteID)
	binding := selected.Clients[clientID]
	binding.Protocol, binding.ModelProvider, binding.Authentication = runtime.Protocol, runtime.ModelProvider, runtime.Authentication
	selected.Clients[clientID] = binding
	dependencies, err := s.clientDependencies([]string{clientID}, cfg, selected)
	if err != nil {
		return client.Verification{}, err
	}
	return s.registry().Verify(ctx, dependencies, cfg, clientID, runtime, explicitRoute)
}

// ReconcileClient projects one unchanged Route without saving configuration or
// touching other clients. Converged settings remain a byte-exact no-op.
func (s Synchronizer) ReconcileClient(ctx context.Context, cfg configuration.Config, clientID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	projectable, err := s.credentialReadyClients(cfg, clientID)
	if err != nil {
		return err
	}
	if len(projectable) == 0 {
		return nil
	}
	dependencies, err := s.clientDependencies([]string{clientID}, cfg)
	if err != nil {
		return err
	}
	if _, err := s.registry().Plan(dependencies, cfg, cfg, clientID); err != nil {
		return err
	}
	undoEntrypoint, err := s.prepareCredentialEntrypoint(cfg, clientID)
	if err != nil {
		return err
	}
	receipt, err := s.registry().Apply(ctx, dependencies, cfg, cfg, clientID)
	if err != nil {
		if !errors.Is(err, client.ErrProjectionRollbackFailed) {
			err = errors.Join(err, undoCreatedEntrypoint(undoEntrypoint))
		}
		return err
	}
	finalizeErr := s.finalizeCredentialEntrypoint(cfg, clientID)
	if finalizeErr == nil {
		return nil
	}
	if rollbackErr := receipt.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: credential entrypoint finalization failed: %w; client rollback also failed: %w", client.ErrProjectionRollbackFailed, finalizeErr, rollbackErr)
	}
	return errors.Join(fmt.Errorf("credential entrypoint finalization failed; client projection was rolled back: %w", finalizeErr), undoCreatedEntrypoint(undoEntrypoint))
}

func (s Synchronizer) prepareCredentialEntrypoint(cfg configuration.Config, clientIDs ...string) (func() error, error) {
	accounts, err := s.credentialEntrypointAccounts(cfg, clientIDs...)
	if err != nil || len(accounts) == 0 {
		return nil, err
	}
	path, err := s.CredentialEntrypointPath()
	if err != nil {
		return nil, err
	}
	undo, err := credential.EnsureEntrypoint(s.AIGWExecutable, path)
	if err != nil || s.Secrets == nil {
		return undo, err
	}
	if err := secrets.VerifyNativeReaderAccess(s.Secrets, path, accounts); err != nil {
		return nil, errors.Join(err, undoCreatedEntrypoint(undo))
	}
	return undo, nil
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
	Runner                       process.VerificationRunner
	Discovery                    discovery.Discoverer
	Registry                     client.Registry
	ClaudeSettingsPath           string
	AIGWExecutable               string
	CredentialPath               string
	ResolveCredentialPath        func() (string, error)
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
	dependencies, err := s.clientDependencies(nil)
	if err != nil {
		return configuration.Config{}, discovered, err
	}
	after, err := s.registry().Converge(dependencies, before, discovered, clientIDs...)
	return after, discovered, err
}

// DesiredSyncConfiguration activates unselected recommendations when a
// read-only credential source gains an Account Token after catalogue import.
// Writable native stores are not searched for unselected credentials.
func (s Synchronizer) DesiredSyncConfiguration(before configuration.Config) (configuration.Config, discovery.Result, error) {
	if !secrets.IsReadOnly(s.Secrets) {
		return s.DesiredClientConfiguration(before)
	}
	connected := make([]string, 0, len(before.Accounts))
	for _, accountID := range slices.Sorted(maps.Keys(before.Accounts)) {
		available, err := s.Secrets.Exists(accountID)
		if err != nil {
			return configuration.Config{}, discovery.Result{}, fmt.Errorf("inspect Account %q Token availability: %w", accountID, err)
		}
		if available {
			connected = append(connected, accountID)
		}
	}
	selected, err := before.SelectRoutesForConnectedAccounts(connected)
	if err != nil {
		return configuration.Config{}, discovery.Result{}, err
	}
	return s.DesiredClientConfiguration(selected)
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

// CredentialEntrypointPath resolves the invocation-owned reader identity only
// when needed. Explicit paths remain authoritative; a failed resolver never
// authorizes fallback to an installer-owned public executable.
func (s Synchronizer) CredentialEntrypointPath() (string, error) {
	if s.CredentialPath != "" || s.ResolveCredentialPath == nil {
		return s.CredentialPath, nil
	}
	path, err := s.ResolveCredentialPath()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", errors.New("AIGW credential reader identity is unavailable")
	}
	return path, nil
}

func (s Synchronizer) clientDependencies(clientIDs []string, configurations ...configuration.Config) (client.Dependencies, error) {
	executable := s.AIGWExecutable
	if s.CredentialPath != "" {
		executable = s.CredentialPath
	}
	for _, cfg := range configurations {
		accounts, err := s.credentialEntrypointAccounts(cfg, clientIDs...)
		if err != nil {
			return client.Dependencies{}, err
		}
		if len(accounts) == 0 {
			continue
		}
		path, err := s.CredentialEntrypointPath()
		if err != nil {
			return client.Dependencies{}, err
		}
		executable = path
		break
	}
	return client.Dependencies{
		Secrets:                      s.Secrets,
		Runner:                       s.Runner,
		Discovery:                    s.Discovery,
		ClaudeSettingsPath:           s.ClaudeSettingsPath,
		AIGWExecutable:               executable,
		AuthorizeCodexRouteSelection: s.AuthorizeCodexRouteSelection,
	}, nil
}

// credentialReadyClients excludes default-Token consumers only when their
// selected Account Token is known absent. Disabled and external-credential
// clients remain eligible for withdrawal or independent convergence.
func (s Synchronizer) credentialReadyClients(cfg configuration.Config, clientIDs ...string) ([]string, error) {
	if len(clientIDs) == 0 {
		clientIDs = s.ClientIDs()
	}
	projectable := make([]string, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		accounts, err := s.credentialEntrypointAccounts(cfg, clientID)
		if err != nil {
			return nil, err
		}
		if s.Secrets == nil {
			projectable = append(projectable, clientID)
			continue
		}
		ready := true
		for _, account := range accounts {
			present, err := s.Secrets.Exists(account)
			if err != nil {
				return nil, fmt.Errorf("inspect Account %q Token availability: %w", account, err)
			}
			if !present {
				ready = false
				break
			}
		}
		if ready {
			projectable = append(projectable, clientID)
		}
	}
	return projectable, nil
}

func (s Synchronizer) credentialEntrypointAccounts(cfg configuration.Config, clientIDs ...string) ([]string, error) {
	if s.CredentialPath == "" && s.ResolveCredentialPath == nil {
		return nil, nil
	}
	if len(clientIDs) == 0 {
		clientIDs = s.ClientIDs()
	}
	var accounts []string
	for _, clientID := range clientIDs {
		if !cfg.Clients[clientID].Enabled {
			continue
		}
		runtime, err := cfg.ResolveRuntime(clientID, "")
		if err != nil {
			return nil, err
		}
		if runtime.RequiresAccountToken() && runtime.CredentialCommand == "" && !slices.Contains(accounts, runtime.AccountID) {
			accounts = append(accounts, runtime.AccountID)
		}
	}
	return accounts, nil
}

// CredentialEntrypointAction names the one filesystem effect required by the
// complete desired Client Binding set. An empty action means no change.
type CredentialEntrypointAction string

const (
	// CredentialEntrypointUnchanged means the desired helper is already aligned.
	CredentialEntrypointUnchanged CredentialEntrypointAction = ""
	// CredentialEntrypointInstall creates the helper for a default Token consumer.
	CredentialEntrypointInstall CredentialEntrypointAction = "install"
)

// CredentialEntrypointPlan prepares the current version only for clients that
// can project now. A deferred client does not prove that cached or rollback
// callers have stopped using an earlier command.
func (s Synchronizer) CredentialEntrypointPlan(cfg configuration.Config, clientIDs ...string) (CredentialEntrypointAction, error) {
	if s.CredentialPath == "" && s.ResolveCredentialPath == nil {
		return CredentialEntrypointUnchanged, nil
	}
	if len(clientIDs) == 0 {
		ready, err := s.credentialReadyClients(cfg)
		if err != nil {
			return CredentialEntrypointUnchanged, err
		}
		if len(ready) == 0 {
			return CredentialEntrypointUnchanged, nil
		}
		clientIDs = ready
	}
	accounts, err := s.credentialEntrypointAccounts(cfg, clientIDs...)
	if err != nil {
		return CredentialEntrypointUnchanged, err
	}
	if len(accounts) == 0 {
		return CredentialEntrypointUnchanged, nil
	}
	path, err := s.CredentialEntrypointPath()
	if err != nil {
		return CredentialEntrypointUnchanged, err
	}
	missing, err := credential.EntrypointNeeded(path)
	if err != nil {
		return CredentialEntrypointUnchanged, err
	}
	if missing {
		return CredentialEntrypointInstall, nil
	}
	return CredentialEntrypointUnchanged, nil
}

func (s Synchronizer) discoveredResult() (discovery.Result, error) {
	if s.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return s.Discovery.Discover(), nil
}
