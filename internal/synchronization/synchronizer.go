// Package synchronization owns atomic convergence between AIGW configuration,
// Codex projections, and Codex native authentication.
package synchronization

import (
	"context"
	"fmt"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
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
	Commit(configuration.Snapshot, configuration.Config) (configuration.Snapshot, error)
	RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error
}

// Synchronizer carries the explicit dependencies required for one convergence
// transaction. It does not discover credentials, paths, or executables on its
// own, which keeps host and test policy outside the synchronization domain.
type Synchronizer struct {
	Config             ConfigStore
	Secrets            secrets.Store
	Runner             Runner
	Discovery          discovery.Discoverer
	ClaudeSettingsPath string
	AIGWExecutable     string
}

// DesiredClientConfiguration discovers the requested clients and returns the
// configuration that can be activated with the currently available
// executables, surfaces, and selected routes. An empty client list means
// every admitted client; an explicit list leaves every other adapter unchanged.
func (s Synchronizer) DesiredClientConfiguration(before configuration.Config, clients ...string) (configuration.Config, discovery.Result, error) {
	if len(clients) == 0 {
		clients = configuration.AdmittedClientIDs()
	}
	discovered, err := s.discoveredResult()
	if err != nil {
		return configuration.Config{}, discovery.Result{}, err
	}
	after := before.Clone()
	for _, client := range clients {
		switch client {
		case configuration.ClientClaude:
			if err := s.convergeClaude(&after, discovered); err != nil {
				return configuration.Config{}, discovery.Result{}, err
			}
		case configuration.ClientCodex:
			if err := s.convergeCodex(&after, discovered); err != nil {
				return configuration.Config{}, discovery.Result{}, err
			}
		}
	}
	return after, discovered, nil
}

func (s Synchronizer) convergeClaude(cfg *configuration.Config, discovered discovery.Result) error {
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return nil
	}
	adapter := cfg.Adapters[configuration.ClientClaude]
	executable, err := resolveExecutable(configuration.ClientClaude, adapter.Executable, discovered.Executable(configuration.ClientClaude))
	if err != nil {
		return err
	}
	available, err := s.secretAvailable(clientRuntime.AccountID)
	if err != nil {
		return err
	}
	if executable != "" && (adapter.Enabled || available) {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	}
	return nil
}

func (s Synchronizer) convergeCodex(cfg *configuration.Config, discovered discovery.Result) error {
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return nil
	}
	adapter := cfg.Adapters[configuration.ClientCodex]
	targets := codexTargets(discovered, adapter.Targets)
	executable, err := resolveExecutable(configuration.ClientCodex, adapter.Executable, discovered.Executable(configuration.ClientCodex))
	if err != nil {
		return err
	}
	available, err := s.secretAvailable(clientRuntime.AccountID)
	if err != nil {
		return err
	}
	if executable != "" && len(targets) > 0 && (adapter.Enabled || available) {
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: targets}
	} else if adapter.Enabled && len(targets) == 0 {
		delete(cfg.Adapters, configuration.ClientCodex)
	}
	return nil
}

func resolveExecutable(client, configured, discovered string) (string, error) {
	available, err := discovery.ExecutableAvailable(configured)
	if err != nil {
		return "", fmt.Errorf("inspect configured %s executable: %w", client, err)
	}
	if available || discovered == "" {
		return configured, nil
	}
	return discovered, nil
}

func (s Synchronizer) secretAvailable(accountID string) (bool, error) {
	if s.Secrets == nil {
		return false, nil
	}
	return s.Secrets.Exists(accountID)
}

func codexTargets(discovered discovery.Result, current []string) []string {
	seen := map[string]bool{}
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	appendTarget := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	for _, path := range discovered.AutoManagedCodexTargets() {
		appendTarget(path)
	}
	for _, path := range current {
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			if surface.ID == string(surfaceidentity.CodexHomeDefault) {
				appendTarget(path)
			}
			continue
		}
		appendTarget(path)
	}
	return targets
}
